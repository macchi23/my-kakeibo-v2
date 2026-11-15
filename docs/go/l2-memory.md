# L2 の前に読む: map・ポインタ・sync.Mutex・errors.Is

L2 では `backend/internal/store/memory.go`（インメモリのストア）を書く。口座とカテゴリを、DynamoDB の代わりにメモリ上の map に保存するもの。
ローカルで動かすとき（`go run ./cmd/api`）とテストで使い、本番の DynamoDB 版（L5）と同じ契約テスト（`contract_test.go`）を通す。

L1 の [`basics.md`](basics.md) の続き。Python / PHP との対応は [`python-to-go.md`](python-to-go.md) にも足してある。

## 0. 作るものの全体像

```go
type Memory struct {
	mu         sync.Mutex                                     // 3 節: 同時アクセスから守る鍵
	accounts   map[kakeibo.UserID]map[string]kakeibo.Account  // 1 節: ユーザーごとの map の map
	categories map[kakeibo.UserID]map[string]kakeibo.Category
}
```

Python で書くとこういうもの:

```python
class Memory:
    def __init__(self):
        self.lock = threading.Lock()
        self.accounts = {}      # {user_id: {account_id: Account}}
        self.categories = {}

    def get_account(self, user, id):
        with self.lock:
            acc = self.accounts.get(user, {}).get(id)
            if acc is None:
                raise NotFoundError()
            return copy.deepcopy(acc)
```

ユーザーごとに別の map を持つので、他人の ID を指定しても、そもそも別の map を引くことになり届かない（データ分離を構造で担保する）。

## 1. map の続き（入れ子・nil・make）

### 作り方と、nil の map

```go
var m map[string]int        // 宣言だけ。中身は nil（まだ map が無い）
m2 := map[string]int{}      // 空の map を作る
m3 := make(map[string]int)  // 同じく空の map を作る（make は map・slice を作る組み込み関数）
```

**nil の map は、読むのは安全、書くと panic。**

```go
var m map[string]int
v := m["a"]     // OK。無いキーと同じで、ゼロ値 0 が返る
m["a"] = 1      // panic: assignment to entry in nil map
```

Python で言うと、「`None` に対して `d["a"]` を読むと 0 が返るが、書くと落ちる」という変わった振る舞い。
だから `NewMemory()` では、フィールドの map を `make` で作ってから返す（TODO のヒントのとおり）。

### 入れ子の map

`map[UserID]map[string]Account` は「ユーザー ID → (口座 ID → 口座)」の 2 段の map。

```go
acc, ok := m.accounts[user][id]   // 2 段まとめて引ける
```

- `m.accounts[user]` が無い（そのユーザーがまだ何も作っていない）と、内側の map は nil になる
- nil の map を読むのは安全なので、`[id]` まで書いても panic しない。`ok` が `false` になるだけ
- **書くときは内側の map が無ければ先に作る**（`CreateAccount` の TODO「user の map が無ければ作る」）

```go
if m.accounts[user] == nil {
	m.accounts[user] = map[string]kakeibo.Account{}
}
m.accounts[user][a.ID] = a
```

### 「あるか」を確かめる（comma ok）

```go
v, ok := m["a"]   // ok は「キーがあったか」の bool（Python の "a" in m）
if !ok {
	// 無い
}
```

L1 の `map[...]bool` の例では値そのものを true / false に使ったが、値が struct のときは `ok` で見分ける
（値のゼロ値と「無い」を区別できないため）。

## 2. ポインタと「値のコピー」

### Go の代入はコピー（Python と一番違う所）

```go
a := kakeibo.Account{Name: "現金"}
b := a          // struct の中身がまるごとコピーされる
b.Name = "銀行"
fmt.Println(a.Name) // "現金" のまま
```

Python では `b = a` は同じオブジェクトを指すだけ（`b.name = ...` で `a` も変わる）。Go の struct は**値**なので、代入・引数・戻り値のたびにコピーされる。
map に入れるときも取り出すときもコピー。だから「ストアの中身を呼び出し側が勝手に書き換える」ことは、普通のフィールドでは起きない。

### ポインタ（`*T` と `&`）

コピーせずに「同じものを指したい」ときにポインタを使う。

```go
x := 10
p := &x     // & で「x の場所」を取る。p の型は *int（int へのポインタ）
*p = 20     // * で指している先を読み書き
fmt.Println(x) // 20
```

| 書き方 | 意味 | Python で言うと |
|---|---|---|
| `*T`（型） | T へのポインタ。`nil`（何も指さない）になれる | 「None かもしれない参照」 |
| `&v` | v の場所を取る | （無い。Python は最初から全部参照） |
| `*p` | p が指す先 | （無い） |

