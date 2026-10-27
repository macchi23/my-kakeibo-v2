// httpapi のテスト共通部品。
// これらのテスト（L3）は、先に kakeibo（L1）、store.Memory（L2）、auth/context.go の WithUserID / UserIDFrom と devauth（いずれも L3）が動いている前提。
package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/auth"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/httpapi"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/store"
)

const (
	userA kakeibo.UserID = "user-a"
	userB kakeibo.UserID = "user-b"

	// 形式だけ正しい、存在しない ID
	unknownID = "018f3b0a-0000-7000-8000-000000000000"
)

// testEnv は Memory ストアと、進められる時計を持つ。
type testEnv struct {
	h   http.Handler
	now time.Time
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	e := &testEnv{now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	mem := store.NewMemory()
	e.h = httpapi.NewHandler(&httpapi.Server{
		Accounts:   mem,
		Categories: mem,
		Now:        func() time.Time { return e.now },
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return e
}

// do は user として request を送る。user が空なら context に UserID を載せない（認証されていない状態）。
func (e *testEnv) do(t *testing.T, user kakeibo.UserID, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(e.h, user, method, path, body)
}

func serve(h http.Handler, user kakeibo.UserID, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if user != "" {
		req = req.WithContext(auth.WithUserID(req.Context(), user))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("本文が期待した JSON ではない: %v\nstatus=%d body=%q", err, rec.Code, rec.Body.String())
	}
	return v
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field"`
	} `json:"error"`
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d\nbody=%q", rec.Code, status, rec.Body.String())
	}
}

// wantError は status とエラー本文の code を確かめる。field が空でなければそれも確かめる。
func wantError(t *testing.T, rec *httptest.ResponseRecorder, status int, code, field string) {
	t.Helper()
	wantStatus(t, rec, status)
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	got := decode[apiError](t, rec)
	if got.Error.Code != code {
		t.Errorf("code = %q, want %q", got.Error.Code, code)
	}
	if got.Error.Message == "" {
		t.Error("message が空")
	}
	if field != "" && got.Error.Field != field {
		t.Errorf("field = %q, want %q", got.Error.Field, field)
	}
}

type list[T any] struct {
	Items []T `json:"items"`
}

func js(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
