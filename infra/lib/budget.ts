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

    // 通知先メールは SSM から取る（コードにもリポジトリにも書かないため）。
    const email = StringParameter.valueForStringParameter(this, BUDGET_EMAIL_PARAMETER);

    // 通知 1 件分（種別としきい値）を作る小さな関数。メール通知の宛先は全件で共通。
    const notification = (type: 'ACTUAL' | 'FORECASTED', threshold: number) => ({
      notification: {
        notificationType: type,
        comparisonOperator: 'GREATER_THAN',
        threshold,
        thresholdType: 'PERCENTAGE',
      },
      subscribers: [{ subscriptionType: 'EMAIL', address: email }],
    });

    // 月額の予算本体。Project タグが付いたリソースの費用だけを対象にする。
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
      // 通知: 実績 80% / 実績 100% / 予測 100% の 3 つ。
      notificationsWithSubscribers: [
        notification('ACTUAL', 80),
        // FORECASTED は利用履歴が溜まるまで発火しないので、ACTUAL 100% も置く
        notification('ACTUAL', 100),
        notification('FORECASTED', 100),
      ],
    });
  }
}
