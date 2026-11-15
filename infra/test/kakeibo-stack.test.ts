import { readFileSync } from 'node:fs';
import { Match, Template } from 'aws-cdk-lib/assertions';
import { describe, expect, it } from 'vitest';
import { createApp } from '../lib/app';
import { testContext } from './helpers';
import {
  BOOTSTRAP_QUALIFIER,
  BUDGET_EMAIL_PARAMETER,
  BUDGET_NAME,
  FUNCTION_NAME,
  HTTP_API_NAME,
  LOG_GROUP_NAME,
  PROJECT_TAG_KEY,
  PROJECT_TAG_VALUE,
  STACK_NAME,
  TABLE_NAME,
  THROTTLING_BURST_LIMIT,
  THROTTLING_RATE_LIMIT,
  TOOLKIT_STACK_NAME,
} from '../lib/project-constants';

const cdkJson = JSON.parse(readFileSync(new URL('../cdk.json', import.meta.url), 'utf-8'));
// feature flags が効いた状態でテストする（CLI が cdk.json の context を渡すのと同じ）
// テストでは Go のビルドを走らせない
const { stack } = createApp({ context: testContext() });
const template = Template.fromStack(stack);

describe('Auth', () => {
  it('セルフサインアップが無効', () => {
    template.hasResourceProperties('AWS::Cognito::UserPool', {
      AdminCreateUserConfig: { AllowAdminCreateUserOnly: true },
    });
  });

  it('クライアントはシークレット無しで USER_PASSWORD_AUTH を使わない', () => {
    template.hasResourceProperties('AWS::Cognito::UserPoolClient', {
      GenerateSecret: false,
      PreventUserExistenceErrors: 'ENABLED',
    });
    const clients = template.findResources('AWS::Cognito::UserPoolClient');
    for (const c of Object.values(clients)) {
      const flows: string[] = c.Properties.ExplicitAuthFlows;
      expect(flows).toContain('ALLOW_USER_SRP_AUTH');
      expect(flows).not.toContain('ALLOW_USER_PASSWORD_AUTH');
    }
  });

  it('UserPool に Project タグが付く', () => {
    template.hasResourceProperties('AWS::Cognito::UserPool', {
      UserPoolTags: { [PROJECT_TAG_KEY]: PROJECT_TAG_VALUE },
    });
  });

  it('UserPool は削除保護あり・RETAIN', () => {
    template.hasResource('AWS::Cognito::UserPool', {
      DeletionPolicy: 'Retain',
      Properties: { DeletionProtection: 'ACTIVE' },
    });
  });
});

describe('Auth: OAuth / MFA', () => {
  it('OAuth が無効', () => {
    const clients = template.findResources('AWS::Cognito::UserPoolClient');
    expect(Object.keys(clients)).toHaveLength(1);
    for (const c of Object.values(clients)) {
      expect(c.Properties.AllowedOAuthFlowsUserPoolClient ?? false).toBe(false);
      expect(c.Properties).not.toHaveProperty('AllowedOAuthFlows');
      expect(c.Properties).not.toHaveProperty('CallbackURLs');
    }
  });

  it('MFA は必須で TOTP のみ', () => {
    template.hasResourceProperties('AWS::Cognito::UserPool', {
      MfaConfiguration: 'ON',
      EnabledMfas: ['SOFTWARE_TOKEN_MFA'],
    });
  });
});

