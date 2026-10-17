import { CfnBudget } from 'aws-cdk-lib/aws-budgets';
import { StringParameter } from 'aws-cdk-lib/aws-ssm';
import { Construct } from 'constructs';
import { BUDGET_EMAIL_PARAMETER, BUDGET_NAME, PROJECT_TAG_KEY, PROJECT_TAG_VALUE } from './project-constants';

export interface ProjectBudgetProps {
  /** 月額上限（USD） */
  readonly limitUsd?: number;
}

// 通知先は SSM パラメータ BUDGET_EMAIL_PARAMETER からデプロイ時に解決する（synth 時は AWS に問い合わせない）。
// パラメータが無ければデプロイが失敗する。Budget は常に作る
export class ProjectBudget extends Construct {
  public readonly budget: CfnBudget;

  constructor(scope: Construct, id: string, props: ProjectBudgetProps = {}) {
    super(scope, id);

    const email = StringParameter.valueForStringParameter(this, BUDGET_EMAIL_PARAMETER);

    const notification = (type: 'ACTUAL' | 'FORECASTED', threshold: number) => ({
      notification: {
        notificationType: type,
        comparisonOperator: 'GREATER_THAN',
        threshold,
        thresholdType: 'PERCENTAGE',
      },
      subscribers: [{ subscriptionType: 'EMAIL', address: email }],
    });

    this.budget = new CfnBudget(this, 'Budget', {
      budget: {
        budgetName: BUDGET_NAME,
        budgetType: 'COST',
        timeUnit: 'MONTHLY',
        budgetLimit: { amount: props.limitUsd ?? 5, unit: 'USD' },
        costFilters: { TagKeyValue: [`user:${PROJECT_TAG_KEY}$${PROJECT_TAG_VALUE}`] },
        // クレジット・返金で相殺されてアラートが鳴らなくなるのを防ぐ
        costTypes: { includeCredit: false, includeRefund: false },
      },
      // CfnBudget には Tags.of が効かないので明示する
      resourceTags: [{ key: PROJECT_TAG_KEY, value: PROJECT_TAG_VALUE }],
      notificationsWithSubscribers: [
        notification('ACTUAL', 80),
        // FORECASTED は利用履歴が溜まるまで発火しないので、ACTUAL 100% も置く
        notification('ACTUAL', 100),
        notification('FORECASTED', 100),
      ],
    });
  }
}
