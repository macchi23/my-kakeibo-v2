import { readFileSync } from 'node:fs';
import { Match, Template } from 'aws-cdk-lib/assertions';
import { describe, expect, it } from 'vitest';
import { createApp } from '../lib/app';
import {
  BOOTSTRAP_QUALIFIER,
  BUDGET_EMAIL_PARAMETER,
  BUDGET_NAME,
  PROJECT_TAG_KEY,
  PROJECT_TAG_VALUE,
  STACK_NAME,
  TOOLKIT_STACK_NAME,
} from '../lib/project-constants';

const cdkJson = JSON.parse(readFileSync(new URL('../cdk.json', import.meta.url), 'utf-8'));
// feature flags が効いた状態でテストする（CLI が cdk.json の context を渡すのと同じ）
const { stack } = createApp({ context: cdkJson.context });
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
