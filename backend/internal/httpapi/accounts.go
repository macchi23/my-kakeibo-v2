// このファイルで学ぶこと（L3）:
//   - ハンドラーの定型（ID 検証 → 本文のデコード → 検証 → ストア → レスポンス）
//   - スライスの絞り込み・並べ替え（slices.DeleteFunc / slices.SortStableFunc）
//   - 値とポインタ（ArchivedAt *time.Time）の扱い
//   - nil スライスは JSON で null になる。一覧は必ず [] にする
//
// 全ハンドラー共通の約束:
//   - user は引数で受け取ったものだけをストアに渡す（本文やクエリの userId は使わない）
//   - ID は r.PathValue("id") を kakeibo.ParseID に通してから使う
//   - 時刻は s.Now().UTC() で取る（保存形式は RFC 3339 の UTC）
//   - 保存する名前は strings.TrimSpace したもの
//   - エラーは握りつぶさず return する（HTTP への変換は writeError）
package httpapi

import (
	"net/http"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

// listAccounts: GET /accounts
// 既定ではアーカイブ済みを除く。?includeArchived=true のときだけ含める。
// SortOrder の昇順に並べる（同じ値は ID 順のまま。安定ソート）。
// 200 {"items":[...]}。0 件のときは {"items":[]}（null にしない）。
func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3): ListAccounts → includeArchived で絞る → 並べ替え → writeJSON。
	//   ヒント: r.URL.Query().Get("includeArchived") == "true" / 結果を make([]kakeibo.Account, 0, n) から作ると null を避けられる
	panic("TODO(L3)")
}

// createAccount: POST /accounts → 201 と作成した Account。
// decodeJSON(AccountInput) → Validate → Account を組み立てる（ID は kakeibo.NewID()、CreatedAt = UpdatedAt = 現在時刻）→ CreateAccount。
func (s *Server) createAccount(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}

// getAccount: GET /accounts/{id} → 200 と Account。ID が不正なら 400、無ければ 404。
func (s *Server) getAccount(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}

// updateAccount: PUT /accounts/{id} → 200 と更新後の Account。
// 全項目を置き換える PUT。GetAccount で現在の値を取り、Name / Kind / OpeningBalance / SortOrder と UpdatedAt だけ上書きして UpdateAccount。
// ID・CreatedAt・ArchivedAt は変えず、Get した値をそのまま引き継ぐ（入力から作り直すと ArchivedAt が消えて、アーカイブ済みが一覧に戻る）。
func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}

// archiveAccount: POST /accounts/{id}/archive → 200 と Account。ArchivedAt に現在時刻を入れる。
// すでにアーカイブ済みなら何も変えずにそのまま 200（冪等。ArchivedAt を上書きしない）。
func (s *Server) archiveAccount(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}

// unarchiveAccount: POST /accounts/{id}/unarchive → 200 と Account。ArchivedAt を nil に戻す。
// アーカイブされていなくても何も変えずに 200。
func (s *Server) unarchiveAccount(w http.ResponseWriter, r *http.Request, user kakeibo.UserID) error {
	// TODO(L3)
	panic("TODO(L3)")
}
