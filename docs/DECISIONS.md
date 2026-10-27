# 設計判断とその理由

作り直したくなったら、まずここを読む。多くは一度議論して決めた結論で、
理由を知らずに変えると同じ失敗を繰り返す。

## 目的と前提

**動機は「Go を触ってみたい」。効率は重視しない。** 目標は「Go 使ったことあります（CC に書かせたけど多少わかる）」レベル。
判断に迷ったら「Go の学習になるか」「個人利用で十分か」を優先する。

**GitHub で公開し、Zenn 連載も書く。** 秘密情報（API キー、アカウント ID、Cognito の ID 等）をリポジトリに置かない構成にする。
docs も公開前提で書く。

**環境は1つ（dev/prod を分けない）。** 個人利用のため。

**旧 my-kakeibo（前作）は設計の参照元。** 流用は必須ではない。初期の方針設計で参照し終えたら、旧 sandbox のリソースは削除する。
旧データの移行はしない（sandbox のテストデータのみ）。

## 基盤

**バックエンドは Go on Lambda（`provided.al2023`, arm64）+ API Gateway HTTP API。**

**API は Lambda 1つにまとめ、`net/http` + アダプタで書く。** Go 1.22 以降の `ServeMux` でメソッド・パスパラメータ付きの
ルーティングができるのでフレームワークは不要。`go run` でローカルの HTTP サーバーとして動き、標準的な Go を学べる。
1エンドポイント1Lambda は CDK 定義が増えるだけで Go の学びは変わらない。
レシート解析は別 Lambda（S3 イベント駆動、後述）。

**IaC は CDK（TypeScript）。** 学習対象を Go に集中させるため。CDK を Go で書くとポインタ（`jsii.String(...)`）だらけで冗長になり、
Go らしい書き方の練習にもならない。フロントも TS なので、リポジトリ内の言語は TS と Go の2つで済む。

**DB は DynamoDB。** 無料枠に収まり、キー設計の学習にもなる。集計（月次・カテゴリ別）は SQL で書けないが、
個人の取引は1か月数百件程度なので「対象月を Query して Go 側で集計」で十分。
分析を後回しにしても、キー設計は最初から「月単位で取引を引ける」前提にする。

**リージョンは us-east-1 に統一。** レイテンシは重視せず、コストを優先。
（実際は無料枠に収まるため東京との差は月数セント程度。Bedrock の新モデルが先に来る、`us.` / `global.` 推論プロファイルが使える利点もある）

**認証は Cognito + HTTP API の JWT authorizer。** Go 側で claims から userId を取り、ユーザー単位のデータ分離を自前で実装する。
userId には `sub` を使う（`email` は属性の更新で変わりうる）。

**Cognito はセルフサインアップ無効、MFA は TOTP 必須、OAuth 無効。**
- セルフサインアップ無効: URL もリポジトリも公開されるので、他人が登録できないようにする。ユーザーは `admin-create-user` で作る
- MFA は TOTP（Google Authenticator 等の認証アプリ）を必須にする。家計データを公開 URL で扱うので、パスワードが漏れても突破されないように。
  SMS は有料なので使わない。スマホを無くしたら `admin-set-user-mfa-preference` で解除して登録し直す
- App Client はシークレットなし・SRP のみ。OAuth / Hosted UI は使わない。CDK は既定で OAuth の implicit フローと
  `https://example.com` のコールバックを有効にするので `disableOAuth: true` を明示する（P0 の security-reviewer の指摘）
- User Pool は RETAIN と削除保護

**AWS リソースには `Project=my-kakeibo-v2` タグを必ず付ける。** CDK で `Tags.of(app).add(...)` を使う。
CDK の外で作るものは個別に付ける。料金をタグ別に見るには Billing でコスト配分タグとして有効化する。
`Tags.of` が効かないもの（CfnBudget）は `resourceTags` で明示し、bootstrap には `cdk.json` の `tags` で付ける（手順の `--tags` 付け忘れに頼らない）。

**CDK はスタック 1 つ（`MyKakeiboV2`）にし、領域ごとに Construct で分ける。** 環境が 1 つなので、スタック間参照の手間を避ける。
`env` は渡さない（環境非依存。認証なしで synth でき、CI に AWS の認証情報を置かずに済む）。

