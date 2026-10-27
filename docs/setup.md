# セットアップと再構築

このリポジトリを自分の AWS アカウントで動かす手順。設計の理由は `DECISIONS.md`。

## 前提

- node 24 / npm / aws-cli v2 / go 1.27 / gh / shellcheck / gitleaks / jq（hook が依存。無いと fail-closed）
- AWS は IAM Identity Center（SSO）で、専用の権限セット `my-kakeibo-v2` のプロファイルを作る（`infra/iam/permission-set-inline.json`）。ポリシーの作成や bootstrap には、別に IAM の書き込みができる管理者の権限が要る（手順では `default` プロファイル）
- CDK の bootstrap は専用の qualifier（`kakeibov2`）と toolkit スタック名（`CDKToolkit-my-kakeibo-v2`）を使う。アカウント共用の `CDKToolkit` を上書きしないため
- `infra/cdk.json` の `tags` は配列形式（`[{"Key": ..., "Value": ...}]`）でないと CLI に黙って捨てられる（`@aws-cdk/core:explicitStackTags: true` のため deploy 側もフォールバックしない）
- 品質チェックは `scripts/check.sh all`。hook を触ったら `bash .claude/hooks/test_guards.sh`

## 再構築の手順

`default` を使う手順（1・5、ポリシーの更新、スタックの削除）と Identity Center（2）はユーザーが実行する。`my-kakeibo-v2` を使う手順はメインが実行してよい（CLAUDE.md の必須ルール）。`<...>` は自分の値に置き換える（リポジトリには書かない）。

各手順に作業ディレクトリを書いている（ルート = リポジトリのルート）。アカウント ID はシェル変数 `ACCOUNT` に入れ、ファイルに書かない。
シェルを開き直したら、手順 1 の 1 行目で `ACCOUNT` を取り直す。

1. 実行ロール用のポリシーを作る（`default` = 管理者の IAM ユーザー、ルート。P1 以降は下の「P1 の反映」を使う）
   ```sh
   ACCOUNT=$(aws sts get-caller-identity --profile default --query Account --output text)
   # 文法・警告の確認（読み取りのみ）。permission-set-inline.json も同じように確かめる
   aws accessanalyzer validate-policy --profile default --region us-east-1 \
     --policy-type IDENTITY_POLICY --policy-document file://infra/iam/cfn-exec-policy.json
   aws iam create-policy --profile default --policy-name my-kakeibo-v2-cfn-exec \
     --policy-document file://infra/iam/cfn-exec-policy.json --tags Key=Project,Value=my-kakeibo-v2
   ```
2. 権限セットを作る（IAM Identity Center の管理側。管理アカウントか委任管理者）
   - 権限セット `my-kakeibo-v2` を作る（タグ `Project=my-kakeibo-v2`、セッション時間は既定の 1 時間）
   - インラインポリシーに `infra/iam/permission-set-inline.json` の中身を入れ、自分のユーザーとこのアカウントに割り当てる
   - `aws configure sso` でプロファイル `my-kakeibo-v2`（既定リージョン us-east-1）を作り、`aws sso login --profile my-kakeibo-v2`
3. 予算アラートの通知先を SSM に置く（無いと deploy が失敗する。どこでもよい）
   ```sh
   aws ssm put-parameter --profile my-kakeibo-v2 --region us-east-1 \
     --name /my-kakeibo-v2/budget-email --type String --value '<通知先メール>' \
     --tags Key=Project,Value=my-kakeibo-v2
   ```
