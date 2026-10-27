// このファイルで学ぶこと（L1）:
//   - error インターフェース（Error() string を持てば何でも error）
//   - 自作のエラー型と、errors.Is / errors.As で呼び出し側が種類を判別する書き方
package kakeibo

import (
	"errors"
)

// ErrNotFound は「対象が無い」を表す。他人の項目も同じ扱い（存在を教えないため、403 ではなく 404 にする）。
// 呼び出し側は errors.Is(err, kakeibo.ErrNotFound) で判別する。
var ErrNotFound = errors.New("not found")

// ValidationError は入力の誤り。HTTP 層が 400 にする。
// Field は誤りのあるフィールド名（JSON のキーに合わせる。例 "name"）。特定できなければ空。
type ValidationError struct {
	Field   string
	Message string
}

// Error は error インターフェースを満たすためのメソッド。
// 値レシーバにしているので、ValidationError は「値」で返し（&ValidationError{} にしない）、
// 判別する側は `var ve kakeibo.ValidationError; errors.As(err, &ve)` と書く。
func (e ValidationError) Error() string {
	// TODO(L1): Field があれば "name: 1〜50 文字で入力してください" のように、Field と Message をつないだ文字列を返す。
	//   ヒント: fmt.Sprintf / Field が空かどうかで分岐
	panic("TODO(L1)")
}