**bootstrap は専用の qualifier（`kakeibov2`）と toolkit スタック名（`CDKToolkit-my-kakeibo-v2`）を使う。** AWS アカウントは他のプロジェクトと共用で、
既定の `CDKToolkit` を上書きしないため。推奨の feature flags は初回デプロイ前に設定する（デプロイ後に変えるとリソースが作り直されることがある）。

**予算アラートは `Project` タグで絞った月額 5 USD の Budget。** ACTUAL 80% と FORECASTED 100% でメール通知。
- 通知先のメールアドレスは SSM パラメータ（`/my-kakeibo-v2/budget-email`）からデプロイ時に解決する。リポジトリに書かない。
  当初は CDK の context で渡し、未指定なら Budget を作らない設計だったが、`-c` を付け忘れてデプロイすると Budget が黙って削除されるため、
  Budget は常に作り、パラメータが無ければデプロイが失敗する形に変えた（P0 の cost-reviewer / code-reviewer の指摘）
- クレジットと返金は集計から除く。含めるとクレジットで相殺され、実費が出ていてもアラートが鳴らない
- アカウント全体の予算は置かない（共用アカウントのため、この PJ の管理対象外）。タグに乗らない費用（Bedrock のオンデマンド呼び出し）は、
  P3 で application inference profile にタグを付けて拾う
- タグで絞った予算は、Billing でコスト配分タグを有効化する前の費用を集計しない（有効化は最大 24h × 2 かかる）。初回デプロイ時に有効化する

## AWS の権限

**普段使う権限（プロファイル `my-kakeibo-v2`）は専用の権限セットに絞り、広い権限は bootstrap などの一時的な作業だけに使う。**
アカウントは他 PJ と共用なので、誤操作で他 PJ のリソースを壊す事故の可能性を下げる。
- ポリシーの作成・更新、bootstrap（初回と作り直し）、Amplify Hosting の初期接続は、管理者の IAM ユーザー（`default` プロファイル）で行う。
  他 PJ と共用の SSO の権限セットには IAM の書き込み権限が無かった（2026-10-17 に判明）。他 PJ と共用の権限セットに権限を足すより、既存の管理者で済ませる
- それ以外（deploy、diff、SSM、Cognito のユーザー操作、確認用の読み取り）は `my-kakeibo-v2` で行う
- `my-kakeibo-v2` での操作は Claude（メイン）が実行してよい。コストか権限がおかしくなる変更は除く（2026-10-18 にユーザーが決定）。
  当初は deploy もすべてユーザーが行う方針だったが、権限セットと実行ロールを絞ったことで、このプロファイルで壊せる範囲がこのプロジェクトのリソースに限られた。
  広い権限を使う操作は引き続きユーザーが行い、hook（guard-deploy）はメインの書き込みのたびに確認を出す

**CDK の実行ロール（`cdk-kakeibov2-cfn-exec-role`）も絞る。** 既定は AdministratorAccess で、人の権限を絞っても deploy-role 経由で
CloudFormation に何でも作らせられる（他 PJ のリソースにも手を出せる）ため、人だけ絞っても効果が薄い（2026-10-15 にユーザーが指摘）。
- 事前に作る customer managed policy `my-kakeibo-v2-cfn-exec`（`infra/iam/cfn-exec-policy.json`）を bootstrap の `--cloudformation-execution-policies` で付ける。
  P0 で許可するのは Cognito（作成はリクエストタグ、作成後はリソースタグ `Project` で限定。DeleteUserPool は外す）、Budget（名前の接頭辞 `my-kakeibo-v2-` で限定）、SSM の 2 パラメータの読み取りだけ。us-east-1 以外は一律 Deny。
  ほかに `billing:GetBillingViewData` を許可している。CloudFormation の Budget のハンドラーが作成・更新時に要求するため（リソーススキーマの handler permissions）。削ると Budget を作れない
- カスタム bootstrap テンプレートは使わない。800 行超のテンプレートを fork すると CDK の更新ごとに追従が要る
- ポリシーの中身の変更は版の更新（`create-policy-version`）だけで反映され、bootstrap のやり直しは要らない。bootstrap するのは付けるポリシーの ARN を増減するときと bootstrap のバージョンが上がるときだけ。
  ただし toolkit スタックを作り直すときにフラグを付け忘れると AdministratorAccess に戻る
- ポリシーが名前で絞るので、Budget 名を固定した（`my-kakeibo-v2-monthly-v1`）。名前と通知設定はどちらも作成後に変えられず、変えると置換になる。
  同じ名前のまま通知を変えると、置換で新しい Budget を先に作るときに名前が重複して失敗する。そこで、通知先やしきい値を変えるときは末尾の版を上げる（P0 の cost-reviewer の指摘）