4. 依存を入れる（ルートから）: `cd infra && npm ci`。以降の 5・6 は `infra/` で実行する
5. bootstrap（`default`、`infra/`。初回と、ポリシーの ARN を増減するとき）
   ```sh
   npx cdk bootstrap "aws://${ACCOUNT}/us-east-1" --profile default \
     --qualifier kakeibov2 --toolkit-stack-name CDKToolkit-my-kakeibo-v2 --tags Project=my-kakeibo-v2 \
     --cloudformation-execution-policies "arn:aws:iam::${ACCOUNT}:policy/my-kakeibo-v2-cfn-exec"
   ```
   - **`--cloudformation-execution-policies` は必須。** toolkit スタックを作り直すときに付け忘れると、実行ロールが AdministratorAccess に戻る
   - qualifier・toolkit スタック名・タグは `cdk.json` にもあるが、明示する。`cdk.json` の無いディレクトリで実行すると、既定の `CDKToolkit`（他 PJ と共用）に向かい、他 PJ の実行ロールを差し替えてしまうため
   - 出力に AdministratorAccess の警告が出ず、`my-kakeibo-v2-cfn-exec` が付いたことを確かめる
   - `--bootstrap-customer-key` は付けない（KMS の固定費がかかる）
6. デプロイ（`my-kakeibo-v2`、`infra/`）: `npx cdk diff --profile my-kakeibo-v2 --method=template` → `npx cdk deploy --profile my-kakeibo-v2`
   - 初回の diff は `--method=template` にする。既定の change set 方式は、スタックが無いと空のスタックを作ってから消そうとし、DeleteStack が無いので消せずに残る
   - deploy-role・lookup role を引き受けられない旨の警告（"Proceeding anyway" など）は想定どおり。人には deploy-role を使わせない（DECISIONS.md の「AWS の権限」）
   - AccessDenied で止まったら、権限を広げて回避せず、下の「P1 の反映」の「AccessDenied が出たとき」に従う
   - 初回作成が失敗して `ROLLBACK_COMPLETE` になったら、再 deploy の前に `default` で `aws cloudformation delete-stack --region us-east-1 --stack-name MyKakeiboV2` する（CLI は再 deploy の前に削除しようとするが、`my-kakeibo-v2` には DeleteStack が無い）
   - 出力の UserPoolId / UserPoolClientId はリポジトリに保存しない（`--outputs-file` を使うならリポジトリの外に出す）
7. 自分のユーザーを作る（セルフサインアップは無効）
   ```sh
   aws cognito-idp admin-create-user --profile my-kakeibo-v2 --region us-east-1 \
     --user-pool-id <UserPoolId> --username '<メール>' \
     --user-attributes Name=email,Value='<メール>' Name=email_verified,Value=true
   ```
   - 仮パスワードがメールで届く。初回ログイン（P2 のフロント）でパスワード変更と TOTP の登録をする
8. Billing → コスト配分タグで `Project` を有効化する（他 PJ で有効化済みなら不要）。有効化前の費用は予算に集計されない
9. 確認（`my-kakeibo-v2`、us-east-1）
   - `aws cognito-idp describe-user-pool --user-pool-id <UserPoolId>` で `AllowAdminCreateUserOnly: true`、`MfaConfiguration: ON`
   - `aws resourcegroupstaggingapi get-resources --tag-filters Key=Project,Values=my-kakeibo-v2` で User Pool・Budget・bootstrap のバケットが出る
   - 隔離: `aws sts assume-role --profile my-kakeibo-v2 --role-arn "arn:aws:iam::${ACCOUNT}:role/cdk-kakeibov2-deploy-role-${ACCOUNT}-us-east-1" --role-session-name check` が AccessDenied になる
   - 実行ロールの限定（`default`）: `aws iam simulate-principal-policy --policy-source-arn "arn:aws:iam::${ACCOUNT}:role/cdk-kakeibov2-cfn-exec-role-${ACCOUNT}-us-east-1" --action-names cognito-idp:DeleteUserPool iam:CreateRole` がどちらも denied

ポリシーの更新（サービスを足すとき。`default`、ルート）:
```sh
aws iam create-policy-version --profile default \
  --policy-arn "arn:aws:iam::${ACCOUNT}:policy/my-kakeibo-v2-cfn-exec" \
  --policy-document file://infra/iam/cfn-exec-policy.json --set-as-default
```
- bootstrap は要らない。版は 5 つまでなので、超えたら `aws iam delete-policy-version` で古い版を消す
- 権限セットのインラインポリシーは Identity Center の管理側で更新し、割り当て先へ再プロビジョニングする

