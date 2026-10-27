package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/httpapi"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

func createAccount(t *testing.T, e *testEnv, user kakeibo.UserID, in kakeibo.AccountInput) kakeibo.Account {
	t.Helper()
	rec := e.do(t, user, http.MethodPost, "/accounts", js(in))
	wantStatus(t, rec, http.StatusCreated)
	return decode[kakeibo.Account](t, rec)
}

func TestAccountLifecycle(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	created := e.now

	// 作成: 名前の前後の空白は除いて保存する
	rec := e.do(t, userA, http.MethodPost, "/accounts", `{"name":"  三井住友  ","kind":"BANK","openingBalance":10000,"sortOrder":1}`)
	wantStatus(t, rec, http.StatusCreated)
	acc := decode[kakeibo.Account](t, rec)
	if _, err := kakeibo.ParseID(acc.ID); err != nil {
		t.Errorf("ID が UUID ではない: %q", acc.ID)
	}
	if acc.Name != "三井住友" || acc.Kind != kakeibo.AccountKindBank || acc.OpeningBalance != 10000 || acc.SortOrder != 1 {
		t.Errorf("作成結果 = %+v", acc)
	}
	if !acc.CreatedAt.Equal(created) || !acc.UpdatedAt.Equal(created) {
		t.Errorf("createdAt/updatedAt = %v / %v, want %v", acc.CreatedAt, acc.UpdatedAt, created)
	}
	if acc.ArchivedAt != nil || strings.Contains(rec.Body.String(), "archivedAt") {
		t.Errorf("作成直後は archivedAt を出さない: %s", rec.Body.String())
	}

	// 取得
	rec = e.do(t, userA, http.MethodGet, "/accounts/"+acc.ID, "")
	wantStatus(t, rec, http.StatusOK)
	if got := decode[kakeibo.Account](t, rec); got.ID != acc.ID || got.Name != acc.Name {
		t.Errorf("取得結果 = %+v, want %+v", got, acc)
	}

	// 更新: ID・createdAt は変わらず、updatedAt は進む
	e.now = e.now.Add(time.Hour)
	rec = e.do(t, userA, http.MethodPut, "/accounts/"+acc.ID, `{"name":"みずほ","kind":"CASH","openingBalance":-5,"sortOrder":3}`)
	wantStatus(t, rec, http.StatusOK)
	upd := decode[kakeibo.Account](t, rec)
	if upd.ID != acc.ID || upd.Name != "みずほ" || upd.Kind != kakeibo.AccountKindCash || upd.OpeningBalance != -5 || upd.SortOrder != 3 {
		t.Errorf("更新結果 = %+v", upd)
	}
	if !upd.CreatedAt.Equal(created) || !upd.UpdatedAt.Equal(e.now) {
		t.Errorf("createdAt/updatedAt = %v / %v, want %v / %v", upd.CreatedAt, upd.UpdatedAt, created, e.now)
	}

	// アーカイブ: 既定の一覧から外れ、includeArchived=true では残る
	archivedAt := e.now.Add(time.Hour)
	e.now = archivedAt
	rec = e.do(t, userA, http.MethodPost, "/accounts/"+acc.ID+"/archive", "")
	wantStatus(t, rec, http.StatusOK)
	if got := decode[kakeibo.Account](t, rec); got.ArchivedAt == nil || !got.ArchivedAt.Equal(archivedAt) {
		t.Errorf("archivedAt = %v, want %v", got.ArchivedAt, archivedAt)
	}
	rec = e.do(t, userA, http.MethodGet, "/accounts", "")
	wantStatus(t, rec, http.StatusOK)
	if items := decode[list[kakeibo.Account]](t, rec).Items; len(items) != 0 {
		t.Errorf("既定の一覧にアーカイブ済みが含まれる: %+v", items)
	}
	rec = e.do(t, userA, http.MethodGet, "/accounts?includeArchived=true", "")
	if items := decode[list[kakeibo.Account]](t, rec).Items; len(items) != 1 || items[0].ID != acc.ID {
		t.Errorf("includeArchived=true の一覧 = %+v", items)
	}

	// 取得はアーカイブ済みでも返る
	rec = e.do(t, userA, http.MethodGet, "/accounts/"+acc.ID, "")
	wantStatus(t, rec, http.StatusOK)

	// 再アーカイブは冪等: archivedAt を上書きしない
	e.now = e.now.Add(time.Hour)
	rec = e.do(t, userA, http.MethodPost, "/accounts/"+acc.ID+"/archive", "")
	wantStatus(t, rec, http.StatusOK)
	if got := decode[kakeibo.Account](t, rec); got.ArchivedAt == nil || !got.ArchivedAt.Equal(archivedAt) {
		t.Errorf("再アーカイブで archivedAt が変わった: %v, want %v", got.ArchivedAt, archivedAt)
	}

	// 解除すると一覧に戻る
	rec = e.do(t, userA, http.MethodPost, "/accounts/"+acc.ID+"/unarchive", "")
	wantStatus(t, rec, http.StatusOK)
	if got := decode[kakeibo.Account](t, rec); got.ArchivedAt != nil {
		t.Errorf("解除後も archivedAt がある: %v", got.ArchivedAt)
	}
	rec = e.do(t, userA, http.MethodGet, "/accounts", "")
	if items := decode[list[kakeibo.Account]](t, rec).Items; len(items) != 1 {
		t.Errorf("解除後の既定の一覧 = %+v", items)
	}
}

