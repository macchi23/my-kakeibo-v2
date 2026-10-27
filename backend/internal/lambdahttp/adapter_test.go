package lambdahttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/auth"
)

// これらのテスト（L6）は、auth の context.go（WithClaims は L4）と jwt.go（L4）が動いている前提（claims の往復を確かめるため）。

func loadEvent(t *testing.T, name string) events.APIGatewayV2HTTPRequest {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var ev events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(b, &ev); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return ev
}

func invoke(t *testing.T, h http.Handler, ev events.APIGatewayV2HTTPRequest) events.APIGatewayV2HTTPResponse {
	t.Helper()
	resp, err := Wrap(h)(context.Background(), ev)
	if err != nil {
		t.Fatalf("Wrap の関数が error を返した（Lambda が失敗扱いになる）: %v", err)
	}
	return resp
}

// header は大文字小文字を区別せずに値を探す。
func header(resp events.APIGatewayV2HTTPResponse, key string) string {
	for k, v := range resp.Headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

type echoed struct {
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	EscapedPath string              `json:"escapedPath"`
	ID          string              `json:"id"`
	Query       map[string][]string `json:"query"`
	XTest       string              `json:"xTest"`
	Cookies     map[string]string   `json:"cookies"`
	Body        string              `json:"body"`
	User        string              `json:"user"`
}

// echoHandler はリクエストの内容を JSON で返す。ステータス 201、ヘッダー、Set-Cookie 2 つも付ける。
func echoHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /echo/{id}", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			panic(err) // テスト用
		}
		e := echoed{
			Method:      r.Method,
			Path:        r.URL.Path,
			EscapedPath: r.URL.EscapedPath(),
			ID:          r.PathValue("id"),
			Query:       r.URL.Query(),
			XTest:       r.Header.Get("X-Test"),
			Cookies:     map[string]string{},
			Body:        string(body),
		}
		for _, c := range r.Cookies() {
			e.Cookies[c.Name] = c.Value
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Reply", "ok")
		w.Header().Add("X-Multi", "1")
		w.Header().Add("X-Multi", "2")
		http.SetCookie(w, &http.Cookie{Name: "s1", Value: "v1"})
		http.SetCookie(w, &http.Cookie{Name: "s2", Value: "v2"})
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(e); err != nil {
			panic(err) // テスト用。ここで失敗するなら responseWriter が壊れている
		}
	})
	return mux
}

func TestWrapRoundTrip(t *testing.T) {
	t.Skip("TODO(L6): 実装したらこの行を消す")

	tests := []struct {
		name     string
		event    string
		wantPath string
		wantEsc  string
		wantID   string
		wantBody string
		wantCook map[string]string
	}{
		{
			name:     "base64 本文と Cookie",
			event:    "post_echo_base64.json",
			wantPath: "/echo/018f3b0a-0000-7000-8000-000000000000",
			wantEsc:  "/echo/018f3b0a-0000-7000-8000-000000000000",
			wantID:   "018f3b0a-0000-7000-8000-000000000000",
			wantBody: `{"name":"口座"}`,
			wantCook: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:     "エスケープされたパスは生のまま渡る",
			event:    "post_echo_plain.json",
			wantPath: "/echo/x#y",
			wantEsc:  "/echo/x%23y",
			wantID:   "x#y",
			wantBody: `{"name":"口座"}`,
			wantCook: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := invoke(t, echoHandler(), loadEvent(t, tt.event))

			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("status = %d, want 201\nbody=%s", resp.StatusCode, resp.Body)
			}
			if got := header(resp, "X-Reply"); got != "ok" {
				t.Errorf("X-Reply = %q", got)
			}
			if got := header(resp, "Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q", got)
			}
			if got := header(resp, "X-Multi"); got != "1, 2" {
				t.Errorf("同じキーの複数値は ', ' でつなぐ: X-Multi = %q", got)
			}
			if header(resp, "Set-Cookie") != "" {
				t.Errorf("Set-Cookie は Headers ではなく Cookies に入れる: %v", resp.Headers)
			}
			if len(resp.Cookies) != 2 || !strings.HasPrefix(resp.Cookies[0], "s1=v1") || !strings.HasPrefix(resp.Cookies[1], "s2=v2") {
				t.Errorf("Cookies = %v", resp.Cookies)
			}
			if resp.IsBase64Encoded {
				t.Error("IsBase64Encoded は false のまま")
			}

			var got echoed
			if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
				t.Fatalf("本文が JSON ではない: %v\n%s", err, resp.Body)
			}
			if got.Method != http.MethodPost || got.Path != tt.wantPath || got.EscapedPath != tt.wantEsc || got.ID != tt.wantID {
				t.Errorf("method/path/escaped/id = %q %q %q %q", got.Method, got.Path, got.EscapedPath, got.ID)
			}
			if got.Body != tt.wantBody {
				t.Errorf("body = %q, want %q", got.Body, tt.wantBody)
			}
			if len(got.Cookies) != len(tt.wantCook) {
				t.Errorf("cookies = %v, want %v", got.Cookies, tt.wantCook)
			}
			for k, v := range tt.wantCook {
				if got.Cookies[k] != v {
					t.Errorf("cookie %s = %q, want %q", k, got.Cookies[k], v)
				}
			}
		})
	}
}