## P1 の反映（Lambda・DynamoDB・API Gateway を足すとき）

P0 のあとで、実行ロールのポリシーを 2 本に分け、permissions boundary を足す（理由は `DECISIONS.md` の「AWS の権限」）。再構築のときも手順 1・5 の代わりにこの節を使う。
以下は P0 の `my-kakeibo-v2-cfn-exec` が既にある前提。**新しいアカウントでは cfn-exec も `create-policy` で作る**（手順 1 の `create-policy-version` の代わりに、`--policy-name my-kakeibo-v2-cfn-exec --policy-document file://infra/iam/cfn-exec-policy.json --tags Key=Project,Value=my-kakeibo-v2`。list / delete-policy-version は不要）。
`default` の手順はユーザーが実行する。ルートで実行し、`ACCOUNT` は手順 1 と同じ。

1. ポリシーを作る・更新する（`default`、ルート）
   ```sh
   ACCOUNT=$(aws sts get-caller-identity --profile default --query Account --output text)
   for f in boundary-policy cfn-exec-app-policy cfn-exec-policy; do
     aws accessanalyzer validate-policy --profile default --region us-east-1 \
       --policy-type IDENTITY_POLICY --policy-document "file://infra/iam/${f}.json"
   done
   # boundary を先に作る。cfn-exec の条件が boundary の ARN を参照する
   aws iam create-policy --profile default --policy-name my-kakeibo-v2-boundary \
     --policy-document file://infra/iam/boundary-policy.json --tags Key=Project,Value=my-kakeibo-v2
   aws iam create-policy --profile default --policy-name my-kakeibo-v2-cfn-exec-app \
     --policy-document file://infra/iam/cfn-exec-app-policy.json --tags Key=Project,Value=my-kakeibo-v2
   # 既存の cfn-exec は版の更新。版が 5 つなら、既定でない最古の版を先に消す
   aws iam list-policy-versions --profile default --policy-arn "arn:aws:iam::${ACCOUNT}:policy/my-kakeibo-v2-cfn-exec"
   aws iam delete-policy-version --profile default --policy-arn "arn:aws:iam::${ACCOUNT}:policy/my-kakeibo-v2-cfn-exec" --version-id <最古の版>
   aws iam create-policy-version --profile default --set-as-default \
     --policy-arn "arn:aws:iam::${ACCOUNT}:policy/my-kakeibo-v2-cfn-exec" \
     --policy-document file://infra/iam/cfn-exec-policy.json
   ```
   - `validate-policy` は ERROR が 0 であること。SUGGESTION・WARNING は内容を読んで判断する
2. bootstrap をやり直す（`default`、`infra/`）。付けるポリシーの ARN が 2 本になるため。他のフラグは手順 5 と同じ
   ```sh
   npx cdk bootstrap "aws://${ACCOUNT}/us-east-1" --profile default \
     --qualifier kakeibov2 --toolkit-stack-name CDKToolkit-my-kakeibo-v2 --tags Project=my-kakeibo-v2 \
     --cloudformation-execution-policies "arn:aws:iam::${ACCOUNT}:policy/my-kakeibo-v2-cfn-exec,arn:aws:iam::${ACCOUNT}:policy/my-kakeibo-v2-cfn-exec-app"
   ```
