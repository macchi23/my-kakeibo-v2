# infra/iam

共用 AWS アカウントで、このプロジェクトの CDK 実行ロールと人の権限セットを絞るためのポリシー JSON。作成コマンドは `docs/setup.md` を参照。

| ファイル | 反映先 |
|---|---|
| `cfn-exec-policy.json` | customer managed policy `my-kakeibo-v2-cfn-exec`。bootstrap の `--cloudformation-execution-policies` に渡す（CloudFormation が使う実行ロールの権限） |
| `permission-set-inline.json` | IAM Identity Center の権限セット `my-kakeibo-v2` のインラインポリシー（人の権限） |

JSON が CDK の実体（定数・リソース型）と食い違わないことは `infra/test/iam-policies.test.ts` が確かめる。

## 書き方の規則

- ARN のアカウント欄には IAM ポリシー変数を書けない（変数はリソース部分と Condition の値だけ）。アカウント欄は `*` にし、アカウントの限定はロール名・バケット名の中の `${aws:PrincipalAccount}` で行う
- 実アカウント ID は書かない（テストが 12 桁の数字を検出する）
- Allow に `*` や `<service>:*` を使わない

## 人は deploy-role / lookup-role を使わない

deploy-role は全スタックの CloudFormation 操作を持ち、他プロジェクトのスタックを Admin の実行ロールで更新・削除できてしまうため。CDK CLI は deploy-role を引き受けられないと自分の資格情報で続行し、exec role を RoleARN に渡す。deploy 時の "Proceeding anyway" などの警告は想定どおり。

## 更新手順

- 中身の変更: `aws iam create-policy-version --set-as-default`。版は 5 つまでなので、超えたら古い版を消す。bootstrap は不要
- bootstrap が要るのは、付けるポリシーの ARN を増減するときと、bootstrap バージョンが上がるとき
- **toolkit スタックを作り直すときに `--cloudformation-execution-policies` を付け忘れると AdministratorAccess に戻る**

## P1 で足すもの

- permissions boundary `my-kakeibo-v2-boundary`（`boundary-policy.json`）。`create-policy` に `--tags Key=Project,Value=my-kakeibo-v2` を付ける
- exec role の IAM 部分: path `/my-kakeibo-v2/` 配下かつ boundary 付きでのみ CreateRole 等を許可、PassRole は lambda のみ、boundary 外し・ポリシー書き換えの Deny

`cdk gc` は全スタックのテンプレートを読むので権限セットには入れず、管理者の IAM ユーザー（`default` プロファイル）で実行する。

権限セットに `Resource: "*"` でしか許可できない列挙系（`ssm:DescribeParameters`、`cognito-idp:ListUserPools` など）は入れない。他プロジェクトのリソース名まで見えてしまうため。例外は確認手順で使う `tag:GetResources`。
