// Package devauth はローカル開発専用の認証。cmd/api だけが import する。
//
// このファイルで学ぶこと（L3）:
//   - ミドルウェアの書き方（auth.RequireJWT と同じ形）、r.Header.Get
//
// 本番の Lambda に入ると、ヘッダーひとつで他人になりすませる。
// そのため cmd/lambda がこのパッケージに依存していないことを scripts/check.sh が検査する。
package devauth

import (
	"net/http"
)

const (
	// Header でユーザーを切り替えられる（2 人分の分離をローカルで試すため）。
	Header = "X-Dev-User"
	// DefaultUser はヘッダーが無いときのユーザー。
	DefaultUser = "local-dev"
)

// Middleware は X-Dev-User ヘッダーの値（無ければ DefaultUser）を UserID にして context に載せる。
func Middleware(next http.Handler) http.Handler {
	// TODO(L3): http.HandlerFunc を返す。r.Header.Get(Header) が空なら DefaultUser。auth.WithUserID で載せて next.ServeHTTP。
	panic("TODO(L3)")
}