- 初回デプロイで AccessDenied が出たら、スタックイベントで足りない操作を確かめて版を上げる（許可が足りずに止まる方向なので安全側）

**人（`my-kakeibo-v2`）には deploy-role と lookup-role を使わせない。** deploy-role は CloudFormation の更新・削除を `Resource: "*"` で持ち、
CloudFormation はスタックに紐づく実行ロールを PassRole の有無に関係なく使う。deploy-role を引き受けられると、他 PJ の CDK スタックを
そのスタックの Admin の実行ロールで更新・削除できてしまう（P0 の researcher の調査）。
- CDK の CLI は deploy-role を引き受けられないと自分の資格情報で続行し、実行ロールを RoleARN に渡す。そこで権限セットには「自スタック `MyKakeiboV2` だけの CloudFormation 操作」
  「実行ロールを渡すときだけ change set を作れる条件」「実行ロールの PassRole」「file-publishing role の AssumeRole」を与える（`infra/iam/permission-set-inline.json`）
- 代償として deploy のたびに警告が出る。deploy-role を絞るためにテンプレートを fork する案は保守負担で外した
- lookup role はアカウント全体の ReadOnlyAccess で、env 非依存なので lookup も使わない
- DeleteStack は持たせない。destroy は管理者の IAM ユーザーで行う。CREATE に失敗したスタックの削除（CLI は再 deploy の前に削除しようとする）も同じ
- `cdk gc` は全スタックのテンプレートを読むので、権限セットには入れず管理者の IAM ユーザーで回す
- `Resource: "*"` でしか許可できない列挙系（`ssm:DescribeParameters`、`cognito-idp:ListUserPools`）は持たせない。他 PJ のリソース名やパラメータ名まで見えるため。
  例外は、確認手順で使う `tag:GetResources` だけ
- 権限セットはインラインポリシーにする（customer managed の参照は各アカウントに同名のポリシーが要り、上限も小さい）

**IAM ロールを作る経路（P1 から）は permissions boundary と role path で囲う。**
- CDK の context `@aws-cdk/core:permissionsBoundary` で、アプリが作るすべてのロール（CDK 内部のカスタムリソース用を含む）に `my-kakeibo-v2-boundary` を付け、
  Aspect で path を `/my-kakeibo-v2/` に揃える（P0 で土台だけ入れた。P0 にはロールが無い）
- P1 で実行ロールに「path 配下かつ boundary 付きでのみ CreateRole 等」「PassRole は lambda にだけ」「boundary 外し・ポリシー書き換えの Deny」を足す。
  path で絞ると、boundary 条件に対応しない操作（TagRole など）も他 PJ のロールに届かない
- bootstrap の `--custom-permissions-boundary`（実行ロール自身に付く）は使わない。再 bootstrap で付け忘れると黙って外れる。`--example-permissions-boundary` は中身が `Allow *` ベースでタグも付かない

**P1 で実行ロールのポリシーを 2 本に分けた（`my-kakeibo-v2-cfn-exec` と `my-kakeibo-v2-cfn-exec-app`）。**
- 分けた理由: 管理ポリシーの上限は 6,144 文字。P0 の分に Lambda / DynamoDB / Logs / API Gateway / IAM を足すと、1 本に収まらない
- 中身: `-cfn-exec` は P0 の分、IAM、Deny。`-cfn-exec-app` はアプリのリソース
- 代償: 付けるポリシーの ARN が増えるので、bootstrap をやり直す必要がある
- 文字数の上限はテストで確かめる

**P1 のリソースは名前を明示し、接頭辞 `my-kakeibo-v2-` で絞る。** CloudFormation が自動で付ける名前は、形式が保証されていない。
置換を伴うプロパティ（createOnly）を確かめたところ、名前以外にほとんど無いので、名前を固定しても置換で詰まりにくい。
- 作成はリクエストタグ `Project`、作成後は名前の接頭辞（API Gateway はリソースタグ）で限定する
- DynamoDB は `TableV2` ではなく `Table` を使う。`TableV2` は `AWS::DynamoDB::GlobalTable` になり、application-autoscaling などの余計な権限が要る
- テーブルは RETAIN で削除保護付き。実行ロールに DeleteTable を持たせない（Cognito の DeleteUserPool と同じ考え方）
- Lambda のロググループは `logGroup` プロップで明示的に作る。`logRetention` は、`Resource: "*"` の権限を持つカスタムリソースが生成されるので使わない

