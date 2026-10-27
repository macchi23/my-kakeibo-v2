// Package kakeibo はドメイン（口座・カテゴリなど）を表す。AWS と HTTP には依存しない。
package kakeibo

// UserID は Cognito の sub。string のままだと「ただの文字列」と取り違えるので、専用の型にする。
// ストアの全メソッドはこの型を必ず引数で受け取る（データ分離を呼び出し側の注意に頼らず、型で強制するため）。
type UserID string
