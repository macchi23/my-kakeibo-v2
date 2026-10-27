個人用の家計簿アプリ。Go（Lambda, `provided.al2023` arm64）+ API Gateway HTTP API + DynamoDB + Cognito / CDK（TypeScript）/ Next.js（静的書き出し）+ Amplify Hosting。リージョンは us-east-1。
目的は Go の学習（ユーザーは Go 未経験）。GitHub で公開し、Zenn 連載も書く。

作業を始める前に読む:

| ファイル | いつ |
|---|---|
| `notes/handover.md` | 毎回最初に（現状・環境の注意点・次にやること）。`notes/` は非公開のメモ用リポジトリ（下記） |
| `docs/setup.md` | AWS への反映・再構築の手順を確かめるとき |
| `docs/DECISIONS.md` | 設計を変えたくなったとき、新しい領域に入るとき |
| `docs/go/README.md` | Go のコードに触れるとき |
| `docs/LLM.md` | Bedrock / jev に触れる前（P3 で作る） |
| `docs/article-series.md` | Zenn の記事を書くとき |

## 必須ルール

- **実 AWS への書き込みは、プロファイル `my-kakeibo-v2` で行う範囲ならメインが実行してよい**（`cdk diff` / `cdk deploy`、SSM・Cognito のユーザー操作など）。権限セットで、このプロジェクトのリソースにしか届かない。
  - 条件: コストか権限（IAM・ポリシー）がおかしくなる変更ではないこと。deploy の前に `scripts/check.sh` が通り、レビューで重大な指摘が無く、`deploy-reviewer` が「deploy 可」と判定していること。迷ったら実行せずにユーザーに聞く
  - **広い権限（`default` と共用の SSO プロファイル。名前は `notes/handover.md`）を使う操作はユーザーが行う**（ポリシーの作成・更新、bootstrap、destroy、スタックの削除、Identity Center）。メインはコマンドを示すだけ。読み取りはしてよい
  - サブエージェントは synth と読み取りまで（hook が拒否する）
- AWS プロファイルは `my-kakeibo-v2`
- アカウント ID・リージョン・ARN・バケット名・メールアドレスをコードに書かない。CDK は環境解決（`env` を渡さない、`Stack.of(this).region`）、Go は SDK の標準解決に任せる。値が要るものは CDK の context か SSM で渡す
- シークレット（jev の API キーなど）は SSM Parameter Store（SecureString）に置く。`.env` は読まない
- `.env` を含む文字列の置換・生成（sed、heredoc など）は Bash ではなく Edit / Write で行う。Bash だと guard-dotenv に止められる
- 外部 API（Bedrock / jev）はインターフェースの裏に置き、`USE_REAL_API=1` のときだけ実装を使う。既定はモック。`USE_REAL_API=1` の実行はユーザーの承認を得てから
- jev（2026-09-15 公開）と Bedrock の仕様は記憶で書かない。一次資料と `docs/LLM.md` を根拠にする
- AWS リソースには `Project=my-kakeibo-v2` タグを付ける（CDK は `Tags.of(app)`。CDK の外で作るものは個別に）
- 検証は `scripts/check.sh [go|infra|web|shell|hooks|secrets|all]`。終了コード 0 = 通過 / 1 = 失敗 / 2 = 実行不能。秘密情報の混入は hook が一次防衛、gitleaks（`secrets`）が最終防衛
- リポジトリは公開前提。コミット前に `git status --short` で入れてはいけないファイルが無いか確かめる
- **push 済みの値は force push では消えない。** GitHub は書き換え前のコミットを残し、Activity に force push の記録も残す。公開してはいけない値を push してしまったら、履歴の書き換えや amend で済ませず、リポジトリの作り直し（ユーザーが削除）か GitHub Support への依頼で消す
- **`notes/` は非公開のメモ用リポジトリ**（公開側の `.gitignore` で除外。自身は別の private リポジトリ）。現状・台帳・環境の具体名（認証主体）・参照元のプロジェクト名はここに書き、公開側の docs・コード・コミットには書かない。penusta-ticket 以外の参照元は公開側では「別プロジェクト」と書く。notes の変更は公開側とは別にコミットする

## Go の学習

`docs/go/README.md` の進め方に従う（虫食い方式）。要点:

- **Go の関数本体は Claude が書き切らない。** メインが型定義・シグネチャ・解説コメント・`// TODO:` までを用意し、ユーザーが埋める。implementer にも Go の本体は書かせない
- ユーザーが「ヒント」と言ったら、ヒント → 部分的な答え → 完全な答え の順で出す
- 書けたらレビューし、より Go らしい書き方があれば示す
- 新しい概念は `docs/go/` に書いてから、会話では要点だけ触れる。1 回のやりとりは 1 機能・数十行
- 例外: ユーザーが明示的に「書いて」と言った部分、テストの足場、生成コード

## 委譲

メインは分解・ブリーフ作成・結果の統合・ユーザーとの対話・Go の学習の伴走を担当し、作業はサブエージェントへ委譲する。
メインが直接やってよいのは、数行の自明な修正、ドキュメント修正、git 操作、Go の足場（型・シグネチャ・TODO）だけ。

