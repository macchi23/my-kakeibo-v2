# Go 学習の進捗

| # | やること | 学ぶ概念 | 状態 |
|---|---|---|---|
| L1 | `internal/kakeibo`: 型と Validate、ID | package、名前付きの型、struct とメソッド、エラー、rune、テーブル駆動テスト | ⬜ |
| L2 | `internal/store/memory.go`: インメモリのストア | map、ポインタ、`sync.Mutex`、`errors.Is` | ⬜ |
| L3 | `internal/auth/context.go`（WithUserID・UserIDFrom）、`internal/devauth`、`internal/httpapi`: ハンドラーと応答 | インターフェース、`net/http`、`PathValue`、JSON、`errors.As` | ⬜ |
| L4 | `internal/auth`（WithClaims・RequireJWT）: JWT の claims から userId を取り出す | `context`、ミドルウェア（関数を返す関数） | ⬜ |
| L5 | `internal/store`: DynamoDB の実装 | AWS SDK、`attributevalue`、条件式、ページング | ⬜ |
| L6 | `internal/lambdahttp`: Lambda と net/http のアダプタ | インターフェースの実装（`http.ResponseWriter`）、base64 | ⬜ |

L3 まで終わると `go run ./cmd/api` でローカルの API が動く。L6 まで終わったら deploy する。

## ログ

（ステップごとに `### Step N — 題` → 何をしたか 1〜2 行 → 書いたコードの抜粋 → **学んだこと**。任意で「動作確認（わざと壊してみた）」「気づき」）
