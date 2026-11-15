import { CfnStage, HttpApi, HttpMethod } from 'aws-cdk-lib/aws-apigatewayv2';
import { HttpUserPoolAuthorizer } from 'aws-cdk-lib/aws-apigatewayv2-authorizers';
import { HttpLambdaIntegration } from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import { AttributeType, BillingMode, ProjectionType, Table } from 'aws-cdk-lib/aws-dynamodb';
import { Architecture, Runtime } from 'aws-cdk-lib/aws-lambda';
import { GoFunction } from '@aws-cdk/aws-lambda-go-alpha';
import { Duration, RemovalPolicy } from 'aws-cdk-lib';
import { LogGroup, RetentionDays } from 'aws-cdk-lib/aws-logs';
import { IUserPool, IUserPoolClient } from 'aws-cdk-lib/aws-cognito';
import { PolicyStatement } from 'aws-cdk-lib/aws-iam';
import { fileURLToPath } from 'node:url';
import { Construct } from 'constructs';
import {
  FUNCTION_NAME,
  HTTP_API_NAME,
  LOG_GROUP_NAME,
  TABLE_NAME,
  THROTTLING_BURST_LIMIT,
  THROTTLING_RATE_LIMIT,
} from './project-constants';

// Api Construct に渡す設定。認証で作ったユーザープールと、段階的な deploy のための切り替えフラグ。
export interface ApiProps {
  userPool: IUserPool;
  userPoolClient: IUserPoolClient;
  // false のとき HTTP API 一式を作らない（deploy を 2 回に分ける）
  withHttpApi: boolean;
  // false のとき既定ステージを作らない（API を作り直した直後、ポリシーに新しい API ID を埋める前の段）
  withHttpApiStage: boolean;
}

// アクセストークンだけを通す。ID トークンにはこのスコープが無いので API Gateway が拒否する
const ACCESS_TOKEN_SCOPE = 'aws.cognito.signin.user.admin';

// テーブルとインデックスの読み書きのスループットの上限（On-Demand の暴走対策）。
const MAX_READ_REQUEST_UNITS = 100;
const MAX_WRITE_REQUEST_UNITS = 25;

// ルートを作る対象のリソース名。どちらも同じ 6 本のルート（一覧・作成・取得・更新・archive・unarchive）を持つ。
const RESOURCES = ['accounts', 'categories'] as const;

// API 一式（DynamoDB のテーブル・Lambda・HTTP API）をまとめた Construct。
// 外から使うもの（テーブル・関数・API）は public readonly で公開する。
export class Api extends Construct {
  public readonly table: Table;
  public readonly function: GoFunction;
  public readonly httpApi?: HttpApi;