**permissions boundary（`my-kakeibo-v2-boundary`）は、自テーブルの項目操作と、自ロググループへの書き込みだけを許可する。**
`logs:CreateLogGroup` は許可しない。保持期間が無期限のロググループが勝手に作られるのを防ぐため。

**実行ロールに渡せるロールは Lambda 向けだけ。** PassRole は `iam:PassedToService=lambda.amazonaws.com` に限る。
次の 3 つは明示的に Deny する。
- boundary の無いロールの作成
- boundary の取り外し
- boundary と実行ロール用ポリシーの書き換え

**権限セットには、リソースのインポートを拒否する Deny を足す。** 実行ロールが他 PJ のリソースに触れる経路は 2 つある。
テンプレートに他 PJ の ID を書く経路は、名前・path・タグの条件で塞ぐ。インポートの経路は、この Deny で塞ぐ。
`lambda:InvokeFunction` は人に持たせない。JWT authorizer を迂回できるため。

**`cdk gc` には `--toolkit-stack-name` を必ず付ける。** CLI の gc は cdk.json の `toolkitStackName` を読まず、
付け忘れるとアカウント共用の `CDKToolkit` のバケットを掃除してしまう。

**IAM ポリシーの ARN のアカウント欄には変数を書けない。** ポリシー変数はリソース部分と Condition の値でしか使えないので、アカウント欄は `*` にし、
ロール名・バケット名の中の `${aws:PrincipalAccount}` で限定する。リポジトリに実アカウント ID を書かないため。

**旧 my-kakeibo で権限を絞れなかったのは Amplify Gen2 のバックエンドのため（推測）。** Gen2 は実行者の権限で CDK を回してバックエンド一式を作るので、
広い権限（`AmplifyBackendDeployFullAccess` など）が要る。v2 は Amplify を Hosting だけに使うので当たらない。
Hosting のデプロイ方式（Git 連携か、zip の手動デプロイか）は P2 で決める。Git 連携なら初期接続だけ広い権限、手動なら `amplify:CreateDeployment` 等をアプリ限定で足す。

## フロントエンド

**旧コードをコピーし、Next を最新化して静的書き出し（`output: 'export'`）にし、データ層を REST に差し替える。**
- データはクライアントから JWT 付きで取るので SSR の利点がない。静的書き出しなら Amplify Hosting の構成も単純
- 動的ルート（`/transactions/[id]`）は `?id=` 方式に書き換える
- Amplify Gen2 バックエンドはやめ、CDK で作る Cognito に `aws-amplify` の Auth を手動設定でつなぐ（ログイン UI を流用できる）
- 旧コードで Amplify Data クライアントを直接呼んでいる14ファイルは、API クライアントのモジュール1つに寄せる
- 目的は Go なので、フロントは「旧資産を最小の手間で動かす」

**ホスティングは Amplify Hosting。**

## 機能とデータモデル

**口座間振替は TRANSFER という取引種別にし、振替元・振替先の口座を持たせる。**
旧実装は振替を「支出」と「収入」の2件で登録し、カテゴリに一覧の先頭を自動で入れていた。そのせいで月次収支とカテゴリ分析に
振替が混ざり、2件が紐づいていないので片方だけ消すと残高がずれた。v2 では振替を分析から外し、残高計算にだけ反映する。

**クレジットカードはカードごとに口座を作る。** 支出は使った日にカード口座から出す。引き落としは銀行 → カードの振替。
カード口座の残高がマイナスなら、それが未払い額。総資産は「銀行 ＋ 現金 − カードの未払い」になる。

**カテゴリは明細単位で付ける。** 「食料品」「嗜好品」「外食」のようなカテゴリを作り、1枚のレシートに食料品と嗜好品が混在する前提。
カテゴリの階層は今は作らない（DynamoDB なら後から親カテゴリの項目を足すだけで済む）。

**カテゴリは収入用（INCOME）と支出用（EXPENSE）に分け、作成後は種別を変えられない（P1 で決定）。**
旧実装には区別がなく、問題が 3 つあった。
- 入金・振替のカテゴリに一覧の先頭が自動で入り、月次収支とカテゴリ分析が狂った
- 収入・支出のどちらの入力画面でも全カテゴリを選べた
- 分析は取引種別で絞るしかなかった

