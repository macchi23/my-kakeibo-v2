# Go の基本（L1 の前に読む）

P1 の最初のステップ（L1: `internal/kakeibo` の型と Validate）を書くのに要ることだけを扱う。
`net/http`、`context`、`sync.Mutex` などは、使うステップの直前に別のファイルで扱う。
TypeScript との対応は [`ts-to-go.md`](ts-to-go.md) に表でまとめている。

## 1. module と package

```
backend/
  go.mod                  ← module の宣言。module = 依存の単位（npm の package.json に近い）
  internal/kakeibo/       ← package kakeibo。ディレクトリ 1 つ = package 1 つ
    account.go
    category.go
```

- `go.mod` の 1 行目の `module github.com/macchi23/my-kakeibo-v2/backend` が import パスの先頭になる。
  ほかの package からは `import "github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"` で読み込み、`kakeibo.Account` のように **package 名.名前** で使う
- 同じディレクトリのファイルは、すべて同じ package になる。ファイルをまたいでも、import なしで互いの名前を使える（TS のファイル単位のモジュールとは違う）
- `internal/` の下は、この module の外から import できない（Go のツールが強制する）。外に見せない実装を置く場所

## 2. 公開・非公開は「先頭の文字」で決まる

```go
type Account struct { ... }   // 大文字で始まる → package の外から見える（TS の export）
func userPK(u UserID) string  // 小文字で始まる → package の中だけ
```

`export` や `public` のキーワードは無い。struct のフィールドも同じ規則で、JSON に出したいフィールドは大文字で始める
（`encoding/json` は別の package なので、小文字のフィールドを読めない）。

## 3. 型と変数

```go
var n int          // 宣言だけ。値はゼロ値（後述）の 0
name := "現金"     // := は宣言 + 代入。型は右辺から推論される（関数の中でだけ使える）
const MaxName = 50 // 定数
```

**名前付きの型**で、同じ中身でも意味の違う値を区別できる。

```go
type Yen int64       // 中身は int64 だが、別の型
type UserID string

var a Yen = 100
var b int64 = a      // コンパイルエラー。Yen と int64 は混ぜられない
var c int64 = int64(a) // 明示的に変換すれば OK
```

TS の `type Yen = number` は単なる別名なので混ぜられるが、Go の `type Yen int64` は別の型になる。
「userId を渡すべき所に口座 ID を渡す」ような取り違えを、コンパイラが止めてくれる。

**列挙型は無い。** 名前付きの型と定数で表す。

```go
type AccountKind string

const (
    AccountKindBank AccountKind = "BANK"
    AccountKindCash AccountKind = "CASH"
)
```

これで `AccountKind` 型の値を作れるのは定数だけ、とはならない（`AccountKind("FOO")` も作れる）。
だから JSON から読んだ値は、`Validate` で列挙値のどれかであることを確かめる。

## 4. ゼロ値

Go の変数は、初期化しなければ型ごとの**ゼロ値**になる。`undefined` や `null` で落ちることがない。

| 型 | ゼロ値 |
|---|---|
| 数値 | `0` |
| string | `""` |
| bool | `false` |
| ポインタ、スライス、map、インターフェース、関数 | `nil` |
| struct | 全フィールドがそれぞれのゼロ値 |

裏返すと、「値が 0 なのか、送られてこなかったのか」は区別できない。
`openingBalance` が JSON に無ければ 0 になる。今回はそれで困らないので、そのままにする。
区別したいときはポインタ（`*time.Time`）を使い、`nil` を「無い」とみなす（例: `ArchivedAt`）。

## 5. struct とメソッド

```go
type AccountInput struct {
    Name string `json:"name"`   // バッククォートの中は struct タグ（後述）
    Kind AccountKind `json:"kind"`
}

// (in AccountInput) が「レシーバ」。in.Validate() の形で呼べる
func (in AccountInput) Validate() error {
    ...
}
```

- class は無い。struct（データ）と、それに紐づく関数（メソッド）を別々に書く
- レシーバ `(in AccountInput)` は値のコピーを受け取る。メソッドの中で書き換えても、呼び出し元は変わらない
- 書き換えたいときはポインタレシーバ `(c *Category)` にする。L1 の Validate は読むだけなので値レシーバでよい

