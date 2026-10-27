package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

func TestUserIDContext(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	if _, ok := UserIDFrom(context.Background()); ok {
		t.Error("何も載せていないのに ok=true")
	}
	ctx := WithUserID(context.Background(), "user-a")
	if got, ok := UserIDFrom(ctx); !ok || got != "user-a" {
		t.Errorf("got (%q, %v), want (user-a, true)", got, ok)
	}
	if _, ok := UserIDFrom(WithUserID(context.Background(), "")); ok {
		t.Error("空の UserID は ok=false にする")
	}
}

func TestClaimsContext(t *testing.T) {
	t.Skip("TODO(L4): 実装したらこの行を消す")

	if _, ok := claimsFrom(context.Background()); ok {
		t.Error("何も載せていないのに ok=true")
	}
	ctx := WithClaims(context.Background(), map[string]string{"sub": "user-a"})
	got, ok := claimsFrom(ctx)
	if !ok || got["sub"] != "user-a" {
		t.Errorf("got (%v, %v)", got, ok)
	}
}

func TestRequireJWT(t *testing.T) {
	t.Skip("TODO(L4): 実装したらこの行を消す")

	tests := []struct {
		name       string
		claims     map[string]string // nil なら WithClaims を呼ばない（authorizer が付いていない場合）
		wantStatus int
		wantUser   kakeibo.UserID // 200 のとき next に渡る UserID
	}{
		{name: "正常（アクセストークン）", claims: map[string]string{"sub": "user-a", "token_use": "access"}, wantStatus: http.StatusOK, wantUser: "user-a"},
		{name: "claims が無い", claims: nil, wantStatus: http.StatusUnauthorized},
		{name: "空の claims", claims: map[string]string{}, wantStatus: http.StatusUnauthorized},
		{name: "sub が空", claims: map[string]string{"sub": "", "token_use": "access"}, wantStatus: http.StatusUnauthorized},
		{name: "sub が無い", claims: map[string]string{"token_use": "access"}, wantStatus: http.StatusUnauthorized},
		{name: "ID トークン", claims: map[string]string{"sub": "user-a", "token_use": "id"}, wantStatus: http.StatusUnauthorized},
		{name: "token_use が無い", claims: map[string]string{"sub": "user-a"}, wantStatus: http.StatusUnauthorized},
		{name: "token_use が空", claims: map[string]string{"sub": "user-a", "token_use": ""}, wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			var gotUser kakeibo.UserID
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotUser, _ = UserIDFrom(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
			if tt.claims != nil {
				req = req.WithContext(WithClaims(req.Context(), tt.claims))
			}
			rec := httptest.NewRecorder()
			RequireJWT(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				if !called || gotUser != tt.wantUser {
					t.Errorf("next: called=%v user=%q, want user=%q", called, gotUser, tt.wantUser)
				}
				return
			}
			if called {
				t.Error("401 なのに next が呼ばれた")
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("本文が JSON ではない: %v (%q)", err, rec.Body.String())
			}
			if body.Error.Code != "unauthenticated" || body.Error.Message == "" {
				t.Errorf("エラー本文 = %+v", body.Error)
			}
		})
	}
}