func TestAccountList(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)

	// 0 件は null ではなく []
	rec := e.do(t, userA, http.MethodGet, "/accounts", "")
	wantStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("0 件の本文 = %q, want items が []", rec.Body.String())
	}

	// sortOrder の昇順（作成順ではない）
	for i, name := range []string{"c", "b", "a"} {
		createAccount(t, e, userA, kakeibo.AccountInput{Name: name, Kind: kakeibo.AccountKindCash, SortOrder: 2 - i})
	}
	rec = e.do(t, userA, http.MethodGet, "/accounts", "")
	var names []string
	for _, a := range decode[list[kakeibo.Account]](t, rec).Items {
		names = append(names, a.Name)
	}
	if got := strings.Join(names, ","); got != "a,b,c" {
		t.Errorf("並び順 = %s, want a,b,c", got)
	}
}

func TestAccountBadRequests(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{name: "name が空", body: `{"name":"","kind":"BANK"}`, wantField: "name"},
		{name: "name が空白だけ", body: `{"name":"   ","kind":"BANK"}`, wantField: "name"},
		{name: "name が 51 文字", body: `{"name":"` + strings.Repeat("あ", 51) + `","kind":"BANK"}`, wantField: "name"},
		{name: "kind が列挙外", body: `{"name":"x","kind":"WALLET"}`, wantField: "kind"},
		{name: "kind が無い", body: `{"name":"x"}`, wantField: "kind"},
		{name: "本文に userId（未知のフィールド）", body: `{"name":"x","kind":"BANK","userId":"user-b"}`},
		{name: "本文に id（未知のフィールド）", body: `{"id":"018f3b0a-0000-7000-8000-000000000000","name":"x","kind":"BANK"}`},
		{name: "JSON が壊れている", body: `{"name":`},
		{name: "空の本文", body: ``},
		{name: "型が違う", body: `{"name":"x","kind":"BANK","openingBalance":"千円"}`},
		{name: "JSON の後ろに余分なデータ", body: `{"name":"x","kind":"BANK"}{"name":"y"}`},
		{name: "JSON の後ろにゴミ", body: `{"name":"x","kind":"BANK"} x`},
		{name: "JSON の後ろに閉じ括弧が余分", body: `{"name":"x","kind":"BANK"}}`},
	}
	for _, tt := range tests {
		t.Run("POST "+tt.name, func(t *testing.T) {
			e := newEnv(t)
			rec := e.do(t, userA, http.MethodPost, "/accounts", tt.body)
			wantError(t, rec, http.StatusBadRequest, "validation_error", tt.wantField)

			// 作られていないこと
			rec = e.do(t, userA, http.MethodGet, "/accounts?includeArchived=true", "")
			if items := decode[list[kakeibo.Account]](t, rec).Items; len(items) != 0 {
				t.Errorf("400 なのに作成された: %+v", items)
			}
		})
		t.Run("PUT "+tt.name, func(t *testing.T) {
			e := newEnv(t)
			acc := createAccount(t, e, userA, kakeibo.AccountInput{Name: "元", Kind: kakeibo.AccountKindBank})
			rec := e.do(t, userA, http.MethodPut, "/accounts/"+acc.ID, tt.body)
			wantError(t, rec, http.StatusBadRequest, "validation_error", tt.wantField)

			rec = e.do(t, userA, http.MethodGet, "/accounts/"+acc.ID, "")
			if got := decode[kakeibo.Account](t, rec); got.Name != "元" {
				t.Errorf("400 なのに更新された: %+v", got)
			}
		})
	}
}

