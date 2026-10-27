// このファイルで学ぶこと（L4）:
//   - ミドルウェア（http.Handler を受け取って http.Handler を返す関数）と http.HandlerFunc
//   - 認証に失敗したら「次へ進まず return」する書き方、構造化ログ（slog）
package auth

import (
	"net/http"
)

// RequireJWT は API Gateway の JWT authorizer を通過した claims を検証し、userId を context に載せる。
// authorizer が付いていないルートができた場合の保険として、Go 側でも必ず確かめる（多層防御）。
//
// 401 にする条件（レスポンス本文は {"error":{"code":"unauthenticated","message":"..."}}、Content-Type は application/json）:
//   - claims が context に無い
//   - sub が空
//   - token_use が "access" ではない（ID トークンを拒否する。欠落も拒否）
//
// 401 のときは slog.ErrorContext で理由を記録する（本文には理由を詳しく書かない）。
// OK なら claims["sub"] を kakeibo.UserID にして WithUserID を通し、next.ServeHTTP を呼ぶ。
//
// NOTE: httpapi のエラー形式と同じ形を自分で書く。httpapi が auth に依存しているため、auth から httpapi は import できない。
func RequireJWT(next http.Handler) http.Handler {
	// TODO(L4): http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ... }) を返す。
	//   ヒント: claimsFrom / claims["sub"] / r.WithContext(...) / json.NewEncoder(w).Encode
	panic("TODO(L4)")
}
