# Python / PHP → Go の対応表

知っている Python（必要なら PHP）の書き方から、Go の書き方を引くための表。説明は [`intro.md`](intro.md)、[`basics.md`](basics.md) と各ステップの資料にある。
ステップが進んで新しい概念が出たら、行を足す。

## 基本

| やりたいこと | Python | PHP | Go |
|---|---|---|---|
| 変数 | `n = 1` | `$n = 1;` | `n := 1`（関数内）/ `var n int` |
| 定数 | `MAX = 50`（慣習） | `const MAX = 50;` | `const Max = 50` |
| 型の注釈 | `n: int = 1`（任意） | `int $n`（引数など） | 必須（`:=` なら推論） |
| 関数 | `def add(a: int, b: int) -> int:` | `function add(int $a, int $b): int` | `func add(a int, b int) int` |
| 複数の戻り値 | `return v, err`（タプル） | 配列で返す | `return v, err`（言語の機能） |
| 型に名前を付ける | `Yen = int`（別名。混ぜられる） | - | `type Yen int64`（別の型。混ぜるには変換が要る） |
| 列挙 | `class Kind(Enum): BANK = "BANK"` | `enum Kind: string { case Bank = 'BANK'; }` | `type Kind string` + `const ( KindBank Kind = "BANK" ... )` |
| 公開・非公開 | `_name`（慣習） | `public` / `private` | 名前を大文字で始めると公開 |
| 「無い」 | `None` | `null` | ゼロ値（`0`、`""`、`false`）。「無い」を区別したいならポインタの `nil` |
| 文字列の長さ | `len(s)`（文字数） | `mb_strlen($s)` | `utf8.RuneCountInString(s)`（`len(s)` はバイト数） |
| 文字列の組み立て | `f"USER#{id}"` | `"USER#{$id}"` | `"USER#" + id` / `fmt.Sprintf("USER#%s", id)` |
| 条件分岐 | `if x > 0:` | `if ($x > 0) {` | `if x > 0 {`（括弧なし、波括弧は必須） |

## 構造とメソッド

| やりたいこと | Python | PHP | Go |
|---|---|---|---|
| データの型 | `@dataclass class Account: name: str` | `class Account { public string $name; }` | `type Account struct { Name string }` |
| メソッド | class の中に `def label(self):` | class の中に `function label()` | `func (a Account) Label() string { ... }`（struct の外に書く） |
| `self` / `$this` | `self` | `$this` | レシーバ（上の `a`）。名前は自由で、型名の頭文字 1 字が慣習 |
| 書き換えるメソッド | `self.name = ...` | `$this->name = ...` | ポインタレシーバ `func (a *Account) Rename(n string)` |
| コンストラクタ | `__init__` | `__construct` | 無い。`NewAccount(...)` という普通の関数を作る慣習 |
| 継承 | `class B(A):` | `class B extends A` | 無い。必要なら struct の埋め込み（この家計簿では使わない） |
| インターフェース | `Protocol`（ダックタイピング） | `implements` を書く | 書かない。メソッドがそろっていれば満たす（L2 の `l2-memory.md` 6 節） |
| 書き換えるメソッド・コピーしてはいけない型 | - | - | ポインタレシーバ `func (m *Memory) ...`（`sync.Mutex` を持つ型など） |
| JSON のキー名 | `dict` のキーそのまま | 配列のキーそのまま | struct タグ `` `json:"name"` ``（フィールドは大文字で始める） |

## エラー

| やりたいこと | Python | PHP | Go |
|---|---|---|---|
| 失敗を伝える | `raise ValueError("x")` | `throw new Exception("x");` | `return errors.New("x")`（最後の戻り値） |
| 受け取る | `try: ... except ValueError as e:` | `try { ... } catch (Exception $e) { ... }` | `v, err := f(); if err != nil { ... }` |
| 種類で分ける | `except NotFoundError:` / `isinstance(e, ...)` | `catch (NotFoundException $e)` | `errors.Is(err, ErrNotFound)` / `errors.As(err, &ve)` |
| 包む | `raise X("x") from e` | `new Exception("x", 0, $e)` | `fmt.Errorf("x: %w", err)` |
| 後始末 | `finally:` / `with` | `finally` | `defer f.Close()` |
| 決まったエラー | `class NotFoundError(Exception)` | `class NotFoundException extends Exception` | `var ErrNotFound = errors.New("not found")` |