L1 で出てきた `ArchivedAt *time.Time` は、「無い（nil）」を表すためにポインタにしている（`time.Time` のゼロ値は「西暦 1 年」で、無いとは区別しにくい）。

### 落とし穴: struct をコピーしても、中のポインタは同じ先を指す

```go
a := kakeibo.Account{ArchivedAt: &t}
b := a                 // b は a のコピー。でも b.ArchivedAt と a.ArchivedAt は同じ t を指す
*b.ArchivedAt = 別の時刻 // a の方も変わってしまう
```

Python の浅いコピー（`copy.copy`）と同じ問題。ストアに保存した口座の `ArchivedAt` を、呼び出し側があとで書き換えるとストアの中身まで変わる。
だから TODO に「返す前に ArchivedAt をコピーする」とある。小さなヘルパーを作るとよい:

```go
// cloneTime は *time.Time の中身をコピーした新しいポインタを返す（nil は nil のまま）。
func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t      // 中身をコピー
	return &c    // コピーの場所を返す
}
```

（関数の中で作った変数の場所を返してよい。Go が自動で長生きさせてくれる。C のような「消えた変数を指す」事故は起きない）

### メソッドのレシーバが `*Memory` なのは

```go
func (m *Memory) ListAccounts(...)   // ポインタレシーバ
```

L1 の `func (e ValidationError) Error()` は値レシーバ（コピーを受け取る）だった。`*Memory` にしているのは:
- ストアの中身（map）を書き換えるので、コピーではなく本体を触りたい
- `sync.Mutex` はコピーしてはいけない（コピーすると別の鍵になり、守れなくなる）

`NewMemory()` が `*Memory`（`&Memory{...}`）を返すのも同じ理由。呼ぶ側は `m.ListAccounts(...)` と普通に書けば、Go が自動でポインタとして渡す。

## 3. sync.Mutex と defer

### なぜ鍵が要るか

Go の HTTP サーバーは、リクエストごとに goroutine（軽いスレッド）を立てて**同時に**処理する。
2 つのリクエストが同時に同じ map に書き込むと、Go の map は壊れる（実行時に `fatal error: concurrent map writes` で落ちる）。
そこで、map を触る間だけ鍵（Mutex）をかけて、1 つずつ順番に触らせる。

```go
func (m *Memory) GetAccount(...) (...) {
	m.mu.Lock()          // 鍵をかける（他の goroutine はここで待つ）
	defer m.mu.Unlock()  // この関数を抜けるときに必ず鍵を外す
	// ここから下は 1 つずつしか実行されない
	...
}
```

Python の `with self.lock:` とほぼ同じ。

### defer

`defer 処理` は「この関数を抜けるときに実行する」予約。`return` がいくつあっても、途中でエラーを返しても、必ず実行される。
Python の `try: ... finally:` や `with` の後始末、PHP の `finally` に当たる。

```go
m.mu.Lock()
defer m.mu.Unlock()
if 見つからない {
	return kakeibo.Account{}, kakeibo.ErrNotFound   // ここで抜けても Unlock される
}
return 見つかった口座, nil                          // ここで抜けても Unlock される
```

`Lock()` の直後に `defer Unlock()` を書く、を 1 セットで覚える。外し忘れると、次のリクエストが永遠に待つ（デッドロック）。

## 4. errors.Is と「決まったエラー」

L1 の `errors.go` に `var ErrNotFound = errors.New("not found")` があった。これは「見つからない」を表す決まったエラーの値（sentinel error）。

```go
// 返す側（ストア）
return kakeibo.Account{}, kakeibo.ErrNotFound

// 受け取る側（HTTP 層）
acc, err := store.GetAccount(ctx, user, id)
if errors.Is(err, kakeibo.ErrNotFound) {
	// 404 を返す
}
```

| やりたいこと | Python | Go |
|---|---|---|
| 「見つからない」を表す | `class NotFoundError(Exception)` | `var ErrNotFound = errors.New("not found")` |
| 投げる・返す | `raise NotFoundError()` | `return ..., ErrNotFound` |
| 種類で分ける | `except NotFoundError:` | `if errors.Is(err, ErrNotFound)` |
| 文脈を付けて包む | `raise X("...") from e` | `fmt.Errorf("口座 %s: %w", id, err)` |

