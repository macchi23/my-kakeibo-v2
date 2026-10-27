package kakeibo

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func wantValidationField(t *testing.T, err error, field string) {
	t.Helper()
	if field == "" {
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		return
	}
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("ValidationError を期待したが err=%v", err)
	}
	if ve.Field != field {
		t.Errorf("Field = %q, want %q", ve.Field, field)
	}
}

func TestCategoryInputValidate(t *testing.T) {
	t.Skip("TODO(L1): 実装したらこの行を消す")

	tests := []struct {
		name      string
		in        CategoryInput
		wantField string
	}{
		{name: "正常", in: CategoryInput{Name: "食料品", Kind: CategoryKindExpense, Description: "スーパーで買う食材"}},
		{name: "説明は空でよい", in: CategoryInput{Name: "給与", Kind: CategoryKindIncome}},
		{name: "説明は 200 文字ちょうど", in: CategoryInput{Name: "x", Kind: CategoryKindExpense, Description: strings.Repeat("あ", 200)}},
		{name: "説明は 201 文字", in: CategoryInput{Name: "x", Kind: CategoryKindExpense, Description: strings.Repeat("あ", 201)}, wantField: "description"},
		{name: "名前は 51 文字", in: CategoryInput{Name: strings.Repeat("あ", 51), Kind: CategoryKindExpense}, wantField: "name"},
		{name: "名前が空白だけ", in: CategoryInput{Name: "  ", Kind: CategoryKindExpense}, wantField: "name"},
		{name: "kind が空", in: CategoryInput{Name: "x"}, wantField: "kind"},
		{name: "kind が列挙外", in: CategoryInput{Name: "x", Kind: "TRANSFER"}, wantField: "kind"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantValidationField(t, tt.in.Validate(), tt.wantField)
		})
	}
}

func TestCheckCategoryConstraints(t *testing.T) {
	t.Skip("TODO(L1): 実装したらこの行を消す")

	archived := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cat := func(id, name string, kind CategoryKind, archivedAt *time.Time) Category {
		return Category{ID: id, Name: name, Kind: kind, ArchivedAt: archivedAt}
	}

	// アーカイブされていない EXPENSE を n 件並べる
	expenses := func(n int) []Category {
		out := make([]Category, 0, n)
		for i := range n {
			out = append(out, cat(fmt.Sprintf("e%d", i), fmt.Sprintf("支出%d", i), CategoryKindExpense, nil))
		}
		return out
	}

	tests := []struct {
		name      string
		existing  []Category
		c         Category
		wantField string
	}{
		{
			name:     "既存なしで作成",
			existing: nil,
			c:        cat("new", "食料品", CategoryKindExpense, nil),
		},
		{
			name:      "同じ kind で名前が重複",
			existing:  []Category{cat("a", "食料品", CategoryKindExpense, nil)},
			c:         cat("new", "食料品", CategoryKindExpense, nil),
			wantField: "name",
		},
		{
			name:     "kind が違えば同じ名前でよい",
			existing: []Category{cat("a", "ポイント", CategoryKindIncome, nil)},
			c:        cat("new", "ポイント", CategoryKindExpense, nil),
		},
		{
			name:     "アーカイブ済みの同名は衝突しない",
			existing: []Category{cat("a", "食料品", CategoryKindExpense, &archived)},
			c:        cat("new", "食料品", CategoryKindExpense, nil),
		},
		{
			name:     "自分自身（同じ ID）とは衝突しない（更新で名前を変えない）",
			existing: []Category{cat("a", "食料品", CategoryKindExpense, nil)},
			c:        cat("a", "食料品", CategoryKindExpense, nil),
		},
		{
			name: "別のカテゴリと同じ名前に改名",
			existing: []Category{
				cat("a", "食料品", CategoryKindExpense, nil),
				cat("b", "外食", CategoryKindExpense, nil),
			},
			c:         cat("b", "食料品", CategoryKindExpense, nil),
			wantField: "name",
		},
		{
			name:      "アーカイブ解除で名前が衝突",
			existing:  []Category{cat("a", "食料品", CategoryKindExpense, nil), cat("b", "食料品", CategoryKindExpense, &archived)},
			c:         cat("b", "食料品", CategoryKindExpense, nil),
			wantField: "name",
		},
		{
			name:     "アーカイブ済みの c は制約の対象外",
			existing: []Category{cat("a", "食料品", CategoryKindExpense, nil)},
			c:        cat("b", "食料品", CategoryKindExpense, &archived),
		},
		{
			name:     "EXPENSE が 254 件の状態で 1 件追加できる（255 件になる）",
			existing: expenses(254),
			c:        cat("new", "新しい支出", CategoryKindExpense, nil),
		},
		{
			name:      "EXPENSE が 255 件の状態で追加すると 256 件になるので拒否",
			existing:  expenses(255),
			c:         cat("new", "新しい支出", CategoryKindExpense, nil),
			wantField: "kind",
		},
		{
			name:     "EXPENSE が 255 件でも既存の 1 件の更新は通る（自分を数えない）",
			existing: expenses(255),
			c:        cat("e0", "改名", CategoryKindExpense, nil),
		},
		{
			name:     "EXPENSE が 255 件でも INCOME は追加できる",
			existing: expenses(255),
			c:        cat("new", "給与", CategoryKindIncome, nil),
		},
		{
			name:     "アーカイブ済みの EXPENSE は件数に数えない",
			existing: append(expenses(254), cat("old", "古い", CategoryKindExpense, &archived)),
			c:        cat("new", "新しい支出", CategoryKindExpense, nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantValidationField(t, CheckCategoryConstraints(tt.existing, tt.c), tt.wantField)
		})
	}
}

func TestCheckKindUnchanged(t *testing.T) {
	t.Skip("TODO(L1): 実装したらこの行を消す")

	tests := []struct {
		name      string
		current   Category
		in        CategoryInput
		wantField string
	}{
		{name: "同じ kind", current: Category{Kind: CategoryKindExpense}, in: CategoryInput{Kind: CategoryKindExpense}},
		{name: "EXPENSE から INCOME", current: Category{Kind: CategoryKindExpense}, in: CategoryInput{Kind: CategoryKindIncome}, wantField: "kind"},
		{name: "INCOME から EXPENSE", current: Category{Kind: CategoryKindIncome}, in: CategoryInput{Kind: CategoryKindExpense}, wantField: "kind"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantValidationField(t, CheckKindUnchanged(tt.current, tt.in), tt.wantField)
		})
	}
}
