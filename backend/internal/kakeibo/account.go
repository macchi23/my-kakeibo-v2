// このファイルで学ぶこと（L1）:
//   - struct と JSON タグ、string ベースの独自型（列挙の代わり）
//   - utf8.RuneCountInString（文字数 ≠ バイト数）、strings.TrimSpace
//   - 入力用の型（AccountInput）と保存用の型（Account）を分ける理由
package kakeibo

import (
	"time"
)

type AccountKind string

const (
	AccountKindBank   AccountKind = "BANK"
	AccountKindCash   AccountKind = "CASH"
	AccountKindCard   AccountKind = "CARD"
	AccountKindEMoney AccountKind = "E_MONEY"
	AccountKindOther  AccountKind = "OTHER"
)

const (
	AccountNameMin = 1
	AccountNameMax = 50
)

// Account は口座。物理削除はせず、ArchivedAt を入れることでアーカイブする（nil なら有効）。
type Account struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Kind           AccountKind `json:"kind"`
	OpeningBalance Yen         `json:"openingBalance"`
	SortOrder      int         `json:"sortOrder"`
	ArchivedAt     *time.Time  `json:"archivedAt,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

// AccountInput はクライアントから受け取る入力。userId や id を持たせないのが意図。
// JSON は未知のフィールドを拒否する（httpapi の decodeJSON）ので、本文に userId を入れると 400 になる。
type AccountInput struct {
	Name           string      `json:"name"`
	Kind           AccountKind `json:"kind"`
	OpeningBalance Yen         `json:"openingBalance"`
	SortOrder      int         `json:"sortOrder"`
}

// Validate は入力を検証する。誤りは ValidationError で返す（Field は JSON のキー名）。
//   - name: 前後の空白を除いて 1〜50 文字（バイト数ではなく rune 数）
//   - kind: 上の定数のどれか
//
// 空白を除いた名前を保存するのは呼び出し側（ハンドラー）の仕事。ここは検査だけ。
func (in AccountInput) Validate() error {
	// TODO(L1): name と kind を検査する。
	//   ヒント: strings.TrimSpace / utf8.RuneCountInString / switch でも map でもよい（kind の判定）
	panic("TODO(L1)")
}