- `==` で比べてもよさそうだが、`fmt.Errorf("...: %w", err)` で包まれていても見つけられるよう、`errors.Is` を使う
- 失敗したときの戻り値の 1 つ目は、型のゼロ値（`kakeibo.Account{}` や `nil` の slice）を返すのが慣習
- `CreateAccount` で「既にある」ときは、決まったエラーではなく `fmt.Errorf("account %s already exists", a.ID)` のような普通のエラーでよい（TODO のとおり）

## 5. map の順番は毎回変わる → 自分で並べる

Go の map を `for k, v := range m` で回す順番は、**わざと毎回ばらばら**にしてある（順番に頼ったバグを早く見つけるため）。
Python の dict は挿入順が保たれるが、Go は保たれない。

`ListAccounts` は「ID の昇順」で返す契約なので、自分で並べる。方法は 2 つ（TODO のヒント）:

```go
// (1) キーを並べてから、順に取り出す
for _, id := range slices.Sorted(maps.Keys(byID)) {
	out = append(out, byID[id])
}

// (2) 値を slice に集めてから、ID で並べる
out := make([]kakeibo.Account, 0, len(byID))
for _, a := range byID {
	out = append(out, a)
}
slices.SortFunc(out, func(a, b kakeibo.Account) int {
	return cmp.Compare(a.ID, b.ID)   // a < b なら負、同じなら 0、a > b なら正
})
```

- (1) は Python の `for id in sorted(d.keys()):`、(2) は `out.sort(key=lambda a: a.id)` に近い
- `slices`・`maps`・`cmp` は標準パッケージ（import に足す）
- (2) の `func(a, b kakeibo.Account) int { ... }` はその場で作る関数（Python の `lambda`。ただし複数行書ける）
- UUIDv7 の ID は作成順に並ぶ文字列なので、ID の昇順 ≒ 作成順になる
- 返す前に、各要素の `ArchivedAt` をコピーするのを忘れない（2 節）

`make([]T, 0, len(byID))` は「長さ 0、容量 len(byID) の slice」を作る。容量を先に確保しておくと `append` のたびに作り直さずに済む（無くても正しく動く）。

## 6. インターフェースは「宣言なし」で満たす

HTTP 層（`internal/httpapi/routes.go`）は、ストアに求めるメソッドをインターフェースで決めている:

```go
type AccountStore interface {
	ListAccounts(ctx context.Context, user kakeibo.UserID) ([]kakeibo.Account, error)
	GetAccount(ctx context.Context, user kakeibo.UserID, id string) (kakeibo.Account, error)
	CreateAccount(ctx context.Context, user kakeibo.UserID, a kakeibo.Account) error
	UpdateAccount(ctx context.Context, user kakeibo.UserID, a kakeibo.Account) error
}
```

`Memory` 側には「AccountStore を実装します」とはどこにも書かない。**同じ名前・同じ引数と戻り値のメソッドがそろっていれば、自動的に満たす。**
PHP の `implements` のような宣言は要らない。Python のダックタイピング（`typing.Protocol`）に近いが、Go はそれをコンパイル時に確かめる。

- メソッドが 1 つでも足りない・型が違うと、`cmd/api/main.go` で `Memory` を `AccountStore` として渡す所がコンパイルエラーになる
- これで、HTTP 層は「Memory か Dynamo か」を知らずに同じコードで動く（L3 で使う）

## 7. ctx（context.Context）はまだ使わない

全メソッドの最初の引数 `ctx context.Context` は、「リクエストが取り消されたら処理をやめる」などに使うもの。L4 で扱う。
Memory はメモリを触るだけなので、受け取って使わなくてよい（Go は使わない**引数**はエラーにしない。使わない**変数**はエラーになる）。

## 8. 進め方とテスト

TODO は `memory.go` の 9 個（`NewMemory` と、口座・カテゴリの List / Get / Create / Update）。おすすめの順:

1. `NewMemory`（map を `make` して `&Memory{...}` を返す）
2. `CreateAccount` → `GetAccount`（入れて取り出せることを先に作る）
3. `UpdateAccount` → `ListAccounts`
4. カテゴリの 4 つ（口座と同じ形。2〜3 が書ければほぼ写すだけ）

テストは `contract_test.go` の `TestContractMemory` の `t.Skip(...)` を消して、`backend/` で:

```sh
go test ./internal/store/ -run TestContractMemory -v
```

契約テストは全メソッドを使うので、途中だと panic（`TODO(L2)`）で止まる。止まった所が次に書く所、と読めばよい。
`-race` を付けると、鍵のかけ忘れ（同時アクセス）を検出してくれる: `go test -race ./internal/store/ -run TestContractMemory`
