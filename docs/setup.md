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

1. 実行ロール用のポリシーを作る（`default` = 管理者の IAM ユーザー、ルート）
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
   - AccessDenied で止まったら、スタックイベントで足りない操作を確かめ、`infra/iam/cfn-exec-policy.json` を直して下の「ポリシーの更新」をする
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

運用メモ:
- bootstrap のバケットはデプロイごとにアセットが溜まる（Lambda を足す P1 以降）。ときどき `infra/` で `npx cdk gc --unstable=gc --type=s3 --profile default` で掃除する。gc は全スタックのテンプレートを読むので、`my-kakeibo-v2` には権限を持たせない
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
| `docs/articles/` | Qiita の下書き |