| 作業 | エージェント | 既定モデル |
|---|---|---|
| コード探索（どこに何があるか） | `Explore` | haiku |
| 調査（設計比較・原因究明・外部仕様） | `researcher` | opus |
| 実装 + テスト（CDK・Next.js。Go は足場まで） | `implementer` | sonnet |
| check の実行と失敗の要約 | `verifier` | haiku |
| コードレビュー | `code-reviewer` | opus |
| セキュリティレビュー | `security-reviewer` | opus |
| コストレビュー | `cost-reviewer` | sonnet |
| deploy 直前の差分（cdk diff）の判定 | `deploy-reviewer` | sonnet |

呼び出し時のモデル上書き:

- `implementer` を opus に上げる: sonnet の実装が、同じ指摘でレビューを 2 回差し戻されたとき
- `implementer` を haiku に下げる: 「既存パターンの踏襲 / 1〜2 ファイル / 設計判断なし」をすべて満たすとき

### fable への格上げ（レビュー系）

**格上げはメインが判断する（ユーザーの指示を待たない）。** 基準は次のどちらか:

- 間違えると問題になる: セキュリティ事故、課金の暴走、データ不整合
- 間違ったまま進むと後々の影響が大きい: データモデル・DynamoDB のキー設計・認証方式・IAM の土台など、後から直しにくいもの

代表例（これに当たらなくても、基準に合えば上げる）:

| レビュアー | 例 |
|---|---|
| `code-reviewer` | 金額・残高・振替の整合性、DynamoDB のキー設計 |
| `security-reviewer` | 認証・認可（Cognito、JWT authorizer、userId によるデータ分離）、IAM ポリシー、外部入力の受け口（画像アップロード、Slack 取り込み）、リポジトリを public にする前の確認 |
| `cost-reviewer` | 課金リソースの初追加、Bedrock・jev の呼び出し経路、S3 イベント → Lambda の連鎖、DynamoDB のキャパシティ設定、Budgets |

- 格上げしたら、報告に「fable に上げた理由」を 1 行書く
- **Fable が使えないとき**（エラー・提供終了）は opus で続行し、`notes/handover.md` の「モデル切り替え履歴」に日付と理由を書き、報告で明示する

## 標準の流れ

1. `Explore` で現状把握
2. 設計判断が要るなら `researcher`
3. メインが方針を提示
4. `implementer`
5. `verifier`
6. `code-reviewer`。セキュリティ・コストに触れる変更なら `security-reviewer` / `cost-reviewer` も並列で
7. 指摘を `implementer` へ戻す（最大 2 往復）
8. AWS に反映するなら `deploy-reviewer` → 「deploy 可」なら deploy（`my-kakeibo-v2`）
9. 結果報告

- ブリーフに書くこと: 目的 / 完了条件 / 対象ファイル / 既知の制約と除外済みの仮説 / 返してほしい形式
- レビューには実装者の説明を渡さない。仕様と diff の範囲だけ渡す
- 完了条件: `scripts/check.sh` が通る + レビューの重大指摘なし
- 同じファイルを触る `implementer` を並列に走らせない
- コミットはユーザーが指示したときにメインが行う。サブエージェントはコミットしない
- 1 ステップ分の 4〜8 は `/step` スキルでまとめて進める

## ハーネスの育成

hooks と skills は最小限から始め、運用で必要になったものを足す。候補の台帳は `notes/harness-backlog.md`（記録時と提案時だけ読む）。

タスク完了時、次のどれかに当たったら台帳に追記するか、既存行の発生回数を増やす:

- hook 候補: 同じルール違反が `verifier` かレビューで検出された
- skill 候補: 3 ステップ以上の同じ手順を繰り返し指示した / ユーザーが同じ指示を 2 回した
- 定義の修正候補: ブリーフ不足で手戻りした / 返却形式が守られなかった / モデル配分が合わなかった

サブエージェントの返却にある「ハーネスへの気づき」は台帳へ転記する。
発生回数が 2 に達した候補は、完了報告の末尾に 1 行で昇格を提案する。追加はユーザーの承認後。却下された候補は再提案しない。

## 一般規約

- **変更スコープ**: 依頼されたことだけを変更する。スコープ外の変更が必要に見えたら報告する（黙って適用しない）
- **エラーを握りつぶさない**: Go で `err` を捨てない（`_ = f()` で黙らせない）。TS で空の `catch` を書かない
- **IAM は最小権限**: `actions: ['*']` / `resources: ['*']` / `grantFullAccess` は使わない
- **環境変数・設定値**: 新しく必要な場合はユーザーに追加を依頼する。値を推測したりプレースホルダーを書いたりしない
- **コメント**: 何をしているかではなく、なぜそうするかを書く。自明なコードにはコメントしない（Go の学習用の解説コメントは例外）
- **ライブラリの版・API は前提にしない**: 公式ドキュメントか型定義で確かめる
- **Git**: コミットメッセージは日本語、`<種別>: <内容>`（feat / fix / docs / refactor / test / chore）。`git add -A` は使わず、ファイルを指定する
