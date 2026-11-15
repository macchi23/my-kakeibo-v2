// このファイルで学ぶこと（L1）:
//   - スライスの走査（range）と条件の組み合わせ
//   - 「保存済みのデータ全体を見ないと判定できない不変条件」を、純粋な関数として切り出す書き方
package kakeibo

import (
	"strings"
	"time"
	"unicode/utf8"
)

// CategoryKind はカテゴリの種類（支出・収入）。AccountKind と同じく、string ベースの独自型で列挙を代用する。
type CategoryKind string

const (
	CategoryKindExpense CategoryKind = "EXPENSE"
	CategoryKindIncome  CategoryKind = "INCOME"
)

// カテゴリ名・説明の長さの範囲（rune 数）と、jev に渡せる選択肢の上限。
const (
	CategoryNameMin        = 1
	CategoryNameMax        = 50
	CategoryDescriptionMax = 200
	// MaxActiveExpenseCategories は jev の choice の上限。アーカイブされていない EXPENSE の数に対する上限。
	MaxActiveExpenseCategories = 255
)

// Category はカテゴリ。Kind は作成後に変えられない（過去の明細が逆の種別を指して矛盾するため）。
// Description は jev に選択肢の説明として渡すので、分類精度に効く。
type Category struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Kind        CategoryKind `json:"kind"`
	Description string       `json:"description"`
	SortOrder   int          `json:"sortOrder"`
	ArchivedAt  *time.Time   `json:"archivedAt,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// CategoryInput はクライアントから受け取る入力。AccountInput と同じく ID やユーザー ID は持たせない。
type CategoryInput struct {
	Name        string       `json:"name"`
	Kind        CategoryKind `json:"kind"`
	Description string       `json:"description"`
	SortOrder   int          `json:"sortOrder"`
}

// IsValidはCategoryKindが有効かどうか確認する
func (k CategoryKind) IsValid() bool {
	switch k {
	// 有効なものたち
	case CategoryKindExpense, CategoryKindIncome:
		return true
	}
	// それ以外は無効
	return false
}

// Validate は入力単体の検査。
//   - name: 前後の空白を除いて 1〜50 文字（rune 数）
//   - description: 0〜200 文字（rune 数）
//   - kind: EXPENSE か INCOME
func (in CategoryInput) Validate() error {
	// 名前と説明を rune 数で数える（前後の空白は除く）。
	nameRune := utf8.RuneCountInString(strings.TrimSpace(in.Name))
	descriptionRune := utf8.RuneCountInString(strings.TrimSpace(in.Description))
	// 名前・説明・種類の順に、範囲外なら最初の違反を ValidationError で返す。
	if nameRune < CategoryNameMin || nameRune > CategoryNameMax {
		return ValidationError{Field: "name", Message: "カテゴリ名は1-50文字にしてください"}
	}
	if descriptionRune > CategoryDescriptionMax {
		return ValidationError{Field: "description", Message: "説明は200文字以内にしてください"}
	}
	if !in.Kind.IsValid() {
		return ValidationError{Field: "kind", Message: "カテゴリの種類（支出・収入）が不正です"}
	}
	return nil
}

// CheckCategoryConstraints は他のカテゴリとの関係で決まる不変条件を検査する。
// existing は同じユーザーの全カテゴリ（アーカイブ済みを含んでよい）、c は作成・更新・アーカイブ解除しようとしている後の姿。
//   - c がアーカイブ済み（ArchivedAt != nil）なら、制約の対象外なので nil を返す
//   - アーカイブされていない同じ kind の中で、name の重複を禁止する（大文字小文字も区別する完全一致）。
//     existing の中の c 自身（同じ ID）は比較から除く。更新で名前を変えないときに自分と衝突しないため
//   - c が EXPENSE なら、アーカイブされていない EXPENSE（c 自身を除く）が MaxActiveExpenseCategories 件に達している場合は拒否
//
// 違反は ValidationError（重複は Field "name"、件数超過は Field "kind"）で返す。
// NOTE: 検査と書き込みの間は原子的ではない（同時に 2 件作ると重複しうる）。個人用なので P1 では許容する。
func CheckCategoryConstraints(existing []Category, c Category) error {
	// c がアーカイブ済み（ArchivedAt != nil）なら、制約の対象外なので nil を返す
	if c.ArchivedAt != nil {
		return nil
	}
	// 支出のカテゴリのカウンタを定義
	count := 0
	// 既存カテゴリを回す
	for _, e := range existing {
		// eが自分自身またはアーカイブ済みなら飛ばす
		if e.ID == c.ID || e.ArchivedAt != nil {
			continue
		}
		// eの種類と名前が一緒なら名前重複のエラーを返す
		if e.Name == c.Name && e.Kind == c.Kind {
			return ValidationError{Field: "name", Message: "カテゴリ名が重複しています"}
		}
		// eの種類が支出ならカウンタを進める
		if e.Kind == CategoryKindExpense {
			count++
		}
	}
	// cが支出かつ支出のカテゴリが上限超えている場合は上限超過のエラーを出す
	if c.Kind == CategoryKindExpense && count >= MaxActiveExpenseCategories {
		return ValidationError{Field: "kind", Message: "支出カテゴリ数の上限を超えています"}
	}
	return nil
}

// CheckKindUnchanged は更新で kind を変えようとしていないか検査する。変えようとしたら ValidationError（Field "kind"）。
// current は保存済みのカテゴリ、in は更新リクエストの入力。
func CheckKindUnchanged(current Category, in CategoryInput) error {
	// 保存済みの種類と入力の種類が違えば拒否する。
	if current.Kind != in.Kind {
		return ValidationError{Field: "kind", Message: "カテゴリの種類（支出・収入）は変更できません"}
	}
	return nil
}
