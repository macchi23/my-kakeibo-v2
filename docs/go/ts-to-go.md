# TypeScript → Go の対応表

知っている TypeScript の書き方から、Go の書き方を引くための表。説明は [`basics.md`](basics.md) と各ステップの資料にある。
ステップが進んで新しい概念が出たら、行を足す。

## 基本

| やりたいこと | TypeScript | Go |
|---|---|---|
| 変数 | `let n = 1` / `const s = "a"` | `n := 1`（関数内）/ `var n int` / `const s = "a"` |
| 型の別名 | `type Yen = number`（混ぜられる） | `type Yen int64`（別の型。混ぜるには変換が要る） |
| 列挙 | `type Kind = "BANK" \| "CASH"` | `type Kind string` + `const ( KindBank Kind = "BANK" ... )` |
| 公開 | `export` | 名前を大文字で始める |
| null / undefined | `null` / `undefined` | ゼロ値。「無い」を表すならポインタの `nil` |
| オプショナル | `archivedAt?: Date` | `ArchivedAt *time.Time` + `json:",omitempty"` |
| 文字列の長さ | `s.length`（UTF-16 単位） | `utf8.RuneCountInString(s)`（`len(s)` はバイト数） |
| テンプレート文字列 | `` `USER#${id}` `` | `"USER#" + id` / `fmt.Sprintf("USER#%s", id)` |

## 構造とメソッド

| やりたいこと | TypeScript | Go |
|---|---|---|
| データの型 | `interface Account { name: string }` | `type Account struct { Name string }` |
| メソッド | `class` の中に書く | `func (a Account) Label() string { ... }`（struct の外に書く） |
| 書き換えるメソッド | `this.name = ...` | ポインタレシーバ `func (a *Account) Rename(n string)` |
| インターフェース | `implements` を書く | 書かない。メソッドがそろっていれば満たす（L3 で） |
| JSON のキー名 | そのまま | struct タグ `` `json:"name"` `` |

## エラー

| やりたいこと | TypeScript | Go |
|---|---|---|
| 失敗を伝える | `throw new Error("x")` | `return errors.New("x")`（最後の戻り値） |
| 受け取る | `try { ... } catch (e) { ... }` | `v, err := f(); if err != nil { ... }` |
| 種類で分ける | `e instanceof NotFoundError` | `errors.Is(err, ErrNotFound)` / `errors.As(err, &ve)` |
| 包む | `new Error("x", { cause: e })` | `fmt.Errorf("x: %w", err)` |

## コレクション

| やりたいこと | TypeScript | Go |
|---|---|---|
| 配列 | `const a: T[] = []` | `a := []T{}` |
| 追加 | `a.push(x)` | `a = append(a, x)` |
| ループ | `for (const x of a)` | `for _, x := range a` |
| 辞書 | `new Map<string, T>()` / `Record<string, T>` | `map[string]T{}` |
| キーの有無 | `m.has(k)` | `v, ok := m[k]` |
| filter / map | `a.filter(...)` | 標準では無い。`for` で書く（`slices` パッケージに一部ある） |

## テスト

| やりたいこと | TypeScript（vitest） | Go |
|---|---|---|
| テストファイル | `foo.test.ts` | `foo_test.go` |
| テスト | `it("...", () => {})` | `func TestFoo(t *testing.T)` |
| ケース分け | `it.each([...])` | テーブル駆動 + `t.Run` |
| 失敗 | `expect(x).toBe(y)` | `if x != y { t.Errorf(...) }` |
| 飛ばす | `it.skip` | `t.Skip("理由")` |
