// このファイルで学ぶこと（L1）:
//   - error インターフェース（Error() string を持てば何でも error）
//   - 自作のエラー型と、errors.Is / errors.As で呼び出し側が種類を判別する書き方
package kakeibo

import (
	"errors"
	"fmt"
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
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}