**struct タグ**は、ほかの package（`encoding/json`、DynamoDB の `attributevalue`）が読む付加情報。
`json:"name"` は「JSON ではキー `name` にする」、`json:"archivedAt,omitempty"` は「ゼロ値なら出力しない」。

## 6. エラーは戻り値で返す

例外（`throw` / `try`）は無い。失敗しうる関数は、最後の戻り値に `error` を返す。

```go
id, err := kakeibo.ParseID(s)   // 戻り値を 2 つ受け取る（多値返却）
if err != nil {
    return err                  // 呼び出し元に返す。握りつぶさない
}
// ここから先は id が正しい
```

- `error` は「`Error() string` メソッドを持つ型」なら何でもよい（インターフェース。L3 で詳しく）
- 単純なエラーは `errors.New("not found")` で作る。`var ErrNotFound = errors.New(...)` のように package の変数にしておくと、
  呼び出し側で `errors.Is(err, kakeibo.ErrNotFound)` と比べられる
- フィールドを持つエラーは、自分で struct を作り `Error()` メソッドを付ける（L1 の `ValidationError`）。
  呼び出し側は `errors.As(err, &ve)` で取り出す
- 包むときは `fmt.Errorf("get account: %w", err)`。`%w` で包むと、`errors.Is` / `errors.As` が中まで見てくれる
- `_ = f()` でエラーを捨てない（このプロジェクトのルール）

## 7. 文字列の長さは rune で数える

```go
len("食料品")                    // 9（バイト数。UTF-8 で 1 文字 3 バイト）
utf8.RuneCountInString("食料品") // 3（文字数）
strings.TrimSpace("  現金 ")     // "現金"
```

Go の string は UTF-8 のバイト列。`len` はバイト数を返すので、「名前は 50 文字まで」は `utf8.RuneCountInString` で数える。

## 8. slice と map と for

```go
cats := []Category{}          // slice（TS の配列に近い。長さは可変）
cats = append(cats, c)        // 追加。戻り値を受け取り直すのを忘れない

for i, c := range cats {      // インデックスと要素。要らない方は _ にする
    _ = i
}
for _, c := range cats { ... }

seen := map[string]bool{}     // map（TS の Map / Record に近い）
seen["食料品"] = true
if seen[name] { ... }         // 無いキーはゼロ値（false）が返る
```

`for` が唯一のループ（`while` も `for cond { }` で書く）。

## 9. テスト

```
category.go
category_test.go   ← _test.go で終わるファイルがテスト。go test が拾う
```

```go
func TestCategoryInputValidate(t *testing.T) {
    tests := []struct {         // テーブル駆動テスト: ケースを slice に並べる
        name    string
        in      CategoryInput
        wantErr bool
    }{
        {"正常", CategoryInput{Name: "食料品", Kind: CategoryKindExpense}, false},
        {"名前が空", CategoryInput{Kind: CategoryKindExpense}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {   // ケースごとに名前付きのサブテスト
            err := tt.in.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

- assert ライブラリは使わず、`if` と `t.Errorf` で書くのが標準的
- `go test ./internal/kakeibo/` でその package だけ、`-run TestCategory` で名前を絞れる

**このプロジェクトの虫食いの進め方**: テストは Claude が先に書いてある。各テスト関数の先頭にある
`t.Skip("TODO(L1): ...")` の行を消すと、そのテストが動き出す。テストが通るように `panic("TODO(L1)")` の所を埋める。

## 10. 道具

```sh
cd backend
go build ./...     # コンパイルが通るか
go vet ./...       # 怪しい書き方の検出
gofmt -w .         # 整形（書き方の議論をしなくて済むよう、整形は 1 通りに決まっている）
go test ./...      # テスト
go doc strings.TrimSpace   # 標準ライブラリのドキュメント
```

コンパイルエラーは「ファイル:行:列: 内容」の形で出る。上から 1 つずつ直すのが早い
（Go は使っていない変数や import もエラーにする）。
