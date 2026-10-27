// Package httpapi は HTTP のハンドラー。ストアの実装（インメモリ / DynamoDB）は知らず、下のインターフェースだけに依存する。
//
// このファイルで学ぶこと（L3）:
//   - インターフェースは「使う側」で定義する Go の慣習（実装側のパッケージを import しない）
//   - 関数型 (userHandler) に error を返させて、エラー処理を 1 か所にまとめる書き方
//   - net/http の ServeMux（メソッド + パスのパターン、r.PathValue）
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

// AccountStore は口座の永続化。全メソッドが user を受け取る（PK を組む唯一の入力であり、他人のデータに届かなくするため）。
//   - ListAccounts は ID の昇順で、アーカイブ済みを含めて全件返す（絞り込みと並べ替えはハンドラーの仕事）
//   - GetAccount / UpdateAccount は、対象が無い・他人のものなら kakeibo.ErrNotFound
//   - CreateAccount は ID が既に存在するとエラー（ErrNotFound ではない普通のエラー。ID の衝突はバグなので 500 になる）
type AccountStore interface {
	ListAccounts(ctx context.Context, user kakeibo.UserID) ([]kakeibo.Account, error)
	GetAccount(ctx context.Context, user kakeibo.UserID, id string) (kakeibo.Account, error)
	CreateAccount(ctx context.Context, user kakeibo.UserID, a kakeibo.Account) error
	UpdateAccount(ctx context.Context, user kakeibo.UserID, a kakeibo.Account) error
}

// CategoryStore はカテゴリの永続化。契約は AccountStore と同じ。
// NOTE: メソッド名に Account / Category が付くのは、1 つの型（store.Memory / store.Dynamo）が両方を満たすため。
// Go はメソッドのオーバーロードができず、List が 2 つあると型を共用できない。
type CategoryStore interface {
	ListCategories(ctx context.Context, user kakeibo.UserID) ([]kakeibo.Category, error)
	GetCategory(ctx context.Context, user kakeibo.UserID, id string) (kakeibo.Category, error)
	CreateCategory(ctx context.Context, user kakeibo.UserID, c kakeibo.Category) error
	UpdateCategory(ctx context.Context, user kakeibo.UserID, c kakeibo.Category) error
}

// Server はハンドラーの依存をまとめる。Now を差し替えられるようにして、テストで時刻を固定する。
type Server struct {
	Accounts   AccountStore
	Categories CategoryStore
	Now        func() time.Time
	Log        *slog.Logger
}

// NewHandler はルートを登録した http.Handler を返す。
// パターンにメソッドを書く（Go 1.22+）ので、許可していないメソッドは自動で 405 になる。
// 認証は含まない。userId は context に載っている前提で、載せるのは外側のミドルウェア（auth.RequireJWT / devauth.Middleware）。
func NewHandler(s *Server) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /accounts", s.handle(s.listAccounts))
	mux.Handle("POST /accounts", s.handle(s.createAccount))
	mux.Handle("GET /accounts/{id}", s.handle(s.getAccount))
	mux.Handle("PUT /accounts/{id}", s.handle(s.updateAccount))
	mux.Handle("POST /accounts/{id}/archive", s.handle(s.archiveAccount))
	mux.Handle("POST /accounts/{id}/unarchive", s.handle(s.unarchiveAccount))

	mux.Handle("GET /categories", s.handle(s.listCategories))
	mux.Handle("POST /categories", s.handle(s.createCategory))
	mux.Handle("GET /categories/{id}", s.handle(s.getCategory))
	mux.Handle("PUT /categories/{id}", s.handle(s.updateCategory))
	mux.Handle("POST /categories/{id}/archive", s.handle(s.archiveCategory))
	mux.Handle("POST /categories/{id}/unarchive", s.handle(s.unarchiveCategory))

	return mux
}

// userHandler は「認証済みのユーザーが分かっている」ハンドラー。
// user を引数で受け取る形にして、ハンドラー本体が context を触らずに済むようにする。
// エラーは return するだけにして、HTTP のレスポンスへの変換は handle に任せる。
type userHandler func(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error

// handle は userHandler を http.Handler にする。
func (s *Server) handle(h userHandler) http.Handler {
	// TODO(L3): http.HandlerFunc を返す。
	//   1. auth.UserIDFrom(r.Context()) でユーザーを取る。無ければ 401（writeError に渡せるエラー値を用意する）
	//   2. h(w, r, user) を呼ぶ。err が nil でなければ s.writeError(w, r, err)
	//   ヒント: クロージャ（関数が外側の変数 h, s を覚えている）
	panic("TODO(L3)")
}