区別すると、次のようになる。
- 支出の明細の選択肢は支出用だけになる
- jev に渡す選択肢も支出用だけになる（「おにぎり」が「利息」に分類されることが構造上起きず、選択肢が減る分トークンも減る）
- 分析で収入と支出が混ざらない
- 振替（TRANSFER）にはカテゴリを付けない

種別を変えられないのは、変えると過去の明細が逆の種別のカテゴリを指して矛盾するため。間違えて作ったらアーカイブして作り直す。
代償は 2 つ。
- 返品を「支出カテゴリの収入」として書けないので、元の取引を修正する。翌月の返品だと元の月の数字が後から変わるが、許容する
- 両方で使う概念は 2 つ作る（ポイントをもらうのは収入用。使うのは支出用のカテゴリで、ポイント口座から出す）

不変条件:
- アーカイブされていない同じ種別の中で、名前を重複させない（jev の choice は選択肢名を map のキーにするため）
- アーカイブされていない支出用は 255 件まで（jev の choice の上限）

**口座・カテゴリは物理削除せず、アーカイブ（`archivedAt`）だけにする（P1 で決定）。** アーカイブはいつでも解除できる。
- アーカイブすると、選択肢・jev の選択肢・一覧の既定表示から外れる。過去の取引の表示、残高、分析には残る
- 旧実装は参照を確かめずに物理削除していたので、取引が宙に浮いた
- 「未使用なら物理削除」も検討した。採らなかったのは 2 点のため
  - 参照を確かめるには、明細の categoryId に GSI が要る
  - 確かめてから消すまでの間を原子的にできない
- 物理削除が無いので、取引を書くときに「参照先が存在するか」を読んでから書くだけで安全に確かめられる

**金額は円の整数（Go の `int64`）、取引日は `YYYY-MM-DD`（タイムゾーンなし）、`createdAt` / `updatedAt` は RFC 3339 の UTC。**
旧実装は金額を float で持っていた。取引日を文字列にすると、辞書順がそのまま日付順になる。

**P2 の取引に向けて決めたこと（実装は P2）。**
- 収入も明細形式にする（通常は 1 行）。取引の形を 1 種類にして、Go のコードと検証を分岐させない
- 明細は 1 円以上。値引きは対象の品目に合算する。マイナスの明細を許すと、品目別の単価分析と入力検証が複雑になる
- 1 取引の明細は 90 件まで。TransactWriteItems は 1 回 100 アクションまでで、編集時は「ヘッダ 1 + leg 2 + 新旧で多い方の明細数」になるため（理論上の限界は 97）
- 下書きは残高に含めない。口座の動き（leg）は確定するときに作る

**カテゴリには説明文を持たせる。** jev に選択肢の説明として渡すので、分類精度に直結する。
「コンビニ弁当は食料品か外食か」のような境界も説明文に書く。

## DynamoDB のキー設計（P1 で決定）

主キーは後から変えられないので、P1（口座・カテゴリ）の時点で P2〜P4（取引・明細・振替・下書き・月次集計・残高）まで見越して決めた。

**テーブルは 1 つ（シングルテーブル）。** テーブル名は `my-kakeibo-v2-data-v1`。
- 取引と明細を 1 回の Query で取れる（DynamoDB には JOIN が無い）
- テーブルごとに IAM・CDK の定義が増えない
- キー設計の学習という DynamoDB を選んだ理由にも合う
- 名前を明示しているので、キー設計を変えるときは末尾の版を上げて別テーブルとして作り直す

**主キーは ID だけで組み、日付を入れない。日付順の取得は GSI に任せる。**

| 項目 | PK | SK | GSI1PK / GSI1SK | GSI2PK / GSI2SK |
|---|---|---|---|---|
| 口座 | `USER#{sub}` | `ACCOUNT#{id}` | なし | なし |
| カテゴリ | `USER#{sub}` | `CATEGORY#{id}` | なし | なし |
| 取引ヘッダ（P2） | `USER#{sub}` | `TXN#{id}` | `USER#{sub}#TXN` / `{date}#{id}` | 下書きの間だけ `USER#{sub}#DRAFT` / `{createdAt}#{id}` |
| 明細（P2） | `USER#{sub}` | `TXN#{id}#LINE#{001..}` | `USER#{sub}#TXN` / `{date}#{id}#LINE#{nnn}` | なし |
| 口座の動き（leg、P2。確定した取引だけ） | `USER#{sub}` | `TXN#{id}#LEG#{1\|2}` | `USER#{sub}#ACCOUNT#{accountId}` / `{date}#{id}#{1\|2}` | なし |
| 品名辞書（P3） | `USER#{sub}` | `DICT#{品名}` | なし | なし |

