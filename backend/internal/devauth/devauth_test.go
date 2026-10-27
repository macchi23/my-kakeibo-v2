package devauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/auth"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

func TestMiddleware(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	tests := []struct {
		name   string
		header string
		want   kakeibo.UserID
	}{
		{name: "ヘッダーなしは既定のユーザー", header: "", want: DefaultUser},
		{name: "ヘッダーのユーザー", header: "user-b", want: "user-b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got kakeibo.UserID
			var ok bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, ok = auth.UserIDFrom(r.Context())
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set(Header, tt.header)
			}
			Middleware(next).ServeHTTP(httptest.NewRecorder(), req)
			if !ok || got != tt.want {
				t.Errorf("got (%q, %v), want %q", got, ok, tt.want)
			}
		})
	}
}
