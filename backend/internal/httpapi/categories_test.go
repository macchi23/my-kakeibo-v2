package httpapi_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

func createCategory(t *testing.T, e *testEnv, user kakeibo.UserID, in kakeibo.CategoryInput) kakeibo.Category {
	t.Helper()
	rec := e.do(t, user, http.MethodPost, "/categories", js(in))
	wantStatus(t, rec, http.StatusCreated)
	return decode[kakeibo.Category](t, rec)
}

func TestCategoryLifecycle(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	created := e.now

	rec := e.do(t, userA, http.MethodPost, "/categories", `{"name":" 食料品 ","kind":"EXPENSE","description":"スーパーの食材","sortOrder":1}`)
	wantStatus(t, rec, http.StatusCreated)
	cat := decode[kakeibo.Category](t, rec)
	if _, err := kakeibo.ParseID(cat.ID); err != nil {
		t.Errorf("ID が UUID ではない: %q", cat.ID)
	}
	if cat.Name != "食料品" || cat.Kind != kakeibo.CategoryKindExpense || cat.Description != "スーパーの食材" || cat.SortOrder != 1 {
		t.Errorf("作成結果 = %+v", cat)
	}
	if !cat.CreatedAt.Equal(created) || !cat.UpdatedAt.Equal(created) || cat.ArchivedAt != nil {
		t.Errorf("時刻 = %v / %v / %v", cat.CreatedAt, cat.UpdatedAt, cat.ArchivedAt)
	}

	rec = e.do(t, userA, http.MethodGet, "/categories/"+cat.ID, "")
	wantStatus(t, rec, http.StatusOK)

	// 更新（kind は同じ）
	e.now = e.now.Add(time.Hour)
	rec = e.do(t, userA, http.MethodPut, "/categories/"+cat.ID, `{"name":"食費","kind":"EXPENSE","description":"食材と外食","sortOrder":2}`)
	wantStatus(t, rec, http.StatusOK)
	upd := decode[kakeibo.Category](t, rec)
	if upd.Name != "食費" || upd.Description != "食材と外食" || upd.SortOrder != 2 || upd.ID != cat.ID {
		t.Errorf("更新結果 = %+v", upd)
	}
	if !upd.CreatedAt.Equal(created) || !upd.UpdatedAt.Equal(e.now) {
		t.Errorf("createdAt/updatedAt = %v / %v", upd.CreatedAt, upd.UpdatedAt)
	}

	// 名前を変えない更新が、自分自身との重複で弾かれない
	rec = e.do(t, userA, http.MethodPut, "/categories/"+cat.ID, `{"name":"食費","kind":"EXPENSE","description":"説明だけ変更"}`)
	wantStatus(t, rec, http.StatusOK)

	// アーカイブ → 既定の一覧から外れる → 解除
	archivedAt := e.now.Add(time.Hour)
	e.now = archivedAt
	rec = e.do(t, userA, http.MethodPost, "/categories/"+cat.ID+"/archive", "")
	wantStatus(t, rec, http.StatusOK)
	if got := decode[kakeibo.Category](t, rec); got.ArchivedAt == nil || !got.ArchivedAt.Equal(archivedAt) {
		t.Errorf("archivedAt = %v", got.ArchivedAt)
	}
	rec = e.do(t, userA, http.MethodGet, "/categories", "")
	if items := decode[list[kakeibo.Category]](t, rec).Items; len(items) != 0 {
		t.Errorf("既定の一覧にアーカイブ済みが含まれる: %+v", items)
	}
	rec = e.do(t, userA, http.MethodGet, "/categories?includeArchived=true", "")
	if items := decode[list[kakeibo.Category]](t, rec).Items; len(items) != 1 {
		t.Errorf("includeArchived=true の一覧 = %+v", items)
	}

	e.now = e.now.Add(time.Hour)
	rec = e.do(t, userA, http.MethodPost, "/categories/"+cat.ID+"/archive", "")
	if got := decode[kakeibo.Category](t, rec); got.ArchivedAt == nil || !got.ArchivedAt.Equal(archivedAt) {
		t.Errorf("再アーカイブで archivedAt が変わった: %v", got.ArchivedAt)
	}

	rec = e.do(t, userA, http.MethodPost, "/categories/"+cat.ID+"/unarchive", "")
	wantStatus(t, rec, http.StatusOK)
	if got := decode[kakeibo.Category](t, rec); got.ArchivedAt != nil {
		t.Errorf("解除後も archivedAt がある: %v", got.ArchivedAt)
	}
	rec = e.do(t, userA, http.MethodGet, "/categories", "")
	if items := decode[list[kakeibo.Category]](t, rec).Items; len(items) != 1 {
		t.Errorf("解除後の既定の一覧 = %+v", items)
	}
}