- **日付を変えても、GSI1 のキーを書き換えるだけで済む。**
  - SK に日付を入れる案（`TXN#{date}#{id}`）だと、月別一覧を強い整合で読める利点はある
  - しかし主キーは UpdateItem で変えられず、日付を変えるたびに全項目の Delete + Put が要る。明細の上限も約 48 件に下がる
- **取引 ID での取得は、ベーステーブルの `begins_with(SK, "TXN#{id}")` 1 回で、ヘッダ・明細・leg がそろう。**
- **月別の一覧は GSI1 の `begins_with(GSI1SK, "2026-10")` で引き、Go 側で集計する。** 複数月は `BETWEEN` 1 回で済む
- **振替は leg を 2 つ持つ**（振替元はマイナス、振替先はプラス）。口座別の一覧を、振替元・振替先の区別なく GSI1 の 1 回の Query で引ける。
  ヘッダの accountId と toAccountId にそれぞれ GSI を張る案は、GSI が 2 つ要り、Go 側で結果をマージする必要がある
- **下書きは GSI2 で引く。** 下書きの間だけ GSI2 のキーを持たせ、確定時に REMOVE すると索引から消える（sparse index）
- **GSI1 と GSI2 は、P1 のテーブル作成時に両方作る。**
  - CloudFormation は 1 回の更新で GSI を 1 つしか作成・削除できない
  - キー属性を持つ項目が無い GSI には書き込みが発生しない
- 明細をヘッダの List 属性に埋め込む案は有力な次点だった。原子性が 1 項目で済み、コードも最も単純になる。
  ただし P4 の品目横断の GSI が張れず、振り返りの 1 タップでも項目全体を書き直すことになる
- 全項目に素の `type` と `userId` を持たせる。P4 で複数属性キーの GSI を後付けするとき、合成キーを埋め直さずに済む

**残高は保存せず、`openingBalance + Σ leg.amount` で毎回計算する。**
口座項目に残高を持たせて更新する案は採らなかった。理由は 3 つ。
- 編集のたびに旧口座と新口座の差分を計算する必要があり、誤ると気づかないままずれる
- 同じ口座項目を同時に更新するとトランザクションが失敗する
- キャンセルされたトランザクションも容量を消費する

leg は小さい項目なので、全期間を読んでも軽い。P4 で遅くなったら、再計算できるキャッシュとして後から足す。

**ID はサーバーが UUIDv7 で採番する（Go 1.27 標準の `uuid` パッケージ）。** 時刻順に並び、外部ライブラリが要らない（ULID だと要る）。
パスの ID は Parse で検証する。キーの区切り文字 `#` が入り込むのもこれで防げる。

**書き込みには条件を付ける。** 作成は `attribute_not_exists(PK)`、更新は `attribute_exists(PK)`。
UpdateItem は対象が無いと項目を新規作成してしまうため。他人の ID を指定したときも、条件が失敗して 404 になる。

## API（Go）の構成（P1 で決定）

**main を 2 つに分ける。** `cmd/api` はローカル用（インメモリのストア、開発用の認証）、`cmd/lambda` は本番用（DynamoDB、JWT）。
1 つの main で実行環境を判定する案は、開発用の認証が Lambda のバイナリにも入るため、判定を誤ると認証が外れる。
そこで、`cmd/lambda` が `internal/devauth` に依存していないことを `scripts/check.sh` で検査する。

**Lambda と net/http の間のアダプタは自前で書く。** `aws-lambda-go` の `events` 型だけに依存する。
- `aws-lambda-go-api-proxy` は 2024-12 にアーカイブされている
- `algnhsa` は最終リリースが 2023-12
- 60〜80 行で書けて、`http.ResponseWriter` を実装するので、Go のインターフェースの学習題材にもなる

**ローカルの実行とハンドラーのテストは、インメモリのストアで行う。** 実 DynamoDB は使わない（環境が 1 つなので本番データを汚す）。
DynamoDB の実装は、インメモリと同じ契約テストを DynamoDB Local で流して確かめる（CI ではサービスコンテナで動かす）。

