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

export interface ApiProps {
  userPool: IUserPool;
  userPoolClient: IUserPoolClient;
  // false のとき HTTP API 一式を作らない（deploy を 2 回に分ける）
  withHttpApi: boolean;
}

// アクセストークンだけを通す。ID トークンにはこのスコープが無いので API Gateway が拒否する
const ACCESS_TOKEN_SCOPE = 'aws.cognito.signin.user.admin';

const MAX_READ_REQUEST_UNITS = 100;
const MAX_WRITE_REQUEST_UNITS = 25;

const RESOURCES = ['accounts', 'categories'] as const;

export class Api extends Construct {
  public readonly table: Table;
  public readonly function: GoFunction;
  public readonly httpApi?: HttpApi;

  constructor(scope: Construct, id: string, props: ApiProps) {
    super(scope, id);

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

    // logRetention は Resource: "*" のカスタムリソースを作り、boundary に logs:CreateLogGroup も無いので明示的に作る
    const logGroup = new LogGroup(this, 'LogGroup', {
      logGroupName: LOG_GROUP_NAME,
      retention: RetentionDays.ONE_MONTH,
      removalPolicy: RemovalPolicy.DESTROY,
    });

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

    if (!props.withHttpApi) return;

    const httpApi = new HttpApi(this, 'HttpApi', { apiName: HTTP_API_NAME });
    this.httpApi = httpApi;

    const integration = new HttpLambdaIntegration('LambdaIntegration', this.function);
    const authorizer = new HttpUserPoolAuthorizer('Authorizer', props.userPool, {
      userPoolClients: [props.userPoolClient],
    });
    const add = (path: string, methods: HttpMethod[]) =>
      httpApi.addRoutes({
        path,
        methods,
        integration,
        authorizer,
        authorizationScopes: [ACCESS_TOKEN_SCOPE],
      });
    for (const r of RESOURCES) {
      add(`/${r}`, [HttpMethod.GET]);
      add(`/${r}`, [HttpMethod.POST]);
      add(`/${r}/{id}`, [HttpMethod.GET]);
      add(`/${r}/{id}`, [HttpMethod.PUT]);
      add(`/${r}/{id}/archive`, [HttpMethod.POST]);
      add(`/${r}/{id}/unarchive`, [HttpMethod.POST]);
    }

    // L2 にスロットリングのプロップが無いのでエスケープハッチで設定する
    const stage = httpApi.defaultStage?.node.defaultChild as CfnStage;
    stage.defaultRouteSettings = { throttlingRateLimit: THROTTLING_RATE_LIMIT, throttlingBurstLimit: THROTTLING_BURST_LIMIT };
  }
}
