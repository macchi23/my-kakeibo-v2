package store

import (
	"time"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

// DynamoDB に保存する形。ドメインの型（kakeibo.Account）に dynamodbav タグを付けず、保存形式を分ける。
// 全項目に素の type と userId を持たせる（P4 で GSI を後付けするとき、合成キーを埋め直さずに済む）。
// GSI1 / GSI2 の属性は P1 では持たない（キー属性を持たない項目は GSI に載らない）。
//
// time.Time は attributevalue が RFC 3339 の文字列として保存する。UTC にしてから保存すること。
// ArchivedAt が nil のときは omitempty で属性ごと省く。
const (
	itemTypeAccount  = "ACCOUNT"
	itemTypeCategory = "CATEGORY"
)

type accountItem struct {
	PK             string              `dynamodbav:"PK"`
	SK             string              `dynamodbav:"SK"`
	Type           string              `dynamodbav:"type"`
	UserID         string              `dynamodbav:"userId"`
	ID             string              `dynamodbav:"id"`
	Name           string              `dynamodbav:"name"`
	Kind           kakeibo.AccountKind `dynamodbav:"kind"`
	OpeningBalance kakeibo.Yen         `dynamodbav:"openingBalance"`
	SortOrder      int                 `dynamodbav:"sortOrder"`
	ArchivedAt     *time.Time          `dynamodbav:"archivedAt,omitempty"`
	CreatedAt      time.Time           `dynamodbav:"createdAt"`
	UpdatedAt      time.Time           `dynamodbav:"updatedAt"`
}

type categoryItem struct {
	PK          string               `dynamodbav:"PK"`
	SK          string               `dynamodbav:"SK"`
	Type        string               `dynamodbav:"type"`
	UserID      string               `dynamodbav:"userId"`
	ID          string               `dynamodbav:"id"`
	Name        string               `dynamodbav:"name"`
	Kind        kakeibo.CategoryKind `dynamodbav:"kind"`
	Description string               `dynamodbav:"description"`
	SortOrder   int                  `dynamodbav:"sortOrder"`
	ArchivedAt  *time.Time           `dynamodbav:"archivedAt,omitempty"`
	CreatedAt   time.Time            `dynamodbav:"createdAt"`
	UpdatedAt   time.Time            `dynamodbav:"updatedAt"`
}
