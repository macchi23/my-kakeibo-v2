// このファイルで学ぶこと（L3）:
//   - accounts.go と同じ形のハンドラーを、不変条件（kakeibo.CheckCategoryConstraints）付きで書く
//   - 「全件を読んでからドメインの関数で検査する」流れ
//
// 共通の約束は accounts.go の冒頭を参照。
package httpapi

import (
	"net/http"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

// listCategories: GET /categories。仕様は listAccounts と同じ（?includeArchived=true、SortOrder 昇順、{"items":[]}）。
func (s *Server) listCategories(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}

// createCategory: POST /categories → 201。
// Validate の後、ListCategories で既存を取り、組み立てた Category を kakeibo.CheckCategoryConstraints に通してから CreateCategory。
// 名前の重複・EXPENSE の 255 件超過は 400。
func (s *Server) createCategory(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}

// getCategory: GET /categories/{id}。
func (s *Server) getCategory(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}

// updateCategory: PUT /categories/{id}。
// ID・CreatedAt・ArchivedAt は変えず、Get した値を引き継ぐ（入力から作り直すと ArchivedAt が消えて、アーカイブ済みが一覧に戻る）。
// kind を変えようとしたら 400（kakeibo.CheckKindUnchanged）。名前の重複も 400（自分自身は除かれる）。
func (s *Server) updateCategory(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}

// archiveCategory: POST /categories/{id}/archive。アーカイブ済みなら何も変えずに 200。
func (s *Server) archiveCategory(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}

// unarchiveCategory: POST /categories/{id}/unarchive。
// 戻すと有効な同名カテゴリと衝突したり EXPENSE の上限を超えたりしうるので、ArchivedAt を nil にした姿を
// kakeibo.CheckCategoryConstraints に通してから保存する（違反は 400）。
func (s *Server) unarchiveCategory(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}