**ストアのインターフェースは使う側（`httpapi`）で定義する。** Go の慣習（「インターフェースは受け取る側で定義する」）に従う。

**ビルドは CDK の `GoFunction`（alpha）を使う。** synth のたびにビルドするので、ビルドし忘れた古いバイナリを deploy する事故が起きない。
`go build` して `Code.fromAsset` に渡す案は、ビルドの手順に頼ることになる。

## 認可（P1 で決定）

**API に送るのはアクセストークンにし、スコープ `aws.cognito.signin.user.admin` を必須にする。**
- HTTP API の JWT authorizer は `token_use` を見ないので、ID トークンもアクセストークンも通してしまう
- スコープを必須にすれば、スコープを持たない ID トークンを API Gateway の段階で拒否できる
- Go 側でも `token_use != "access"` は 401 にする
- authorizer には既存の App Client を渡す。渡さないと CDK が client を新しく作る

**userId（`sub`）によるデータ分離を、構造で担保する。**
- userId の出どころは、authorizer が検証した claims だけにする。claims が無い、または `sub` が空なら 401
  （authorizer の付いていないルートができてしまった場合に備える）
- 入力の型に userId を持たせない。JSON は未知のフィールドを拒否するので、本文に userId を入れると 400
- ストアの全メソッドが userId を受け取り、PK を組む関数は 1 つだけにする
- 一覧は `Query PK=USER#{sub}` で取得する（Scan は使わない）
- CDK のテストで、すべてのルートに JWT authorizer が付いていることを確かめる。ルートはメソッドを明示し、`ANY` / `$default` は使わない

**アクセスログは P1 では付けない。** 有効にするには `Resource: "*"` の Logs の権限が要るため、必要になったら権限の設計と一緒に足す。

## レシート解析

**非同期の下書き方式。** S3 に画像を置く → S3 イベントで解析 Lambda が起動 → OCR → jev で分類 → 下書きの取引として保存 →
アプリで確認・修正して確定。取引に状態（下書き / 確定）を持たせる。
入口がスマホ撮影・Slack 取り込み（将来）・固定費の自動登録（将来）と増えても、「S3 に置く」か「下書きを作る」だけで済み、
確認画面は1つで足りる。旧実装の同期方式（API で解析 → フォームに反映）だと Slack 取り込みで作り直しになる。

**OCR は Bedrock（Claude）、分類は jev（TypeSafe System One）。** 読み取りは生成が必要なので LLM、分類は型付きの判断なので jev。
jev の `choice` 型（最大255択）でカテゴリ一覧を選択肢にし、明細1件につき1問を投げる。レシート1枚を1回のリクエストで処理できる。
推定は初期値として入れ、人が変更できるようにする。`confidence` の低い明細は目立たせる。
jev には Go の SDK がないので `net/http` で直接叩く（JSON と HTTP クライアントの練習になる）。
jev は 2026-09-15 公開で Claude の知識にない。仕様は記憶で書かず、docs.typesafe.ai を一次情報にする（P3 で `docs/LLM.md` にまとめる）。

**修正の記憶（品名 → カテゴリの辞書）を持つ。** ユーザーが推定を直したら辞書に入れ、次回からその品名は jev を呼ばずに辞書を使う。
使うほど直す手間が減る。

## 分析

**一番知りたいのは「何に使いすぎているか」「カテゴリ別の支出」「食料品か嗜好品か」「無駄遣い」。**

**品目単位の分析をする。** 明細の品名がすべて残るので、品目別のランキングが出せる。品名の表記揺れは、最初は完全一致でまとめる。

**無駄遣いは機械で判定せず、振り返りで本人が決める。** 「なんでこんな物を買ったんだ」は本人の基準次第で、AI に推定させる根拠がない。
- 振り返り機能: 候補（嗜好品・その他、初めて買った品目、一定額以上）に「満足 / 無駄だった」を1タップで付ける。
  「無駄だった」の月別合計を分析に出す。同じ品目をまた買ったら、確定画面で「前回『無駄だった』にした商品」と表示する
- 積み重ねの可視化: 品目別・店別に「回数 × 金額」を月単位で集計し、単価が安いのに月合計が大きいものを上位に出す
- 明細に「振り返り結果」（未評価 / 満足 / 無駄だった）の項目を持たせる

**予算は後回し。** 使う設定にしたときだけ表示する。