3. 権限セットを更新する（Identity Center の管理側）: インラインポリシーを `infra/iam/permission-set-inline.json` の中身に差し替え、割り当て先へ再プロビジョニングする
4. 確認（`default`、ルート）
   ```sh
   EXEC="arn:aws:iam::${ACCOUNT}:role/cdk-kakeibov2-cfn-exec-role-${ACCOUNT}-us-east-1"
   BOUNDARY="arn:aws:iam::${ACCOUNT}:policy/my-kakeibo-v2-boundary"
   REGION_CTX="ContextKeyName=aws:RequestedRegion,ContextKeyValues=us-east-1,ContextKeyType=string"
   # path 配下 + boundary 付きは allowed
   aws iam simulate-principal-policy --profile default --policy-source-arn "$EXEC" \
     --action-names iam:CreateRole iam:DeleteRole iam:UpdateAssumeRolePolicy \
     --resource-arns "arn:aws:iam::${ACCOUNT}:role/my-kakeibo-v2/x" \
     --context-entries "$REGION_CTX" "ContextKeyName=iam:PermissionsBoundary,ContextKeyValues=${BOUNDARY},ContextKeyType=string"
   # path 外のロールは boundary の context を付けても implicitDeny（他 PJ のロールに届かない）
   aws iam simulate-principal-policy --profile default --policy-source-arn "$EXEC" \
     --action-names iam:CreateRole iam:DeleteRole iam:PutRolePolicy \
     --resource-arns "arn:aws:iam::${ACCOUNT}:role/x" \
     --context-entries "$REGION_CTX" "ContextKeyName=iam:PermissionsBoundary,ContextKeyValues=${BOUNDARY},ContextKeyType=string"
   # boundary 無しの CreateRole は explicitDeny
   aws iam simulate-principal-policy --profile default --policy-source-arn "$EXEC" \
     --action-names iam:CreateRole --resource-arns "arn:aws:iam::${ACCOUNT}:role/my-kakeibo-v2/x" \
     --context-entries "$REGION_CTX"
   # 同時実行数の上限（新しいアカウントは低いことがある）
   aws lambda get-account-settings --profile default --region us-east-1 --query 'AccountLimit.ConcurrentExecutions'
   ```
   - 上から順に: path 配下 + boundary 付きは 3 アクションとも `allowed`、path 外は 3 アクションとも `implicitDeny`、boundary 無しは `explicitDeny` で `MatchedStatements` が `DenyRoleWithoutProjectBoundary` であること
   - シミュレーターが `${aws:PrincipalAccount}` を解決しないときは、`aws:PrincipalAccount` も `--context-entries` に足す
   - `ConcurrentExecutions` が低い（10 など）と、予約同時実行数（Lambda 単位の上限）を設定できない
5. deploy は手順 6 と同じ（`my-kakeibo-v2`）。失敗した作成のロールバックで、RETAIN のリソースが残ることがある。残ると再 deploy が同名で衝突する。
   - テーブルは残る。ロググループは `removalPolicy: DESTROY` を指定していれば残らない（CDK の既定は RETAIN）
   - 残っていたら、中身が空であることを確かめ、ユーザーが `default` で削除してから再 deploy する。テーブルは削除保護を外してから消す
     ```sh
     aws dynamodb update-table --profile default --region us-east-1 --table-name my-kakeibo-v2-data-v1 --no-deletion-protection-enabled
     aws dynamodb delete-table --profile default --region us-east-1 --table-name my-kakeibo-v2-data-v1
     ```

AccessDenied が出たとき: スタックイベントで拒否された操作を確かめ、該当の JSON を直して版を上げる（上の手順 1 の `create-policy-version`。`cfn-exec` と `cfn-exec-app` のどちらの JSON か、ARN を取り違えない）。
**権限を広げて回避しない。** 条件を外す、タグ付けの権限を条件なしで足す、といった変更は他 PJ のリソースに届くので、止めてユーザーに相談する。初回 deploy で確かめきれていない点は次のとおり。
1. HTTP API の子リソース（routes / integrations / authorizers / stages / deployments）にリソースタグの条件が効かないとき。相談の案: 先に Api だけを deploy して ID を得て、Resource を `arn:aws:apigateway:us-east-1::/apis/<その ID>/*` に固定する（ID は反映時にシェル変数で埋め、リポジトリには置かない）
2. 各ハンドラーが作成リクエストにタグを含めないとき（`aws:RequestTag/Project` の条件が効かない）
3. Stage のハンドラーが `apigateway:TagResource` を要求するとき（Service Reference に `TagResource` は無い。確認済み: タグ付けは `/tags/*` への `apigateway:PUT` / `DELETE`で、POST ではない）。条件なしで足すと他 PJ の API に `Project` タグを付けて条件を乗っ取れるので、足さずに相談する
4. アセットの読み取りで KMS の権限が要るとき（`aws/s3` 管理キーなら要らない想定）
5. 失敗した作成のロールバックで RETAIN のリソースが残る（上の手順 5）

