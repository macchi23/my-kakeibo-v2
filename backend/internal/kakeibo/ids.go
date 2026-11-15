// このファイルで学ぶこと（L1）:
//   - 標準パッケージ uuid の使い方（複数の戻り値、エラー処理）
//   - 外部入力（URL のパス）を検証してから使う習慣
package kakeibo

import "uuid"

// NewID はサーバーが採番する ID（UUIDv7 の文字列）を返す。時刻順に並ぶので、DynamoDB の SK の並びが作成順になる。
func NewID() string {
	return uuid.NewV7().String()
}

// ParseID は URL のパスなどから来た ID を検証し、正規形（小文字・ハイフン区切り）にして返す。
// 不正なら ValidationError（Field は "id"）を返す。
// DynamoDB のキーは "ACCOUNT#{id}" と組むので、区切り文字の '#' などが入った値をここで必ず弾く。
// uuid.Parse は urn:uuid: や波括弧付きも受け付けるため、入力をそのまま返さず、Parse した結果から文字列を作り直す。
func ParseID(s string) (string, error) {
	// UUID として読めるか検証する。読めなければ ValidationError（err は捨てず、Go の慣習どおり nil か否かで判定）。
	u, err := uuid.Parse(s)
	if err != nil {
		return "", ValidationError{Field: "id", Message: "ID の形式が正しくありません"}
	}
	// 入力ではなく Parse 後の値から正規形の文字列を作り直して返す。
	return u.String(), nil
}
