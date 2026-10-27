// このファイルで学ぶこと（L3: WithUserID・UserIDFrom ／ L4: WithClaims・claimsFrom）:
//   - context.Context に値を載せる書き方と、キーを非公開の型にする理由（他パッケージのキーと衝突しない）
//   - 「値が無い」を (値, bool) で返す慣用句（comma ok）
package auth

import (
	"context"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

// ctxKey は context のキー専用の型。非公開にして、このパッケージの関数経由でしか値を出し入れできないようにする。
// string を直接キーにすると、別のパッケージが同じ文字列を使ったときに衝突する。
type ctxKey int

const (
	userIDKey ctxKey = iota
	claimsKey
)

// WithUserID は認証済みのユーザー ID を context に載せる。
// userId の出どころは JWT の claims（RequireJWT）か開発用ミドルウェア（devauth）だけ。リクエスト本文やクエリからは取らない。
func WithUserID(ctx context.Context, u kakeibo.UserID) context.Context {
	// TODO(L3): context.WithValue で userIDKey に u を載せて返す。
	panic("TODO(L3)")
}

// UserIDFrom は context からユーザー ID を取り出す。無い、または空文字なら ok=false。
func UserIDFrom(ctx context.Context) (kakeibo.UserID, bool) {
	// TODO(L3): ctx.Value(userIDKey) を kakeibo.UserID に型アサーション（v, ok := x.(T)）する。空文字は ok=false にする。
	panic("TODO(L3)")
}

// WithClaims は API Gateway の JWT authorizer が検証済みの claims を context に載せる。
// lambdahttp のアダプタが、イベントの RequestContext.Authorizer.JWT.Claims をここに入れる。
// authorizer が付いていないルートでは claims が入らず、RequireJWT が 401 にする。
func WithClaims(ctx context.Context, claims map[string]string) context.Context {
	// TODO(L4): WithUserID と同じ要領で claimsKey に載せる。
	panic("TODO(L4)")
}

// claimsFrom は WithClaims で載せた claims を返す。載っていなければ ok=false。
func claimsFrom(ctx context.Context) (map[string]string, bool) {
	// TODO(L4): UserIDFrom と同じ要領。
	panic("TODO(L4)")
}