describe('Budget', () => {
  it('常に 1 つ作り、通知は 3 つ', () => {
    template.resourceCountIs('AWS::Budgets::Budget', 1);
    template.hasResourceProperties('AWS::Budgets::Budget', {
      Budget: {
        BudgetType: 'COST',
        TimeUnit: 'MONTHLY',
        BudgetLimit: { Amount: 5, Unit: 'USD' },
        CostFilters: { TagKeyValue: [`user:${PROJECT_TAG_KEY}$${PROJECT_TAG_VALUE}`] },
      },
      NotificationsWithSubscribers: Match.arrayWith([
        Match.objectLike({ Notification: Match.objectLike({ NotificationType: 'ACTUAL', Threshold: 80 }) }),
        Match.objectLike({ Notification: Match.objectLike({ NotificationType: 'ACTUAL', Threshold: 100 }) }),
        Match.objectLike({ Notification: Match.objectLike({ NotificationType: 'FORECASTED', Threshold: 100 }) }),
      ]),
    });
    const [budget] = Object.values(template.findResources('AWS::Budgets::Budget'));
    expect(budget.Properties.NotificationsWithSubscribers).toHaveLength(3);
  });

  it('購読先は SSM パラメータ型の CfnParameter を参照する', () => {
    const params = template.findParameters('*', { Type: 'AWS::SSM::Parameter::Value<String>' });
    const matched = Object.entries(params).filter(([, p]) => p.Default === BUDGET_EMAIL_PARAMETER);
    expect(matched).toHaveLength(1);
    const [paramName] = matched[0];

    const [budget] = Object.values(template.findResources('AWS::Budgets::Budget'));
    for (const n of budget.Properties.NotificationsWithSubscribers) {
      expect(n.Subscribers).toEqual([{ SubscriptionType: 'EMAIL', Address: { Ref: paramName } }]);
    }
  });

  it('BudgetName が BUDGET_NAME', () => {
    template.hasResourceProperties('AWS::Budgets::Budget', { Budget: { BudgetName: BUDGET_NAME } });
  });

  it('クレジットと返金を除外する', () => {
    template.hasResourceProperties('AWS::Budgets::Budget', {
      Budget: { CostTypes: Match.objectLike({ IncludeCredit: false, IncludeRefund: false }) },
    });
  });

  it('Project タグが付く', () => {
    template.hasResourceProperties('AWS::Budgets::Budget', {
      ResourceTags: [{ Key: PROJECT_TAG_KEY, Value: PROJECT_TAG_VALUE }],
    });
  });
});

describe('App 組み立て', () => {
  it('stackName が MyKakeiboV2', () => {
    expect(STACK_NAME).toBe('MyKakeiboV2');
    expect(stack.stackName).toBe('MyKakeiboV2');
  });

  it('synthesizer の qualifier が kakeibov2', () => {
    expect(BOOTSTRAP_QUALIFIER).toBe('kakeibov2');
    const json = JSON.stringify(template.toJSON());
    expect(json).toContain('/cdk-bootstrap/kakeibov2/version');
    expect(Object.keys(template.toJSON().Parameters)).toContain('BootstrapVersion');
  });
});

describe('Outputs', () => {
  it('UserPoolId / UserPoolClientId を出力する', () => {
    template.hasOutput('UserPoolId', {});
    template.hasOutput('UserPoolClientId', {});
  });
});

describe('cdk.json', () => {
  it('toolkitStackName と qualifier が定数と一致する', () => {
    expect(cdkJson.toolkitStackName).toBe(TOOLKIT_STACK_NAME);
    expect(cdkJson.context['@aws-cdk/core:bootstrapQualifier']).toBe(BOOTSTRAP_QUALIFIER);
  });

  it('最上位 tags が CLI の受け付ける配列形式で、定数と一致する（bootstrap / deploy のスタックタグ）', () => {
    expect(Array.isArray(cdkJson.tags)).toBe(true);
    expect(cdkJson.tags).toEqual([{ Key: PROJECT_TAG_KEY, Value: PROJECT_TAG_VALUE }]);
  });

  it('explicitStackTags が有効（スタックのタグは cdk.json の tags 頼み）', () => {
    expect(cdkJson.context['@aws-cdk/core:explicitStackTags']).toBe(true);
  });

  it('watch 設定が無い', () => {
    expect(cdkJson).not.toHaveProperty('watch');
  });
});

