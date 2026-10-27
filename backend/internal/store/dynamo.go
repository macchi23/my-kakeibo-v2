// このファイルで学ぶこと（L5）:
//   - AWS SDK for Go v2 の呼び出し方（Input 構造体、*string を作る aws.String、第 3 引数の optFns）
//   - attributevalue による構造体 <-> DynamoDB の項目の変換（MarshalMap / UnmarshalMap）
//   - 条件付き書き込み（ConditionExpression）と、SDK のエラー型の判別（errors.As）
//   - ページング（LastEvaluatedKey を最後まで追う）
//   - SDK のクライアントを小さなインターフェースの裏に置く（テストで差し替えられる）
package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
)

// L5 で attributevalue を使い始めたら、この行は消す（go mod tidy が依存を落とさないための置き場）。
var _ = attributevalue.MarshalMap

// dynamoAPI は使う操作だけを並べた、このパッケージ専用のインターフェース。*dynamodb.Client がそのまま満たす。
type dynamoAPI interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	Query(ctx context.Context, in *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

// Dynamo は DynamoDB のストア。契約は Memory と同じ（httpapi.AccountStore / CategoryStore）。
//
//   - 作成は ConditionExpression "attribute_not_exists(PK)"。失敗（ConditionalCheckFailedException）は
//     ID の衝突なので、ErrNotFound ではない普通のエラーにする
//   - 更新は "attribute_exists(PK)"。UpdateItem / PutItem は対象が無いと項目を新規作成してしまうため、条件が必須。
//     他人の ID を指定したときも PK が違うので「無い」ことになり、失敗 → kakeibo.ErrNotFound になる
//   - 一覧は Query（Scan は使わない）。KeyConditionExpression は "PK = :pk AND begins_with(SK, :sk)"。
//     ページング（LastEvaluatedKey）を最後まで回す。結果は SK の昇順、つまり ID の昇順
//   - 読み取り（GetItem / Query）は必ず ConsistentRead: aws.Bool(true)。更新は「Get → 全体を PutItem」の形なので、
//     結果整合性で読むと直前の改名を古い値で書き戻す。カテゴリ作成時の重複検査（Query）も、直前に作った同名を見落とす
//   - 取得は GetItem。項目が無い（Item が空）なら kakeibo.ErrNotFound
//   - 保存する時刻は UTC にする（.UTC()）
type Dynamo struct {
	db    dynamoAPI
	table string
}

// NewDynamo はテーブル名を受け取る。テーブル名は cmd/lambda が環境変数 TABLE_NAME から渡す（コードに書かない）。
func NewDynamo(c dynamoAPI, table string) *Dynamo {
	// TODO(L5): Dynamo を作って返す。
	panic("TODO(L5)")
}

// 変換: ドメインの型 <-> 保存用の型。PK / SK / Type / UserID はここで埋める（userPK / accountSK を使う）。
func newAccountItem(user kakeibo.UserID, a kakeibo.Account) accountItem {
	// TODO(L5)
	panic("TODO(L5)")
}

func (it accountItem) toAccount() kakeibo.Account {
	// TODO(L5)
	panic("TODO(L5)")
}

func newCategoryItem(user kakeibo.UserID, c kakeibo.Category) categoryItem {
	// TODO(L5)
	panic("TODO(L5)")
}

func (it categoryItem) toCategory() kakeibo.Category {
	// TODO(L5)
	panic("TODO(L5)")
}

func (d *Dynamo) ListAccounts(ctx context.Context, user kakeibo.UserID) ([]kakeibo.Account, error) {
	// TODO(L5): Query（ConsistentRead: aws.Bool(true) 必須。理由は Dynamo の説明を参照）（begins_with(SK, accountSKPrefix)）→ ページングを回す → attributevalue.UnmarshalListOfMaps → toAccount。
	//   ヒント: dynamodb.NewQueryPaginator が使える。ExpressionAttributeValues は map[string]types.AttributeValue
	//   （types.AttributeValueMemberS{Value: ...}）
	panic("TODO(L5)")
}

func (d *Dynamo) GetAccount(ctx context.Context, user kakeibo.UserID, id string) (kakeibo.Account, error) {
	// TODO(L5): GetItem（ConsistentRead: aws.Bool(true) 必須）の Key に PK と SK を入れる。out.Item が空なら ErrNotFound。UnmarshalMap で accountItem に。
	//   ヒント: Key は map[string]types.AttributeValue。文字列 2 つなので自分で組んでも attributevalue.MarshalMap でもよい
	panic("TODO(L5)")
}

func (d *Dynamo) CreateAccount(ctx context.Context, user kakeibo.UserID, a kakeibo.Account) error {
	// TODO(L5): MarshalMap(newAccountItem(...)) → PutItem + ConditionExpression "attribute_not_exists(PK)"。
	//   ConditionalCheckFailed は errors.As(err, &ccf)（var ccf *types.ConditionalCheckFailedException）で判別して、
	//   fmt.Errorf で「ID が衝突した」エラーにする。
	panic("TODO(L5)")
}

func (d *Dynamo) UpdateAccount(ctx context.Context, user kakeibo.UserID, a kakeibo.Account) error {
	// TODO(L5): 項目全体を置き換える。PutItem + "attribute_exists(PK)" が最も簡単（UpdateItem + SET 式でもよい。条件は必須）。
	//   ConditionalCheckFailed は kakeibo.ErrNotFound に変える。それ以外のエラーは %w で包んで返す。
	panic("TODO(L5)")
}

func (d *Dynamo) ListCategories(ctx context.Context, user kakeibo.UserID) ([]kakeibo.Category, error) {
	// TODO(L5): ListAccounts と同じ要領（SK の prefix が違う）。共通部分をヘルパーに切り出してよい。
	panic("TODO(L5)")
}

func (d *Dynamo) GetCategory(ctx context.Context, user kakeibo.UserID, id string) (kakeibo.Category, error) {
	// TODO(L5)
	panic("TODO(L5)")
}

func (d *Dynamo) CreateCategory(ctx context.Context, user kakeibo.UserID, c kakeibo.Category) error {
	// TODO(L5)
	panic("TODO(L5)")
}

func (d *Dynamo) UpdateCategory(ctx context.Context, user kakeibo.UserID, c kakeibo.Category) error {
	// TODO(L5)
	panic("TODO(L5)")
}
