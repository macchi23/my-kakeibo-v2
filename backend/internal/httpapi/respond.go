// このファイルで学ぶこと（L3）:
//   - encoding/json の Encoder / Decoder（DisallowUnknownFields）
//   - http.MaxBytesReader でリクエストサイズを制限する理由
//   - errors.Is / errors.As でエラーの種類を判別して HTTP ステータスに写す書き方
package httpapi

import (
	"errors"
	"net/http"
)

// maxBodyBytes はリクエスト本文の上限。個人用の小さな JSON しか来ないので 64KiB で足りる。
const maxBodyBytes = 64 << 10

// エラーレスポンスの code。クライアントはこの文字列で分岐する。
const (
	codeValidation      = "validation_error"
	codeNotFound        = "not_found"
	codeUnauthenticated = "unauthenticated"
	codeTooLarge        = "payload_too_large"
	codeInternal        = "internal_error"
)

// errUnauthenticated は handle が「ユーザーが context に無い」ときに writeError へ渡す。→ 401
var errUnauthenticated = errors.New("unauthenticated")

// errorBody は {"error":{"code":"...","message":"...","field":"..."}} の形。field は無ければ省く。
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// writeJSON は Content-Type: application/json で status と v の JSON を書く。
// WriteHeader を呼んだ後はヘッダーを変えられないので、Header().Set を先に行う。
// エンコードに失敗してもステータスは送信済みなので、ここでできるのはログだけ。
func writeJSON(w http.ResponseWriter, status int, v any) {
	// TODO(L3): Header().Set → WriteHeader(status) → json.NewEncoder(w).Encode(v)。
	//   Encode の error を握りつぶさない（slog.Default() でよい。関数の引数を増やさない）。
	panic("TODO(L3)")
}

// writeError は err を HTTP のレスポンスに変える。
//   - kakeibo.ValidationError → 400（field と message をそのまま返す。利用者向けの文言なので出してよい）
//   - kakeibo.ErrNotFound → 404
//   - errUnauthenticated → 401
//   - *http.MaxBytesError → 413
//   - それ以外 → 500。本文は汎用の文言（"internal error"）にし、詳細は s.Log.ErrorContext に出す。
//     DynamoDB のエラー文などを利用者に返さないため
//
// 判別は errors.Is / errors.As（== や型スイッチではない）。fmt.Errorf("%w", err) で包まれていても判別できる。
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	// TODO(L3): 上の対応表を実装する。
	//   ヒント: var ve kakeibo.ValidationError; errors.As(err, &ve) / var mbe *http.MaxBytesError; errors.As(err, &mbe)
	panic("TODO(L3)")
}

// decodeJSON はリクエスト本文を dst（ポインタ）に読み込む。次のどれも拒否する。
//   - 上限（maxBodyBytes）超過 → *http.MaxBytesError を返す（writeError が 413 にする）
//   - 未知のフィールド（本文に userId を入れるなど）→ ValidationError
//   - JSON として不正、空の本文 → ValidationError
//   - JSON の後ろに余分なデータがある（`{} {}` や `{}x`）→ ValidationError
//
// MaxBytesError と ValidationError を取り違えないこと（Decode が返すエラーの中に MaxBytesError が包まれている）。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// TODO(L3): http.MaxBytesReader(w, r.Body, maxBodyBytes) で包み、json.NewDecoder + DisallowUnknownFields + Decode。
	//   その後、もう一度 Decode して io.EOF になるかで余分なデータを検出する（dec.More() は `}` や `]` で false を返すので使わない。`{}}` を見逃す）。
	panic("TODO(L3)")
}