func TestAccountBodyTooLarge(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	body := `{"name":"` + strings.Repeat("a", 70<<10) + `","kind":"BANK"}`
	rec := e.do(t, userA, http.MethodPost, "/accounts", body)
	wantError(t, rec, http.StatusRequestEntityTooLarge, "payload_too_large", "")
}

func TestAccountInvalidID(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	ids := []string{"abc", "ACCOUNT%23" + unknownID, "018f3b0a-0000-7000-8000-00000000000%23", "%20" + unknownID}
	reqs := []struct{ method, suffix, body string }{
		{http.MethodGet, "", ""},
		{http.MethodPut, "", `{"name":"x","kind":"BANK"}`},
		{http.MethodPost, "/archive", ""},
		{http.MethodPost, "/unarchive", ""},
	}
	e := newEnv(t)
	for _, id := range ids {
		for _, rq := range reqs {
			t.Run(rq.method+" "+id+rq.suffix, func(t *testing.T) {
				rec := e.do(t, userA, rq.method, "/accounts/"+id+rq.suffix, rq.body)
				wantError(t, rec, http.StatusBadRequest, "validation_error", "id")
			})
		}
	}
}

func TestAccountNotFound(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	reqs := []struct{ method, suffix, body string }{
		{http.MethodGet, "", ""},
		{http.MethodPut, "", `{"name":"x","kind":"BANK"}`},
		{http.MethodPost, "/archive", ""},
		{http.MethodPost, "/unarchive", ""},
	}
	e := newEnv(t)
	for _, rq := range reqs {
		t.Run(rq.method+rq.suffix, func(t *testing.T) {
			rec := e.do(t, userA, rq.method, "/accounts/"+unknownID+rq.suffix, rq.body)
			wantError(t, rec, http.StatusNotFound, "not_found", "")
		})
	}

	// PUT は存在しない項目を作らない
	rec := e.do(t, userA, http.MethodGet, "/accounts?includeArchived=true", "")
	if items := decode[list[kakeibo.Account]](t, rec).Items; len(items) != 0 {
		t.Errorf("PUT で項目が作られた: %+v", items)
	}
}

func TestAccountNoDelete(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	acc := createAccount(t, e, userA, kakeibo.AccountInput{Name: "x", Kind: kakeibo.AccountKindBank})
	rec := e.do(t, userA, http.MethodDelete, "/accounts/"+acc.ID, "")
	wantStatus(t, rec, http.StatusMethodNotAllowed)
}

// 2 人のユーザーでデータが分離されていること。
func TestAccountIsolation(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	accA := createAccount(t, e, userA, kakeibo.AccountInput{Name: "Aの口座", Kind: kakeibo.AccountKindBank, OpeningBalance: 100})

	// B から A の項目は見えない
	rec := e.do(t, userB, http.MethodGet, "/accounts/"+accA.ID, "")
	wantError(t, rec, http.StatusNotFound, "not_found", "")

	rec = e.do(t, userB, http.MethodPut, "/accounts/"+accA.ID, `{"name":"乗っ取り","kind":"CASH"}`)
	wantError(t, rec, http.StatusNotFound, "not_found", "")

	for _, action := range []string{"archive", "unarchive"} {
		rec = e.do(t, userB, http.MethodPost, "/accounts/"+accA.ID+"/"+action, "")
		wantError(t, rec, http.StatusNotFound, "not_found", "")
	}

	// B の一覧は空（アーカイブ込みでも）
	for _, q := range []string{"", "?includeArchived=true"} {
		rec = e.do(t, userB, http.MethodGet, "/accounts"+q, "")
		wantStatus(t, rec, http.StatusOK)
		if !strings.Contains(rec.Body.String(), `"items":[]`) {
			t.Errorf("B の一覧%s = %q, want 空", q, rec.Body.String())
		}
	}

	// A の項目は変わっていない
	rec = e.do(t, userA, http.MethodGet, "/accounts/"+accA.ID, "")
	wantStatus(t, rec, http.StatusOK)
	got := decode[kakeibo.Account](t, rec)
	if got.Name != "Aの口座" || got.Kind != kakeibo.AccountKindBank || got.ArchivedAt != nil || !got.UpdatedAt.Equal(accA.UpdatedAt) {
		t.Errorf("A の口座が変わった: %+v", got)
	}

	// B が作った項目は A から見えない
	accB := createAccount(t, e, userB, kakeibo.AccountInput{Name: "Bの口座", Kind: kakeibo.AccountKindCash})
	rec = e.do(t, userA, http.MethodGet, "/accounts/"+accB.ID, "")
	wantStatus(t, rec, http.StatusNotFound)
	rec = e.do(t, userA, http.MethodGet, "/accounts", "")
	if items := decode[list[kakeibo.Account]](t, rec).Items; len(items) != 1 || items[0].ID != accA.ID {
		t.Errorf("A の一覧 = %+v", items)
	}
}