func TestWrapHeaderAndQuery(t *testing.T) {
	t.Skip("TODO(L6): 実装したらこの行を消す")

	var gotHeader, gotAccept string
	var gotQuery map[string][]string
	var gotRawQuery string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Test")
		gotAccept = r.Header.Get("Accept")
		gotQuery = r.URL.Query()
		gotRawQuery = r.URL.RawQuery
	})
	resp := invoke(t, h, loadEvent(t, "get_accounts_access_token.json"))

	if resp.StatusCode != http.StatusOK {
		t.Errorf("何も書かないハンドラーのステータス = %d, want 200", resp.StatusCode)
	}
	if gotHeader != "a,b" || gotAccept != "application/json" {
		t.Errorf("ヘッダー X-Test=%q Accept=%q", gotHeader, gotAccept)
	}
	if got := gotQuery["includeArchived"]; len(got) != 1 || got[0] != "true" {
		t.Errorf("includeArchived = %v", got)
	}
	if got := gotQuery["q"]; len(got) != 1 || got[0] != "食費" {
		t.Errorf("q = %v（%%E9... のデコードが必要）", got)
	}
	if !strings.Contains(gotRawQuery, "includeArchived=true") {
		t.Errorf("RawQuery = %q", gotRawQuery)
	}
}

func TestWrapClaims(t *testing.T) {
	t.Skip("TODO(L6): 実装したらこの行を消す")

	tests := []struct {
		name       string
		event      string
		wantStatus int
		wantUser   string
	}{
		{name: "アクセストークンの claims", event: "get_accounts_access_token.json", wantStatus: http.StatusOK, wantUser: "user-a"},
		{name: "authorizer が無い", event: "get_accounts_no_authorizer.json", wantStatus: http.StatusUnauthorized},
		{name: "ID トークン", event: "get_accounts_id_token.json", wantStatus: http.StatusUnauthorized},
		{name: "authorizer はあるが JWT が無い", event: "get_accounts_empty_authorizer.json", wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser string
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				u, _ := auth.UserIDFrom(r.Context())
				gotUser = string(u)
				w.WriteHeader(http.StatusOK)
			})
			resp := invoke(t, auth.RequireJWT(next), loadEvent(t, tt.event))

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d\nbody=%s", resp.StatusCode, tt.wantStatus, resp.Body)
			}
			if tt.wantStatus == http.StatusOK {
				if !called || gotUser != tt.wantUser {
					t.Errorf("called=%v user=%q, want %q", called, gotUser, tt.wantUser)
				}
				return
			}
			if called {
				t.Error("401 なのに next が呼ばれた")
			}
			if !strings.Contains(resp.Body, `"unauthenticated"`) {
				t.Errorf("本文 = %q", resp.Body)
			}
		})
	}
}

func TestWrapBadEvent(t *testing.T) {
	t.Skip("TODO(L6): 実装したらこの行を消す")

	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	ev := loadEvent(t, "post_echo_base64.json")
	ev.Body = "!!!not base64!!!"
	resp := invoke(t, h, ev) // invoke が error を返さないことも確かめる
	if resp.StatusCode != http.StatusBadRequest || called {
		t.Errorf("不正な base64: status=%d called=%v, want 400 / false", resp.StatusCode, called)
	}
}

func TestResponseWriter(t *testing.T) {
	t.Skip("TODO(L6): 実装したらこの行を消す")

	t.Run("Write だけなら 200", func(t *testing.T) {
		w := newResponseWriter()
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Fatal(err)
		}
		resp := w.response()
		if resp.StatusCode != http.StatusOK || resp.Body != "ok" {
			t.Errorf("resp = %+v", resp)
		}
	})
	t.Run("何もしなければ 200 の空", func(t *testing.T) {
		resp := newResponseWriter().response()
		if resp.StatusCode != http.StatusOK || resp.Body != "" {
			t.Errorf("resp = %+v", resp)
		}
	})
	t.Run("WriteHeader は最初の 1 回だけ有効", func(t *testing.T) {
		w := newResponseWriter()
		w.WriteHeader(http.StatusNotFound)
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		if got := w.response().StatusCode; got != http.StatusNotFound {
			t.Errorf("status = %d, want 404", got)
		}
	})
	t.Run("Write の後の WriteHeader は無視", func(t *testing.T) {
		w := newResponseWriter()
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusTeapot)
		if got := w.response().StatusCode; got != http.StatusOK {
			t.Errorf("status = %d, want 200", got)
		}
	})
}

// コンパイル時に http.ResponseWriter を満たすことを確かめる
var _ http.ResponseWriter = (*responseWriter)(nil)