## 段階

| 段階 | 内容 |
|---|---|
| P0 | ハーネス、GitHub リポジトリ、CDK の骨組み（タグ付け）、Cognito |
| P1 | Go の API で口座・カテゴリの CRUD。最初のファイルで Go のレクチャーと虫食いをやる |
| P2 | 取引・明細・振替。フロントも Next の静的書き出しへ移行 |
| P3 | レシート: スマホ撮影 → OCR（Bedrock）→ jev → 下書き → 確定。修正の記憶 |
| P4 | 分析（カテゴリ別、品目別、積み重ね）、振り返り機能、資産状況 |
| 後回し | 固定費の自動登録、予算、Slack 取り込み、カレンダー |

**Go 学習の進め方。** 最初のバックエンドファイルで、Go の基本レクチャー → 虫食いにしてユーザーが埋める → 間違いを修正 →
レビュー → デプロイ。以降も Go の本体は Claude が書き切らず、シグネチャと TODO までを用意する形を基本にする（虫食い方式）。

**セキュリティの要所（Lambda のアダプタ、JWT の claims から userId を取る `RequireJWT`）も、本人が書く（P1 で決定）。**
誤りを防ぐため、Claude が厳しめのテスト（claims なし、`token_use=id` など）を先に用意する。
本体を書いた後に、security-reviewer がデータ分離と合わせてレビューする。

## ハーネス

**最小構成 ＋ 公開リポジトリ向けの必須ガードで始め、運用で育てる。** penusta-ticket の方式。
改善の候補は台帳（非公開の `notes/harness-backlog.md`）に記録し、2回発生したら hook / skill への昇格を提案する。
別プロジェクトで使っている重いガード（レビューと infra ハッシュを結び付けたデプロイ制御、サブエージェント起動回数の予算など）は最初は入れない。

**deploy の直前に `deploy-reviewer` が `cdk diff` を読んで判定する（2026-10-18 にユーザーと決定）。** メインが `my-kakeibo-v2` で deploy してよくなったため。
ソースのレビュー（code / security / cost-reviewer）とは見るものが違う。こちらが見るのは、AWS 上で実際に何が置換・削除されるか、新しいリソース型が実行ロールで許可されているか、IAM ロールに boundary が付くか。
別プロジェクトでは、レビューの印とハッシュで deploy を deny する hook も入れている。しかし、その印は「レビューを実行した」ことしか保証しないうえ、メインの deploy には guard-deploy の確認が毎回出る。
そこで今はエージェントと手順だけにし、hook での強制は「レビューを通さない deploy」が起きたら入れる（台帳）。

初日から入れるもの:
- Go の学習ルール（Go の本体は implementer に任せない。CDK と Next.js は通常どおり委譲）
- アカウント ID・ARN 等のハードコード検知（guard-hardcode）
- `.env` の読み取り禁止、`.claude/state` 等の .gitignore
- デプロイはユーザーが手で行う
- 外部 API は `USE_REAL_API` 方式を Go のインターフェースで実装し、モックが既定
- 品質チェックを1コマンドに（gofmt / go vet / go test、tsc / eslint、cdk synth）。公開リポジトリなので GitHub Actions の CI も置く

**レビュー系エージェントの fable への格上げは、メインが判断する。** ユーザーの指示を待たない。基準は「間違えると問題になる
（セキュリティ事故・課金の暴走・データ不整合）」か「間違ったまま進むと後々の影響が大きい（データモデル・キー設計・認証方式・IAM の土台）」。
代表例と、Fable が使えないときの代替手順は CLAUDE.md の「fable への格上げ」。

**秘密情報の混入は、hook が一次防衛、gitleaks が最終防衛。** hook（guard-hardcode / guard-dotenv）は Edit / Write と Bash の読み取りしか見られず、
python での読み取りや Bash でのファイル生成はすり抜ける。`scripts/check.sh secrets`（履歴と作業ツリー）を CI とコミット前に通す。

**決定の記録はこのファイル（DECISIONS.md）1つにまとめ、理由を厚く書く。** 記事の素材としても読みやすい。

**連載は Zenn に出す（2026-10-27 にユーザーが Qiita から変更）。** 別のプロジェクトで Qiita に書いているので、媒体を分ける。
構成は penusta-ticket に合わせる: `docs/article-series.md`（連載の方針・構成表）と `docs/articles/`（管理表の `README.md` と各回の下書き）。