func TestUnauthenticated(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	reqs := []struct{ method, path, body string }{
		{http.MethodGet, "/accounts", ""},
		{http.MethodPost, "/accounts", `{"name":"x","kind":"BANK"}`},
		{http.MethodGet, "/accounts/" + unknownID, ""},
		{http.MethodGet, "/categories", ""},
		{http.MethodPost, "/categories", `{"name":"x","kind":"EXPENSE"}`},
	}
	for _, rq := range reqs {
		t.Run(rq.method+" "+rq.path, func(t *testing.T) {
			rec := e.do(t, "", rq.method, rq.path, rq.body) // user を載せない
			wantError(t, rec, http.StatusUnauthorized, "unauthenticated", "")
		})
	}
}

// failingStore はすべてのメソッドが内部エラーを返す。500 の本文に詳細が漏れないことを確かめるため。
type failingStore struct{ err error }

func (f failingStore) ListAccounts(context.Context, kakeibo.UserID) ([]kakeibo.Account, error) {
	return nil, f.err
}

func (f failingStore) GetAccount(context.Context, kakeibo.UserID, string) (kakeibo.Account, error) {
	return kakeibo.Account{}, f.err
}

func (f failingStore) CreateAccount(context.Context, kakeibo.UserID, kakeibo.Account) error {
	return f.err
}

func (f failingStore) UpdateAccount(context.Context, kakeibo.UserID, kakeibo.Account) error {
	return f.err
}

func TestInternalErrorHidesDetail(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	const secret = "dynamodb: secret-detail"
	var logs bytes.Buffer
	h := httpapi.NewHandler(&httpapi.Server{
		Accounts: failingStore{err: errors.New(secret)},
		Now:      time.Now,
		Log:      slog.New(slog.NewTextHandler(&logs, nil)),
	})

	rec := serve(h, userA, http.MethodGet, "/accounts", "")
	wantError(t, rec, http.StatusInternalServerError, "internal_error", "")
	if strings.Contains(rec.Body.String(), "secret-detail") {
		t.Errorf("500 の本文に内部エラーの詳細が含まれる: %s", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "secret-detail") {
		t.Errorf("詳細がログに出ていない: %s", logs.String())
	}
}

func TestAccountPutKeepsArchivedAt(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	acc := createAccount(t, e, userA, kakeibo.AccountInput{Name: "元", Kind: kakeibo.AccountKindBank})
	archivedAt := e.now.Add(time.Hour)
	e.now = archivedAt
	wantStatus(t, e.do(t, userA, http.MethodPost, "/accounts/"+acc.ID+"/archive", ""), http.StatusOK)

	e.now = e.now.Add(time.Hour)
	rec := e.do(t, userA, http.MethodPut, "/accounts/"+acc.ID, `{"name":"改名","kind":"BANK"}`)
	wantStatus(t, rec, http.StatusOK)
	got := decode[kakeibo.Account](t, rec)
	if got.Name != "改名" || got.ArchivedAt == nil || !got.ArchivedAt.Equal(archivedAt) || !got.CreatedAt.Equal(acc.CreatedAt) {
		t.Errorf("PUT で ArchivedAt / CreatedAt が変わった: %+v", got)
	}

	rec = e.do(t, userA, http.MethodGet, "/accounts", "")
	if items := decode[list[kakeibo.Account]](t, rec).Items; len(items) != 0 {
		t.Errorf("PUT でアーカイブ済みが既定の一覧に戻った: %+v", items)
	}
}
