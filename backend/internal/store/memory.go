// Package store はハンドラーが使う永続化の実装。Memory（ローカル・テスト用）と Dynamo（本番用）の 2 つがあり、
// 同じ契約テスト（contract_test.go）を通す。
//
// このファイルで学ぶこと（L2）:
//   - map（入れ子の map と、キーが無いときのゼロ値）、sync.Mutex と defer
//   - 値のコピーとポインタの共有（ArchivedAt *time.Time を使い回すと、呼び出し側の変更がストアの中身を書き換える）
//   - map の走査順は不定なので、返す順序は自分で決める
//   - メソッドを実装すれば、宣言なしでインターフェースを満たす（暗黙のインターフェース実装）
package store

import (
	"context"
	"sync"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

// Memory はインメモリのストア。ユーザーごとに別の map を持つので、他人のデータには構造上届かない。
// 契約は httpapi.AccountStore / httpapi.CategoryStore を参照（ここでは httpapi を import しない）。
//   - List は ID の昇順（Dynamo の SK の並びと揃える）。アーカイブ済みも含める
//   - Get / Update は、無い・他人のものなら kakeibo.ErrNotFound。Update は項目を新規作成しない
//   - Create は同じユーザーの中で ID が既にあればエラー（別ユーザーの同じ ID は別の項目）
//   - 受け取った値も返す値も、ストア内部と共有しない（ポインタのフィールドはコピーする）
type Memory struct {
	mu         sync.Mutex
	accounts   map[kakeibo.UserID]map[string]kakeibo.Account
	categories map[kakeibo.UserID]map[string]kakeibo.Category
}

// NewMemory は空のストアを返す。
func NewMemory() *Memory {
	// TODO(L2): map を初期化して返す。nil の map に書き込むと panic する。
	//   ヒント: make(map[K]V) / &Memory{...}
	panic("TODO(L2)")
}

func (m *Memory) ListAccounts(ctx context.Context, user kakeibo.UserID) ([]kakeibo.Account, error) {
	// TODO(L2): m.mu.Lock(); defer m.mu.Unlock()。user の map を走査し、ID の昇順で返す。
	//   ヒント: slices.Sorted(maps.Keys(...)) か、スライスに集めて slices.SortFunc。
	//   存在しない user の map を読んでも、nil の map の読み出しは安全（ゼロ値が返る）
	panic("TODO(L2)")
}

func (m *Memory) GetAccount(ctx context.Context, user kakeibo.UserID, id string) (kakeibo.Account, error) {
	// TODO(L2): 見つからなければ kakeibo.ErrNotFound。返す前に ArchivedAt をコピーする（下の cloneTime 的なヘルパーを作ってもよい）。
	//   ヒント: v, ok := m.accounts[user][id]
	panic("TODO(L2)")
}

func (m *Memory) CreateAccount(ctx context.Context, user kakeibo.UserID, a kakeibo.Account) error {
	// TODO(L2): 既にあればエラー（fmt.Errorf）。無ければ保存。user の map が無ければ作る。
	panic("TODO(L2)")
}

func (m *Memory) UpdateAccount(ctx context.Context, user kakeibo.UserID, a kakeibo.Account) error {
	// TODO(L2): 無ければ kakeibo.ErrNotFound（新規作成しない）。あれば置き換える。
	panic("TODO(L2)")
}

func (m *Memory) ListCategories(ctx context.Context, user kakeibo.UserID) ([]kakeibo.Category, error) {
	// TODO(L2): ListAccounts と同じ要領。
	panic("TODO(L2)")
}

func (m *Memory) GetCategory(ctx context.Context, user kakeibo.UserID, id string) (kakeibo.Category, error) {
	// TODO(L2)
	panic("TODO(L2)")
}

func (m *Memory) CreateCategory(ctx context.Context, user kakeibo.UserID, c kakeibo.Category) error {
	// TODO(L2)
	panic("TODO(L2)")
}

func (m *Memory) UpdateCategory(ctx context.Context, user kakeibo.UserID, c kakeibo.Category) error {
	// TODO(L2)
	panic("TODO(L2)")
}
