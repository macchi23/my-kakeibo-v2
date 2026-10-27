// このファイルで学ぶこと（L1）:
//   - 標準パッケージ uuid の使い方（複数の戻り値、エラー処理）
//   - 外部入力（URL のパス）を検証してから使う習慣
package kakeibo

// NewID はサーバーが採番する ID（UUIDv7 の文字列）を返す。時刻順に並ぶので、DynamoDB の SK の並びが作成順になる。
func NewID() string {
	// TODO(L1): uuid.NewV7() の結果を文字列にして返す。
	//   ヒント: uuid.UUID の String メソッド
	panic("TODO(L1)")
}

// ParseID は URL のパスなどから来た ID を検証し、正規形（小文字・ハイフン区切り）にして返す。
// 不正なら ValidationError（Field は "id"）を返す。
// DynamoDB のキーは "ACCOUNT#{id}" と組むので、区切り文字の '#' などが入った値をここで必ず弾く。
// uuid.Parse は urn:uuid: や波括弧付きも受け付けるため、入力をそのまま返さず、Parse した結果から文字列を作り直す。
func ParseID(s string) (string, error) {
	// TODO(L1): uuid.Parse で検証する。失敗したら ValidationError{Field: "id", ...} を返す。成功したら u.String() を返す。
	panic("TODO(L1)")
}
