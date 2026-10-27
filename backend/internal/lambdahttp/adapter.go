// Package lambdahttp は API Gateway HTTP API（ペイロード v2）のイベントと net/http をつなぐ自前のアダプタ。
// 依存するのは aws-lambda-go の events 型と、auth（claims を context に載せるため）だけ。依存の向きは lambdahttp -> auth。
//
// このファイルで学ぶこと（L6）:
//   - http.ResponseWriter インターフェースを自分の型で実装する（Header / Write / WriteHeader）
//   - httptest ではなく、実際のリクエストを組み立てる http.NewRequestWithContext
//   - 関数を返す関数（Wrap）、base64 のデコード（encoding/base64）、url.Parse
package lambdahttp

import (
	"bytes"
	"context"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

// Wrap は h を Lambda のハンドラー関数にする。lambda.Start に渡せる形。
//
// イベントからリクエストへの対応:
//   - メソッド: RequestContext.HTTP.Method
//   - URL: RawPath と RawQueryString（デコード済みの Path ではなく生のものを使う。%2F などを壊さないため）。
//     url.Parse(RawPath + "?" + RawQueryString) の形で r.URL を作る。RawQueryString が空なら "?" は付けない
//   - ヘッダー: Headers（キーは小文字で来る。http.Header.Set が正規化する）
//   - Cookie: Cookies（["a=b", "c=d"]）を "; " でつないで Cookie ヘッダーにする
//   - 本文: Body。IsBase64Encoded が true なら base64 デコードしてから渡す
//   - context: 引数の ctx。RequestContext.Authorizer.JWT.Claims があれば auth.WithClaims で載せる
//     （authorizer が無いルートでは Authorizer も JWT もポインタで nil になりうるので、nil チェックを忘れない。
//     載せないと RequireJWT が 401 にする）
//
// レスポンスへの対応: ステータス、ヘッダー（同じキーの複数値は ", " でつなぐ。Set-Cookie だけは Cookies に分ける）、
// 本文（文字列として Body に入れる。バイナリを返す API は無いので base64 にはしない）。
// WriteHeader を呼ばずに Write だけされたときのステータスは 200（net/http と同じ）。
//
// リクエストを組み立てられない（不正な URL、不正な base64）場合は、error を返さず 400 のレスポンスにする。
// error を返すと Lambda が失敗扱いにして、API Gateway が 502 を返してしまうため。
func Wrap(h http.Handler) func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// TODO(L6): 関数リテラルを返す。newRequest でリクエストを作り、responseWriter を用意して h.ServeHTTP、結果を response に変換。
	panic("TODO(L6)")
}

// newRequest はイベントから *http.Request を作る。
func newRequest(ctx context.Context, ev events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	// TODO(L6): 上のコメントの対応表のとおりに組み立てる。
	//   ヒント: http.NewRequestWithContext(ctx, method, target, body) / base64.StdEncoding.DecodeString / strings.NewReader / bytes.NewReader
	panic("TODO(L6)")
}

// responseWriter は http.ResponseWriter を満たす、メモリ上に書き込むだけの実装。
// ハンドラーが書いた内容を溜めておき、最後に events の形に変換する。
type responseWriter struct {
	header      http.Header
	status      int
	body        bytes.Buffer
	wroteHeader bool
}

func newResponseWriter() *responseWriter {
	// TODO(L6): header を make(http.Header) で初期化して返す（nil の map に Set すると panic する）。
	panic("TODO(L6)")
}

// Header はレスポンスヘッダーを返す。ハンドラーはここに Set してから WriteHeader / Write を呼ぶ。
func (w *responseWriter) Header() http.Header {
	// TODO(L6)
	panic("TODO(L6)")
}

// Write は本文を溜める。WriteHeader がまだなら 200 で WriteHeader を呼んだことにする。
func (w *responseWriter) Write(b []byte) (int, error) {
	// TODO(L6)
	panic("TODO(L6)")
}

// WriteHeader はステータスを記録する。2 回目以降は無視する（net/http と同じ）。
func (w *responseWriter) WriteHeader(status int) {
	// TODO(L6)
	panic("TODO(L6)")
}

// response は溜めた内容を API Gateway のレスポンスに変換する。
// WriteHeader も Write もされていなければステータスは 200。
func (w *responseWriter) response() events.APIGatewayV2HTTPResponse {
	// TODO(L6): Set-Cookie は Header から取り出して Cookies に入れ、残りのヘッダーを Headers にする。
	panic("TODO(L6)")
}
