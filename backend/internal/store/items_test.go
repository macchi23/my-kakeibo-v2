package store

import (
	"testing"
	"time"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

// 全項目に PK / SK のほか、素の type と userId を持たせる（docs/DECISIONS.md「DynamoDB のキー設計」）。
func TestNewAccountItem(t *testing.T) {
	t.Skip("TODO(L5): 実装したらこの行を消す")

	at := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		user kakeibo.UserID
		in   kakeibo.Account
	}{
		{name: "有効な口座", user: "user-a", in: kakeibo.Account{ID: "id-1", Name: "n", Kind: kakeibo.AccountKindBank, OpeningBalance: -5, SortOrder: 2}},
		{name: "アーカイブ済み", user: "user-b", in: kakeibo.Account{ID: "id-2", ArchivedAt: &at}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := newAccountItem(tt.user, tt.in)
			if it.PK != "USER#"+string(tt.user) || it.SK != "ACCOUNT#"+tt.in.ID {
				t.Errorf("PK/SK = %q / %q", it.PK, it.SK)
			}
			if it.Type != "ACCOUNT" || it.UserID != string(tt.user) {
				t.Errorf("type/userId = %q / %q", it.Type, it.UserID)
			}
			got := it.toAccount()
			if got.ID != tt.in.ID || got.Name != tt.in.Name || got.OpeningBalance != tt.in.OpeningBalance || (got.ArchivedAt == nil) != (tt.in.ArchivedAt == nil) {
				t.Errorf("往復で値が変わった: %+v", got)
			}
		})
	}
}

func TestNewCategoryItem(t *testing.T) {
	t.Skip("TODO(L5): 実装したらこの行を消す")

	in := kakeibo.Category{ID: "id-1", Name: "n", Kind: kakeibo.CategoryKindIncome, Description: "d", SortOrder: 4}
	it := newCategoryItem("user-a", in)
	if it.PK != "USER#user-a" || it.SK != "CATEGORY#id-1" {
		t.Errorf("PK/SK = %q / %q", it.PK, it.SK)
	}
	if it.Type != "CATEGORY" || it.UserID != "user-a" {
		t.Errorf("type/userId = %q / %q", it.Type, it.UserID)
	}
	got := it.toCategory()
	if got.ID != in.ID || got.Description != in.Description || got.Kind != in.Kind || got.SortOrder != in.SortOrder {
		t.Errorf("往復で値が変わった: %+v", got)
	}
}
