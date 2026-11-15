# Go 学習の進捗

| # | やること | 学ぶ概念 | 状態 |
|---|---|---|---|
| L1 | `internal/kakeibo`: 型と Validate、ID | package、名前付きの型、struct とメソッド、エラー、rune、テーブル駆動テスト | ✅ 2026-11-15 |
| L2 | `internal/store/memory.go`: インメモリのストア | map、ポインタ、`sync.Mutex`、`errors.Is` | ⬜ |
| L3 | `internal/auth/context.go`（WithUserID・UserIDFrom）、`internal/devauth`、`internal/httpapi`: ハンドラーと応答 | インターフェース、`net/http`、`PathValue`、JSON、`errors.As` | ⬜ |
| L4 | `internal/auth`（WithClaims・RequireJWT）: JWT の claims から userId を取り出す | `context`、ミドルウェア（関数を返す関数） | ⬜ |
| L5 | `internal/store`: DynamoDB の実装 | AWS SDK、`attributevalue`、条件式、ページング | ⬜ |
| L6 | `internal/lambdahttp`: Lambda と net/http のアダプタ | インターフェースの実装（`http.ResponseWriter`）、base64 | ⬜ |

L3 まで終わると `go run ./cmd/api` でローカルの API が動く。L6 まで終わったら deploy する。

## ログ

（ステップごとに `### Step N — 題` → 何をしたか 1〜2 行 → 書いたコードの抜粋 → **学んだこと**。任意で「動作確認（わざと壊してみた）」「気づき」）

### Step L1 — `internal/kakeibo`: 型と Validate、ID（2026-11-15）

ValidationError の Error()、ID の採番と検証（NewID / ParseID）、口座とカテゴリの入力検査（Validate）、カテゴリの名前の重複・件数の上限・kind の変更不可を書いた。
途中で、口座種別・カテゴリ種別に `IsValid()` メソッドを生やして Validate から呼ぶ形に書き換えた（テストがあるので壊れていないことをすぐ確かめられた）。

```go
u, err := uuid.Parse(s)
if err != nil {
	return "", ValidationError{Field: "id", Message: "UUIDが不正です"}
}
return u.String(), nil
```

```go
func (k AccountKind) IsValid() bool {
	switch k {
	case AccountKindBank, AccountKindCash, AccountKindCard, AccountKindEMoney, AccountKindOther:
		return true
	}
	return false
}
```

**学んだこと**
- メソッドのレシーバ `(e ValidationError)` は Python の `self`。呼ぶときは `値.メソッド()`（`ValidateAccountKind(in.Kind)` ではなく `in.Kind.IsValid()`）
- 型ごとの「無い」: `string` は `""`、`error`・ポインタ（`*time.Time`）は `nil`
- 戻り値が `(string, error)` なら、どの `return` も 2 つ返す。失敗は早めに `return`、`else` は書かない
- struct の値は `{}` で作る（`ValidationError{...}`。`()` だと関数呼び出しと読まれて `missing ','`）
- import はスラッシュ区切り（`"unicode/utf8"`）、カンマを付けない。行末に `;` は書かない
- 文字数は `utf8.RuneCountInString`（`len` はバイト数）
- `const ( ... )` は定数を並べているだけで配列ではない。列挙の判定は `switch`（数個で固定なら定番）か map
- メソッド名にレシーバの型名を繰り返さない（`IsValid`。呼ぶ側が `in.Kind.IsValid()` と読める）
- 境界値の 1 つずれ（`>` と `>=`）は、境界の両側のテストケースで見つかる

**気づき**
- 最初は Python / PHP の癖（`nil` と `""` の取り違え、`()` で struct を作る、import のカンマ、行末の `;`）がそのままコンパイルエラーになった。エラー文の「ファイル:行:列」から 1 つずつ直せた