各コマンドが内部で呼ぶ API:

| コマンド | プロファイル | 内部で呼ぶ API | 通るか |
|---|---|---|---|
| `validate-policy` | `default` | `access-analyzer:ValidatePolicy` | 管理者権限のため通る想定（未確認） |
| `iam create-policy`（`--tags` 付き） | `default` | `iam:CreatePolicy`、`iam:TagPolicy` | 同上（未確認） |
| `iam list-policy-versions` / `delete-policy-version` / `create-policy-version` | `default` | `iam:ListPolicyVersions` / `iam:DeletePolicyVersion` / `iam:CreatePolicyVersion`（`--set-as-default` は同じ呼び出し） | 同上（未確認） |
| `cdk bootstrap` | `default` | `sts:GetCallerIdentity`、`cloudformation:DescribeStacks` / `GetTemplate` / `CreateChangeSet` / `DescribeChangeSet` / `ExecuteChangeSet` / `DescribeStackEvents`、`ssm:GetParameter`（bootstrap バージョン）。スタック内では CloudFormation が IAM ロール・S3 バケットを更新する | 同上（未確認）。中断したら `DescribeStackEvents` で状態を見て同じコマンドを再実行する |
| `iam simulate-principal-policy` | `default` | `iam:SimulatePrincipalPolicy` | 同上（未確認）。`aws:RequestedRegion` などは自動で埋まらないので `--context-entries` で渡す |
| `lambda get-account-settings` | `default` | `lambda:GetAccountSettings` | 同上（未確認）。`my-kakeibo-v2` には持たせていない |
| `cdk deploy` | `my-kakeibo-v2` | 既存の CloudFormation 操作に加え、実行ロールが Lambda / DynamoDB / Logs / API Gateway / IAM の各ハンドラーを呼ぶ | 実 deploy で確かめる（未確認） |
| `aws logs tail <ロググループ> --follow` | `my-kakeibo-v2` | `logs:FilterLogEvents`（aws-cli 2.34 の `tail` は follow の有無にかかわらず `filter_log_events` だけを呼ぶ） | 通る想定（未確認） |

運用メモ:
- bootstrap のバケットはデプロイごとにアセットが溜まる（Lambda を足す P1 以降）。月 1 回か段階の区切りに、`infra/` で掃除する。gc は全スタックのテンプレートを読むので `default` で実行し、`my-kakeibo-v2` には権限を持たせない
  ```sh
  # 初回は --action=print で対象を確かめる
  npx cdk gc --unstable=gc --type=s3 --rollback-buffer-days=30 --toolkit-stack-name CDKToolkit-my-kakeibo-v2 --profile default --action=print
  npx cdk gc --unstable=gc --type=s3 --rollback-buffer-days=30 --toolkit-stack-name CDKToolkit-my-kakeibo-v2 --profile default
  ```
  - CLI の gc は `cdk.json` の `toolkitStackName` を読まず CLI 引数だけを見る（`infra/node_modules/aws-cdk/lib/index.js` の `case "gc"`）。付け忘れると共用の `CDKToolkit` のバケットを掃除する
- Budget の FORECASTED 通知は利用履歴が数週間溜まるまで発火しない。初期は ACTUAL 80% / 100% が頼り
- TOTP のスマホを無くしたら `aws cognito-idp admin-set-user-mfa-preference` で解除して登録し直す

## コード配置の要点

| パス | 内容 |
|---|---|
| `backend/` | Go（P1 から） |
| `infra/` | CDK（TypeScript） |
| `web/` | Next.js（P2 から） |
| `scripts/check.sh` | 品質チェック |
| `.claude/` | エージェント・スキル・hook |
| `docs/go/` | Go の学習資料と進捗 |
| `docs/articles/` | Zenn の下書き |
