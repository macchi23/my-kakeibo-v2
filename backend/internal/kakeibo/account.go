// このファイルで学ぶこと（L1）:
//   - struct と JSON タグ、string ベースの独自型（列挙の代わり）
//   - utf8.RuneCountInString（文字数 ≠ バイト数）、strings.TrimSpace
//   - 入力用の型（AccountInput）と保存用の型（Account）を分ける理由
package kakeibo

import (
	"strings"
	"time"
	"unicode/utf8"
)

// AccountKind は口座の種別。Go には enum が無いので、string ベースの独自型と定数の組で代用する。
type AccountKind string

// 口座種別の定数。値はそのまま JSON と DynamoDB に保存される。
const (
	AccountKindBank   AccountKind = "BANK"
	AccountKindCash   AccountKind = "CASH"
	AccountKindCard   AccountKind = "CARD"
	AccountKindEMoney AccountKind = "E_MONEY"
	AccountKindOther  AccountKind = "OTHER"
)

// 口座名の長さの範囲（rune 数）。
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

// IsValid は k が定義済みの口座種別（BANK・CASH・CARD・E_MONEY・OTHER）のどれかなら true を返す。
func (k AccountKind) IsValid() bool {
	switch k {
	case AccountKindBank, AccountKindCash, AccountKindCard, AccountKindEMoney, AccountKindOther:
		// 有効な種別
		return true
	}
	// 上記以外は全て無効な種別
	return false
}

// Validate は入力を検証する。誤りは ValidationError で返す（Field は JSON のキー名）。
//   - name: 前後の空白を除いて 1〜50 文字（バイト数ではなく rune 数）
//   - kind: 上の定数のどれか
//
// 空白を除いた名前を保存するのは呼び出し側（ハンドラー）の仕事。ここは検査だけ。
func (in AccountInput) Validate() error {
	// 名前: 前後の空白を除いてから rune 数（文字数）で長さを数える。len() だとバイト数になり、日本語が 3 倍に数えられる。
	n := utf8.RuneCountInString(strings.TrimSpace(in.Name))
	if n < AccountNameMin || n > AccountNameMax {
		return ValidationError{Field: "name", Message: "口座名は1-50文字にしてください"}
	}
	// 種別: 定義済みの値のどれかであること。
	if !in.Kind.IsValid() {
		return ValidationError{Field: "kind", Message: "口座種別の形式が正しくありません"}
	}
	return nil
}