## 値・参照・並行処理（L2 から）

| やりたいこと | Python | PHP | Go |
|---|---|---|---|
| オブジェクトの代入 | `b = a`（同じものを指す） | `$b = $a`（オブジェクトは同じものを指す） | `b := a`（struct は中身がコピーされる） |
| 同じものを指す | （最初から参照） | （オブジェクトは最初から参照） | ポインタ `p := &a`、型は `*T` |
| 参照の先を読む | - | - | `*p`（struct のフィールドは `p.Name` でそのまま読める） |
| 浅いコピーの落とし穴 | `copy.copy` は中のリストを共有 | `clone` は中のオブジェクトを共有 | struct をコピーしても中のポインタ（`*time.Time` など）は共有 |
| 鍵（排他） | `with lock:`（`threading.Lock`） | （通常は使わない） | `mu.Lock(); defer mu.Unlock()`（`sync.Mutex`） |
| 空の dict を作る | `{}` / `dict()` | `[]` | `map[K]V{}` / `make(map[K]V)` |
| 宣言だけの dict | - | - | `var m map[K]V` は nil。読むのは安全、書くと panic |
| dict の順番 | 挿入順が保たれる | 挿入順が保たれる | 毎回ばらばら。並べたいなら `slices.Sorted(maps.Keys(m))` |
| 並べ替え | `sorted(xs, key=lambda a: a.id)` | `usort($xs, fn($a, $b) => $a->id <=> $b->id)` | `slices.SortFunc(xs, func(a, b T) int { return cmp.Compare(a.ID, b.ID) })` |
| その場の関数 | `lambda a: ...`（1 式） | `fn($a) => ...` / `function ($a) { ... }` | `func(a T) int { ... }`（複数行書ける） |

## コレクション

| やりたいこと | Python | PHP | Go |
|---|---|---|---|
| リスト | `a = []` | `$a = [];` | `a := []T{}`（slice。要素の型を決める） |
| 追加 | `a.append(x)` | `$a[] = $x;` | `a = append(a, x)`（戻り値を代入し直す） |
| 長さ | `len(a)` | `count($a)` | `len(a)` |
| ループ | `for x in a:` | `foreach ($a as $x)` | `for _, x := range a`（`_` は添字を捨てる） |
| 添字付きループ | `for i, x in enumerate(a):` | `foreach ($a as $i => $x)` | `for i, x := range a` |
| 辞書 | `d = {}` | `$d = [];`（連想配列） | `d := map[string]T{}` |
| キーの有無 | `k in d` | `isset($d[$k])` | `v, ok := d[k]` |
| 内包表記・filter | `[x for x in a if x > 0]` | `array_filter(...)` | 無い。`for` で書く（`slices` パッケージに一部ある） |

## テスト

| やりたいこと | Python（pytest） | PHP（PHPUnit） | Go |
|---|---|---|---|
| テストファイル | `test_foo.py` | `FooTest.php` | `foo_test.go`（同じディレクトリに置く） |
| テスト | `def test_foo():` | `public function testFoo()` | `func TestFoo(t *testing.T)` |
| ケース分け | `@pytest.mark.parametrize` | `@dataProvider` | テーブル駆動 + `t.Run` |
| 失敗 | `assert x == y` | `$this->assertSame($y, $x)` | `if x != y { t.Errorf(...) }` |
| 飛ばす | `pytest.skip("理由")` | `$this->markTestSkipped()` | `t.Skip("理由")` |
| 実行 | `pytest` | `phpunit` | `go test ./...` |