func TestCategoryList(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	rec := e.do(t, userA, http.MethodGet, "/categories", "")
	wantStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("0 件の本文 = %q, want items が []", rec.Body.String())
	}

	for i, name := range []string{"c", "b", "a"} {
		createCategory(t, e, userA, kakeibo.CategoryInput{Name: name, Kind: kakeibo.CategoryKindExpense, SortOrder: 2 - i})
	}
	rec = e.do(t, userA, http.MethodGet, "/categories", "")
	var names []string
	for _, c := range decode[list[kakeibo.Category]](t, rec).Items {
		names = append(names, c.Name)
	}
	if got := strings.Join(names, ","); got != "a,b,c" {
		t.Errorf("並び順 = %s, want a,b,c", got)
	}
}

func TestCategoryBadRequests(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{name: "name が空", body: `{"name":"","kind":"EXPENSE"}`, wantField: "name"},
		{name: "kind が列挙外", body: `{"name":"x","kind":"TRANSFER"}`, wantField: "kind"},
		{name: "description が 201 文字", body: `{"name":"x","kind":"EXPENSE","description":"` + strings.Repeat("あ", 201) + `"}`, wantField: "description"},
		{name: "本文に userId", body: `{"name":"x","kind":"EXPENSE","userId":"user-b"}`},
		{name: "JSON が壊れている", body: `{`},
		{name: "JSON の後ろに余分なデータ", body: `{"name":"x","kind":"EXPENSE"} {}`},
		{name: "JSON の後ろに閉じ括弧が余分", body: `{"name":"x","kind":"EXPENSE"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			rec := e.do(t, userA, http.MethodPost, "/categories", tt.body)
			wantError(t, rec, http.StatusBadRequest, "validation_error", tt.wantField)
		})
	}
}

func TestCategoryInvalidIDAndNotFound(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	reqs := []struct{ method, suffix, body string }{
		{http.MethodGet, "", ""},
		{http.MethodPut, "", `{"name":"x","kind":"EXPENSE"}`},
		{http.MethodPost, "/archive", ""},
		{http.MethodPost, "/unarchive", ""},
	}
	for _, rq := range reqs {
		t.Run("不正な ID "+rq.method+rq.suffix, func(t *testing.T) {
			rec := e.do(t, userA, rq.method, "/categories/CATEGORY%23x"+rq.suffix, rq.body)
			wantError(t, rec, http.StatusBadRequest, "validation_error", "id")
		})
		t.Run("存在しない ID "+rq.method+rq.suffix, func(t *testing.T) {
			rec := e.do(t, userA, rq.method, "/categories/"+unknownID+rq.suffix, rq.body)
			wantError(t, rec, http.StatusNotFound, "not_found", "")
		})
	}
}

func TestCategoryKindCannotChange(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	cat := createCategory(t, e, userA, kakeibo.CategoryInput{Name: "給与", Kind: kakeibo.CategoryKindIncome})

	rec := e.do(t, userA, http.MethodPut, "/categories/"+cat.ID, `{"name":"給与","kind":"EXPENSE"}`)
	wantError(t, rec, http.StatusBadRequest, "validation_error", "kind")

	rec = e.do(t, userA, http.MethodGet, "/categories/"+cat.ID, "")
	if got := decode[kakeibo.Category](t, rec); got.Kind != kakeibo.CategoryKindIncome {
		t.Errorf("kind が変わった: %+v", got)
	}
}

func TestCategoryDuplicateName(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	food := createCategory(t, e, userA, kakeibo.CategoryInput{Name: "食料品", Kind: kakeibo.CategoryKindExpense})

	// 同じ kind で同名は 400（前後の空白を除いた名前で比べる）
	rec := e.do(t, userA, http.MethodPost, "/categories", `{"name":" 食料品","kind":"EXPENSE"}`)
	wantError(t, rec, http.StatusBadRequest, "validation_error", "name")

	// 別の kind なら同名でよい
	createCategory(t, e, userA, kakeibo.CategoryInput{Name: "食料品", Kind: kakeibo.CategoryKindIncome})

	// 別のユーザーなら同名でよい
	createCategory(t, e, userB, kakeibo.CategoryInput{Name: "食料品", Kind: kakeibo.CategoryKindExpense})

	// 他のカテゴリと同じ名前への改名は 400
	other := createCategory(t, e, userA, kakeibo.CategoryInput{Name: "外食", Kind: kakeibo.CategoryKindExpense})
	rec = e.do(t, userA, http.MethodPut, "/categories/"+other.ID, `{"name":"食料品","kind":"EXPENSE"}`)
	wantError(t, rec, http.StatusBadRequest, "validation_error", "name")

	// アーカイブすると同名を作れる。解除は、その同名と衝突するので 400
	rec = e.do(t, userA, http.MethodPost, "/categories/"+food.ID+"/archive", "")
	wantStatus(t, rec, http.StatusOK)
	createCategory(t, e, userA, kakeibo.CategoryInput{Name: "食料品", Kind: kakeibo.CategoryKindExpense})
	rec = e.do(t, userA, http.MethodPost, "/categories/"+food.ID+"/unarchive", "")
	wantError(t, rec, http.StatusBadRequest, "validation_error", "name")
}

func TestCategoryExpenseLimit(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	var first kakeibo.Category
	for i := range kakeibo.MaxActiveExpenseCategories {
		c := createCategory(t, e, userA, kakeibo.CategoryInput{Name: fmt.Sprintf("支出%d", i), Kind: kakeibo.CategoryKindExpense})
		if i == 0 {
			first = c
		}
	}

	rec := e.do(t, userA, http.MethodPost, "/categories", `{"name":"あふれる","kind":"EXPENSE"}`)
	wantError(t, rec, http.StatusBadRequest, "validation_error", "")

	// INCOME は別枠
	createCategory(t, e, userA, kakeibo.CategoryInput{Name: "給与", Kind: kakeibo.CategoryKindIncome})

	// 満杯でも既存の更新はできる
	rec = e.do(t, userA, http.MethodPut, "/categories/"+first.ID, `{"name":"改名","kind":"EXPENSE"}`)
	wantStatus(t, rec, http.StatusOK)

	// アーカイブして空きを作れば追加できる。解除は上限超過になるので 400
	rec = e.do(t, userA, http.MethodPost, "/categories/"+first.ID+"/archive", "")
	wantStatus(t, rec, http.StatusOK)
	createCategory(t, e, userA, kakeibo.CategoryInput{Name: "あふれる", Kind: kakeibo.CategoryKindExpense})
	rec = e.do(t, userA, http.MethodPost, "/categories/"+first.ID+"/unarchive", "")
	wantError(t, rec, http.StatusBadRequest, "validation_error", "")

	// 他のユーザーの件数は数えない
	createCategory(t, e, userB, kakeibo.CategoryInput{Name: "支出0", Kind: kakeibo.CategoryKindExpense})
}

func TestCategoryIsolation(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	catA := createCategory(t, e, userA, kakeibo.CategoryInput{Name: "Aの分類", Kind: kakeibo.CategoryKindExpense, Description: "A"})

	rec := e.do(t, userB, http.MethodGet, "/categories/"+catA.ID, "")
	wantError(t, rec, http.StatusNotFound, "not_found", "")
	rec = e.do(t, userB, http.MethodPut, "/categories/"+catA.ID, `{"name":"乗っ取り","kind":"EXPENSE"}`)
	wantError(t, rec, http.StatusNotFound, "not_found", "")
	for _, action := range []string{"archive", "unarchive"} {
		rec = e.do(t, userB, http.MethodPost, "/categories/"+catA.ID+"/"+action, "")
		wantError(t, rec, http.StatusNotFound, "not_found", "")
	}

	for _, q := range []string{"", "?includeArchived=true"} {
		rec = e.do(t, userB, http.MethodGet, "/categories"+q, "")
		wantStatus(t, rec, http.StatusOK)
		if !strings.Contains(rec.Body.String(), `"items":[]`) {
			t.Errorf("B の一覧%s = %q, want 空", q, rec.Body.String())
		}
	}

	rec = e.do(t, userA, http.MethodGet, "/categories/"+catA.ID, "")
	got := decode[kakeibo.Category](t, rec)
	if got.Name != "Aの分類" || got.Description != "A" || got.ArchivedAt != nil || !got.UpdatedAt.Equal(catA.UpdatedAt) {
		t.Errorf("A のカテゴリが変わった: %+v", got)
	}

	// カテゴリの ID で口座は引けない（項目種別が混ざらない）
	rec = e.do(t, userA, http.MethodGet, "/accounts/"+catA.ID, "")
	wantError(t, rec, http.StatusNotFound, "not_found", "")
}

func TestCategoryPutKeepsArchivedAt(t *testing.T) {
	t.Skip("TODO(L3): 実装したらこの行を消す")

	e := newEnv(t)
	cat := createCategory(t, e, userA, kakeibo.CategoryInput{Name: "元", Kind: kakeibo.CategoryKindExpense})
	archivedAt := e.now.Add(time.Hour)
	e.now = archivedAt
	wantStatus(t, e.do(t, userA, http.MethodPost, "/categories/"+cat.ID+"/archive", ""), http.StatusOK)

	e.now = e.now.Add(time.Hour)
	rec := e.do(t, userA, http.MethodPut, "/categories/"+cat.ID, `{"name":"改名","kind":"EXPENSE"}`)
	wantStatus(t, rec, http.StatusOK)
	got := decode[kakeibo.Category](t, rec)
	if got.Name != "改名" || got.ArchivedAt == nil || !got.ArchivedAt.Equal(archivedAt) || !got.CreatedAt.Equal(cat.CreatedAt) {
		t.Errorf("PUT で ArchivedAt / CreatedAt が変わった: %+v", got)
	}

	rec = e.do(t, userA, http.MethodGet, "/categories", "")
	if items := decode[list[kakeibo.Category]](t, rec).Items; len(items) != 0 {
		t.Errorf("PUT でアーカイブ済みが既定の一覧に戻った: %+v", items)
	}
}
