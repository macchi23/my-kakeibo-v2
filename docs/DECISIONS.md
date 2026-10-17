# 設計判断とその理由

作り直したくなったら、まずここを読む。多くは一度議論して決めた結論で、
理由を知らずに変えると同じ失敗を繰り返す。

## 目的と前提

**動機は「Go を触ってみたい」。効率は重視しない。** 目標は「Go 使ったことあります（CC に書かせたけど多少わかる）」レベル。
判断に迷ったら「Go の学習になるか」「個人利用で十分か」を優先する。

**GitHub で公開し、Qiita 連載も書く。** 秘密情報（API キー、アカウント ID、Cognito の ID 等）をリポジトリに置かない構成にする。
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
旧実装はカテゴリに収入用・支出用の区別がなかった。v2 で扱いを決める（未決）。

**カテゴリには説明文を持たせる。** jev に選択肢の説明として渡すので、分類精度に直結する。
「コンビニ弁当は食料品か外食か」のような境界も説明文に書く。

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

**Qiita 連載は penusta-ticket の構成に合わせる。** `docs/article-series.md`（連載の方針・構成表）と
`docs/articles/`（管理表の `README.md` と各回の下書き）。