  constructor(scope: Construct, id: string, props: ApiProps) {
    super(scope, id);

    // DynamoDB のテーブル。PK / SK の複合キーで、課金はリクエスト単位（固定費なし）。
    // データを守るため、スタックを消してもテーブルは残し、削除保護と PITR（時点復旧）も有効にする。
    // 1 回の更新で足せる GSI は 1 つなので、2 つとも作成時に作る
    this.table = new Table(this, 'Table', {
      tableName: TABLE_NAME,
      partitionKey: { name: 'PK', type: AttributeType.STRING },
      sortKey: { name: 'SK', type: AttributeType.STRING },
      billingMode: BillingMode.PAY_PER_REQUEST,
      removalPolicy: RemovalPolicy.RETAIN,
      deletionProtection: true,
      pointInTimeRecoverySpecification: { pointInTimeRecoveryEnabled: true },
      // 暴走時の課金の上限（固定費は増えない）
      maxReadRequestUnits: MAX_READ_REQUEST_UNITS,
      maxWriteRequestUnits: MAX_WRITE_REQUEST_UNITS,
    });
    // 別の軸で引くための GSI（グローバルセカンダリインデックス）を 2 本。
    this.table.addGlobalSecondaryIndex({
      indexName: 'GSI1',
      partitionKey: { name: 'GSI1PK', type: AttributeType.STRING },
      sortKey: { name: 'GSI1SK', type: AttributeType.STRING },
      projectionType: ProjectionType.ALL,
      maxReadRequestUnits: MAX_READ_REQUEST_UNITS,
      maxWriteRequestUnits: MAX_WRITE_REQUEST_UNITS,
    });
    // sparse（下書き用）
    this.table.addGlobalSecondaryIndex({
      indexName: 'GSI2',
      partitionKey: { name: 'GSI2PK', type: AttributeType.STRING },
      sortKey: { name: 'GSI2SK', type: AttributeType.STRING },
      projectionType: ProjectionType.ALL,
      maxReadRequestUnits: MAX_READ_REQUEST_UNITS,
      maxWriteRequestUnits: MAX_WRITE_REQUEST_UNITS,
    });

    // Lambda のログの置き場。保持期間を 1 か月にして、ログが溜まり続けないようにする。
    // logRetention は Resource: "*" のカスタムリソースを作り、boundary に logs:CreateLogGroup も無いので明示的に作る
    const logGroup = new LogGroup(this, 'LogGroup', {
      logGroupName: LOG_GROUP_NAME,
      retention: RetentionDays.ONE_MONTH,
      removalPolicy: RemovalPolicy.DESTROY,
    });

    // Go の Lambda。GoFunction が backend/cmd/lambda を arm64 向けにビルドして zip にする。
    // テーブル名は環境変数で渡し、Go 側が起動時に読む。
    // 予約同時実行数は設定しない（アカウントの上限 10 で unreserved に 10 を残す必要がある）
    this.function = new GoFunction(this, 'Function', {
      functionName: FUNCTION_NAME,
      entry: fileURLToPath(new URL('../../backend/cmd/lambda', import.meta.url)),
      moduleDir: fileURLToPath(new URL('../../backend/go.mod', import.meta.url)),
      runtime: Runtime.PROVIDED_AL2023,
      architecture: Architecture.ARM_64,
      memorySize: 256,
      timeout: Duration.seconds(10),
      environment: { TABLE_NAME: this.table.tableName },
      logGroup,
      bundling: {
        cgoEnabled: false,
        goBuildFlags: ['-trimpath', '-buildvcs=false', '-tags=lambda.norpc'],
      },
    });
    // Lambda の実行ロールに、このテーブルとそのインデックスへの必要最小限の操作だけを許可する。
    // 物理削除の API は作らないので DeleteItem / BatchWriteItem は付けない。Scan も使わない。
    // grantReadWriteData は Scan / DeleteItem 等まで含むため、必要な操作だけを自前で書く
    this.function.addToRolePolicy(
      new PolicyStatement({
        actions: [
          'dynamodb:GetItem',
          'dynamodb:BatchGetItem',
          'dynamodb:Query',
          'dynamodb:PutItem',
          'dynamodb:UpdateItem',
          'dynamodb:ConditionCheckItem',
        ],
        resources: [this.table.tableArn, `${this.table.tableArn}/index/*`],
      }),
    );

    // deploy を 2 回に分けるとき、1 回目はここで終わる（テーブルと Lambda だけ）。
    if (!props.withHttpApi) return;

    // HTTP API 本体。ステージ（公開用の URL の入口）を作るかはフラグで切り替える。
    const httpApi = new HttpApi(this, 'HttpApi', {
      apiName: HTTP_API_NAME,
      createDefaultStage: props.withHttpApiStage,
    });
    this.httpApi = httpApi;

    // 全ルートが同じ Lambda を呼び、Cognito の JWT で認証する（統合と authorizer をルート間で共有）。
    const integration = new HttpLambdaIntegration('LambdaIntegration', this.function);
    const authorizer = new HttpUserPoolAuthorizer('Authorizer', props.userPool, {
      userPoolClients: [props.userPoolClient],
    });
    // ルートを 1 本足す小さな関数。統合・authorizer・必要スコープは共通。
    const add = (path: string, methods: HttpMethod[]) =>
      httpApi.addRoutes({
        path,
        methods,
        integration,
        authorizer,
        authorizationScopes: [ACCESS_TOKEN_SCOPE],
      });
    // 口座・カテゴリのそれぞれに 6 本のルートを登録する（Go 側の routes.go と対応）。
    for (const r of RESOURCES) {
      add(`/${r}`, [HttpMethod.GET]);
      add(`/${r}`, [HttpMethod.POST]);
      add(`/${r}/{id}`, [HttpMethod.GET]);
      add(`/${r}/{id}`, [HttpMethod.PUT]);
      add(`/${r}/{id}/archive`, [HttpMethod.POST]);
      add(`/${r}/{id}/unarchive`, [HttpMethod.POST]);
    }

    // L2 にスロットリングのプロップが無いのでエスケープハッチで設定する
    if (httpApi.defaultStage) {
      const stage = httpApi.defaultStage.node.defaultChild as CfnStage;
      stage.defaultRouteSettings = { throttlingRateLimit: THROTTLING_RATE_LIMIT, throttlingBurstLimit: THROTTLING_BURST_LIMIT };
    }
  }
}
