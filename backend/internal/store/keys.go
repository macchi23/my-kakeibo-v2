// このファイルで学ぶこと（L5）:
//   - 文字列でキーを組む関数を 1 か所に閉じ込める設計（PK を組むのは userPK だけ）
//   - 定数（const）と、パッケージ内の非公開関数
package store

import (
	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

// キー設計は docs/DECISIONS.md「DynamoDB のキー設計」を参照。
//
//	口座:     PK = USER#{sub}  SK = ACCOUNT#{id}
//	カテゴリ: PK = USER#{sub}  SK = CATEGORY#{id}
const (
	userPKPrefix     = "USER#"
	accountSKPrefix  = "ACCOUNT#"
	categorySKPrefix = "CATEGORY#"
)

// userPK は PK を組む唯一の関数。ストアの全メソッドはここ経由でしか PK を作らない。
// 他人の PK を組めるコードパスを作らないこと（userId によるデータ分離の要）。
func userPK(u kakeibo.UserID) string {
	// TODO(L5): userPKPrefix + string(u)。
	panic("TODO(L5)")
}

func accountSK(id string) string {
	// TODO(L5)
	panic("TODO(L5)")
}

func categorySK(id string) string {
	// TODO(L5)
	panic("TODO(L5)")
}
