// このファイルで学ぶこと（L1）:
//   - スライスの走査（range）と条件の組み合わせ
//   - 「保存済みのデータ全体を見ないと判定できない不変条件」を、純粋な関数として切り出す書き方
package kakeibo

import (
	"time"
)

type CategoryKind string

const (
	CategoryKindExpense CategoryKind = "EXPENSE"
	CategoryKindIncome  CategoryKind = "INCOME"
)

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

type CategoryInput struct {
	Name        string       `json:"name"`
	Kind        CategoryKind `json:"kind"`
	Description string       `json:"description"`
	SortOrder   int          `json:"sortOrder"`
}

// Validate は入力単体の検査。
//   - name: 前後の空白を除いて 1〜50 文字（rune 数）
//   - description: 0〜200 文字（rune 数）
//   - kind: EXPENSE か INCOME
func (in CategoryInput) Validate() error {
	// TODO(L1): AccountInput.Validate と同じ要領で書く。
	panic("TODO(L1)")
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
	// TODO(L1): existing を range で回して、上の 3 つを検査する。
	panic("TODO(L1)")
}

// CheckKindUnchanged は更新で kind を変えようとしていないか検査する。変えようとしたら ValidationError（Field "kind"）。
// current は保存済みのカテゴリ、in は更新リクエストの入力。
func CheckKindUnchanged(current Category, in CategoryInput) error {
	// TODO(L1): current.Kind と in.Kind を比べる。
	panic("TODO(L1)")
}