describe('API', () => {
  const api = Template.fromStack(createApp({ context: testContext() }).stack);
  const tag = { Key: PROJECT_TAG_KEY, Value: PROJECT_TAG_VALUE };

  it('ルートは 12 個で、JWT + アクセストークンのスコープ、ANY / $default なし', () => {
    const routes = Object.values(api.findResources('AWS::ApiGatewayV2::Route'));
    expect(routes).toHaveLength(12);
    const expected = ['accounts', 'categories'].flatMap((r) => [
      `GET /${r}`,
      `POST /${r}`,
      `GET /${r}/{id}`,
      `PUT /${r}/{id}`,
      `POST /${r}/{id}/archive`,
      `POST /${r}/{id}/unarchive`,
    ]);
    expect(routes.map((r) => r.Properties.RouteKey).sort()).toEqual(expected.sort());
    for (const r of routes) {
      expect(r.Properties.AuthorizationType).toBe('JWT');
      expect(r.Properties.AuthorizationScopes).toEqual(['aws.cognito.signin.user.admin']);
      expect(r.Properties.RouteKey).not.toMatch(/^(ANY|\$default)/);
    }
  });

  it('authorizer は既存の UserPoolClient を参照し、クライアントは 1 つのまま', () => {
    const clients = api.findResources('AWS::Cognito::UserPoolClient');
    expect(Object.keys(clients)).toHaveLength(1);
    const [clientId] = Object.keys(clients);
    api.hasResourceProperties('AWS::ApiGatewayV2::Authorizer', {
      AuthorizerType: 'JWT',
      JwtConfiguration: { Audience: [{ Ref: clientId }] },
    });
  });

  it('全ロールに boundary と Path が付き、信頼先は lambda だけ', () => {
    const roles = Object.values(api.findResources('AWS::IAM::Role'));
    expect(roles.length).toBeGreaterThan(0);
    for (const r of roles) {
      expect(r.Properties.PermissionsBoundary).toBeDefined();
      expect(r.Properties.Path).toBe('/my-kakeibo-v2/');
      const principals = r.Properties.AssumeRolePolicyDocument.Statement.map((s: { Principal: unknown }) => s.Principal);
      expect(principals).toEqual(principals.map(() => ({ Service: 'lambda.amazonaws.com' })));
    }
  });

  it('テーブル', () => {
    api.hasResource('AWS::DynamoDB::Table', {
      DeletionPolicy: 'Retain',
      Properties: {
        TableName: TABLE_NAME,
        DeletionProtectionEnabled: true,
        BillingMode: 'PAY_PER_REQUEST',
        PointInTimeRecoverySpecification: { PointInTimeRecoveryEnabled: true },
        KeySchema: [
          { AttributeName: 'PK', KeyType: 'HASH' },
          { AttributeName: 'SK', KeyType: 'RANGE' },
        ],
        AttributeDefinitions: ['PK', 'SK', 'GSI1PK', 'GSI1SK', 'GSI2PK', 'GSI2SK'].map((n) => ({
          AttributeName: n,
          AttributeType: 'S',
        })),
        GlobalSecondaryIndexes: ['GSI1', 'GSI2'].map((n) => ({
          IndexName: n,
          KeySchema: [
            { AttributeName: `${n}PK`, KeyType: 'HASH' },
            { AttributeName: `${n}SK`, KeyType: 'RANGE' },
          ],
          Projection: { ProjectionType: 'ALL' },
          OnDemandThroughput: { MaxReadRequestUnits: 100, MaxWriteRequestUnits: 25 },
        })),
        OnDemandThroughput: { MaxReadRequestUnits: 100, MaxWriteRequestUnits: 25 },
        Tags: Match.arrayWith([tag]),
      },
    });
    const [table] = Object.values(api.findResources('AWS::DynamoDB::Table'));
    expect(table.Properties).not.toHaveProperty('StreamSpecification');
    expect(table.Properties).not.toHaveProperty('TimeToLiveSpecification');
  });

  it('Lambda のテーブル権限は必要な操作だけで、テーブルとその index に限る', () => {
    const [tableId] = Object.keys(api.findResources('AWS::DynamoDB::Table'));
    const statements = Object.values(api.findResources('AWS::IAM::Policy')).flatMap(
      (p) => p.Properties.PolicyDocument.Statement as { Action: string | string[]; Resource: unknown }[],
    );
    const actions = statements.flatMap((s) => (Array.isArray(s.Action) ? s.Action : [s.Action]));
    expect([...actions].sort()).toEqual(
      [
        'dynamodb:BatchGetItem',
        'dynamodb:ConditionCheckItem',
        'dynamodb:GetItem',
        'dynamodb:PutItem',
        'dynamodb:Query',
        'dynamodb:UpdateItem',
      ].sort(),
    );
    const resources = JSON.stringify(statements.map((s) => s.Resource));
    expect(resources).toContain(tableId);
    expect(resources).toContain('/index/*');
  });

  it('withHttpApi の true / false で共通リソースの論理 ID が一致する（置換が起きない）', () => {
    const common = ['AWS::DynamoDB::Table', 'AWS::Lambda::Function', 'AWS::IAM::Role', 'AWS::IAM::Policy', 'AWS::Logs::LogGroup'];
    const ids = (t: Template) =>
      common.map((type) => Object.keys(t.findResources(type)).sort());
    const off = Template.fromStack(createApp({ context: testContext({ 'kakeibo:withHttpApi': false }) }).stack);
    expect(ids(off)).toEqual(ids(api));
  });

  it.each(['false', false])('withHttpApiStage=%s のときステージが無く、Api・Route・ApiUrl はある', (v) => {
    const t = Template.fromStack(
      createApp({ context: testContext({ 'kakeibo:withHttpApiStage': v }) }).stack,
    );
    t.resourceCountIs('AWS::ApiGatewayV2::Stage', 0);
    t.resourceCountIs('AWS::ApiGatewayV2::Api', 1);
    t.resourceCountIs('AWS::ApiGatewayV2::Route', 12);
    t.hasOutput('ApiUrl', {});
  });

  it('既定ではステージがあり、論理 ID が変わらない', () => {
    api.resourceCountIs('AWS::ApiGatewayV2::Stage', 1);
    expect(Object.keys(api.findResources('AWS::ApiGatewayV2::Stage'))).toEqual(['ApiHttpApiDefaultStageE708EF42']);
  });

  it('withHttpApiStage の true / false で Api・Route・Integration・Authorizer の論理 ID が一致する（段 3 で API が置換されない）', () => {
    const types = ['AWS::ApiGatewayV2::Api', 'AWS::ApiGatewayV2::Route', 'AWS::ApiGatewayV2::Integration', 'AWS::ApiGatewayV2::Authorizer'];
    const ids = (t: Template) => types.map((type) => Object.keys(t.findResources(type)).sort());
    const off = Template.fromStack(createApp({ context: testContext({ 'kakeibo:withHttpApiStage': false }) }).stack);
    expect(ids(off)).toEqual(ids(api));
    for (const group of ids(api)) expect(group.length).toBeGreaterThan(0);
  });

  it('LogGroup', () => {
    api.hasResource('AWS::Logs::LogGroup', {
      DeletionPolicy: 'Delete',
      Properties: { LogGroupName: LOG_GROUP_NAME, RetentionInDays: 30, Tags: Match.arrayWith([tag]) },
    });
  });

  it('Lambda', () => {
    api.hasResourceProperties('AWS::Lambda::Function', {
      FunctionName: FUNCTION_NAME,
      Runtime: 'provided.al2023',
      Architectures: ['arm64'],
      Environment: { Variables: { TABLE_NAME: { Ref: Object.keys(api.findResources('AWS::DynamoDB::Table'))[0] } } },
      LoggingConfig: { LogGroup: { Ref: Object.keys(api.findResources('AWS::Logs::LogGroup'))[0] } },
      Tags: Match.arrayWith([tag]),
    });
    const [fn] = Object.values(api.findResources('AWS::Lambda::Function'));
    expect(fn.Properties).not.toHaveProperty('ReservedConcurrentExecutions');
  });

  it('Lambda::Permission の SourceArn が自 API を参照する', () => {
    const [apiId] = Object.keys(api.findResources('AWS::ApiGatewayV2::Api'));
    const perms = Object.values(api.findResources('AWS::Lambda::Permission'));
    expect(perms.length).toBeGreaterThan(0);
    for (const p of perms) expect(JSON.stringify(p.Properties.SourceArn)).toContain(apiId);
  });

  it('HttpApi の名前・タグ、ステージのスロットリング', () => {
    api.hasResourceProperties('AWS::ApiGatewayV2::Api', { Name: HTTP_API_NAME, Tags: { [PROJECT_TAG_KEY]: PROJECT_TAG_VALUE } });
    api.hasResourceProperties('AWS::ApiGatewayV2::Stage', {
      DefaultRouteSettings: { ThrottlingRateLimit: THROTTLING_RATE_LIMIT, ThrottlingBurstLimit: THROTTLING_BURST_LIMIT },
    });
  });

  it('カスタムリソースが無い', () => {
    const types = Object.values(api.toJSON().Resources).map((r) => (r as { Type: string }).Type);
    expect(types.filter((t) => t.startsWith('Custom::') || t === 'AWS::CloudFormation::CustomResource')).toEqual([]);
  });

  it('Output', () => {
    api.hasOutput('ApiUrl', {});
    api.hasOutput('UserPoolId', {});
    api.hasOutput('UserPoolClientId', {});
  });

  it.each(['false', false])('withHttpApi=%s のとき API Gateway と ApiUrl が無い', (v) => {
    const t = Template.fromStack(
      createApp({ context: testContext({ 'kakeibo:withHttpApi': v }) }).stack,
    );
    const types = Object.values(t.toJSON().Resources).map((r) => (r as { Type: string }).Type);
    expect(types.filter((x) => x.startsWith('AWS::ApiGatewayV2::'))).toEqual([]);
    expect(Object.keys(t.toJSON().Outputs)).not.toContain('ApiUrl');
    t.resourceCountIs('AWS::DynamoDB::Table', 1);
    t.resourceCountIs('AWS::Lambda::Function', 1);
    t.resourceCountIs('AWS::Lambda::Permission', 0);
  });
});
