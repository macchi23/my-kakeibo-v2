package store_test

// 契約テスト: 同じテスト群を Memory（L2）と Dynamo（L5）に流して、2 つの実装が同じふるまいをすることを確かめる。
// Dynamo は DynamoDB Local が要る。環境変数 DYNAMODB_ENDPOINT（例 http://localhost:8000）が無ければ Skip する。
//
//	docker run --rm -p 8000:8000 amazon/dynamodb-local:3.3.2
//	DYNAMODB_ENDPOINT=http://localhost:8000 AWS_ACCESS_KEY_ID=dummy AWS_SECRET_ACCESS_KEY=dummy AWS_REGION=us-east-1 go test ./internal/store/
//
// 実 AWS を叩かないよう、エンドポイントが localhost / 127.0.0.1 でなければ失敗させる。
// このテストは型としてのジェネリクスを使っている（ops[T]）。口座とカテゴリで同じ検査を 2 回書かないため。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/httpapi"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/kakeibo"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/store"
)

const (
	userA kakeibo.UserID = "user-a"
	userB kakeibo.UserID = "user-b"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// contractStore は httpapi が要求する 2 つのインターフェースを両方満たす。
type contractStore interface {
	httpapi.AccountStore
	httpapi.CategoryStore
}

var (
	_ contractStore = (*store.Memory)(nil)
	_ contractStore = (*store.Dynamo)(nil)
)

func TestContractMemory(t *testing.T) {
	t.Skip("TODO(L2): 実装したらこの行を消す")

	runContract(t, func(t *testing.T) contractStore { return store.NewMemory() })
}

func TestContractDynamo(t *testing.T) {
	t.Skip("TODO(L5): 実装したらこの行を消す")

	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		t.Skip("DYNAMODB_ENDPOINT が無いので DynamoDB Local のテストは Skip")
	}
	if !strings.HasPrefix(endpoint, "http://localhost") && !strings.HasPrefix(endpoint, "http://127.0.0.1") {
		t.Fatalf("DYNAMODB_ENDPOINT は localhost / 127.0.0.1 だけ許す（実 AWS を叩かないため）: %q", endpoint)
	}
	runContract(t, func(t *testing.T) contractStore { return newDynamoStore(t, endpoint) })
}

var tableSeq atomic.Int64

// newDynamoStore はテスト専用のテーブルを DynamoDB Local に作り、終わったら消す。
// DynamoDB Local は作成直後から ACTIVE なので、待機（waiter）は入れない。
func newDynamoStore(t *testing.T, endpoint string) contractStore {
	t.Helper()
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatalf("LoadDefaultConfig: %v", err)
	}
	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	table := fmt.Sprintf("contract-%d-%d", time.Now().UnixNano(), tableSeq.Add(1))
	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(table),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("SK"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("PK"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("SK"), KeyType: types.KeyTypeRange},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("CreateTable（DynamoDB Local は起動している? AWS_REGION とダミーの認証情報は設定した?）: %v", err)
	}
	t.Cleanup(func() {
		if _, err := client.DeleteTable(ctx, &dynamodb.DeleteTableInput{TableName: aws.String(table)}); err != nil {
			t.Errorf("DeleteTable: %v", err)
		}
	})
	return store.NewDynamo(client, table)
}

// ops は口座とカテゴリの違いを吸収する。
type ops[T any] struct {
	list        func(ctx context.Context, u kakeibo.UserID) ([]T, error)
	get         func(ctx context.Context, u kakeibo.UserID, id string) (T, error)
	create      func(ctx context.Context, u kakeibo.UserID, v T) error
	update      func(ctx context.Context, u kakeibo.UserID, v T) error
	mk          func(id, name string) T
	idOf        func(v T) string
	archivedAt  func(v T) *time.Time
	setArchived func(v T, at *time.Time) T
}

func accountOps(s contractStore) ops[kakeibo.Account] {
	return ops[kakeibo.Account]{
		list:   s.ListAccounts,
		get:    s.GetAccount,
		create: s.CreateAccount,
		update: s.UpdateAccount,
		mk: func(id, name string) kakeibo.Account {
			return kakeibo.Account{ID: id, Name: name, Kind: kakeibo.AccountKindBank, OpeningBalance: -1200, SortOrder: 3, CreatedAt: t0, UpdatedAt: t0}
		},
		idOf:       func(v kakeibo.Account) string { return v.ID },
		archivedAt: func(v kakeibo.Account) *time.Time { return v.ArchivedAt },
		setArchived: func(v kakeibo.Account, at *time.Time) kakeibo.Account {
			v.ArchivedAt = at
			return v
		},
	}
}

func categoryOps(s contractStore) ops[kakeibo.Category] {
	return ops[kakeibo.Category]{
		list:   s.ListCategories,
		get:    s.GetCategory,
		create: s.CreateCategory,
		update: s.UpdateCategory,
		mk: func(id, name string) kakeibo.Category {
			return kakeibo.Category{ID: id, Name: name, Kind: kakeibo.CategoryKindExpense, Description: "説明", SortOrder: 3, CreatedAt: t0, UpdatedAt: t0}
		},
		idOf:       func(v kakeibo.Category) string { return v.ID },
		archivedAt: func(v kakeibo.Category) *time.Time { return v.ArchivedAt },
		setArchived: func(v kakeibo.Category, at *time.Time) kakeibo.Category {
			v.ArchivedAt = at
			return v
		},
	}
}

func runContract(t *testing.T, newStore func(t *testing.T) contractStore) {
	t.Run("口座", func(t *testing.T) {
		itemContract(t, func(t *testing.T) ops[kakeibo.Account] { return accountOps(newStore(t)) })
	})
	t.Run("カテゴリ", func(t *testing.T) {
		itemContract(t, func(t *testing.T) ops[kakeibo.Category] { return categoryOps(newStore(t)) })
	})
	t.Run("口座とカテゴリは同じ ID でも混ざらない", func(t *testing.T) {
		ctx := context.Background()
		s := newStore(t)
		if err := s.CreateAccount(ctx, userA, accountOps(s).mk("same", "口座")); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateCategory(ctx, userA, categoryOps(s).mk("same", "カテゴリ")); err != nil {
			t.Fatal(err)
		}
		accs, err := s.ListAccounts(ctx, userA)
		if err != nil || len(accs) != 1 || accs[0].Name != "口座" {
			t.Errorf("ListAccounts = %+v, %v", accs, err)
		}
		cats, err := s.ListCategories(ctx, userA)
		if err != nil || len(cats) != 1 || cats[0].Name != "カテゴリ" {
			t.Errorf("ListCategories = %+v, %v", cats, err)
		}
	})
}

// jsonEq は JSON にして比べる。時刻の内部表現（Location など）の違いを無視でき、フィールドの欠落も見つけられる。
func jsonEq[T any](t *testing.T, got, want T) {
	t.Helper()
	g, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	w, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(g) != string(w) {
		t.Errorf("\n got: %s\nwant: %s", g, w)
	}
}

func wantNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, kakeibo.ErrNotFound) {
		t.Errorf("err = %v, want kakeibo.ErrNotFound", err)
	}
}

func itemContract[T any](t *testing.T, newOps func(t *testing.T) ops[T]) {
	ctx := context.Background()

	t.Run("作成して取得できる", func(t *testing.T) {
		o := newOps(t)
		v := o.mk("id-1", "名前")
		if err := o.create(ctx, userA, v); err != nil {
			t.Fatal(err)
		}
		got, err := o.get(ctx, userA, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		jsonEq(t, got, v)
	})

	t.Run("アーカイブ済みの項目を保存して取得できる", func(t *testing.T) {
		o := newOps(t)
		at := t0.Add(time.Hour)
		v := o.setArchived(o.mk("id-1", "名前"), &at)
		if err := o.create(ctx, userA, v); err != nil {
			t.Fatal(err)
		}
		got, err := o.get(ctx, userA, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		jsonEq(t, got, v)
	})

	t.Run("存在しない ID は ErrNotFound", func(t *testing.T) {
		o := newOps(t)
		_, err := o.get(ctx, userA, "missing")
		wantNotFound(t, err)
	})

	t.Run("更新できる（アーカイブの付け外しを含む）", func(t *testing.T) {
		o := newOps(t)
		if err := o.create(ctx, userA, o.mk("id-1", "旧")); err != nil {
			t.Fatal(err)
		}
		at := t0.Add(time.Hour)
		want := o.setArchived(o.mk("id-1", "新"), &at)
		if err := o.update(ctx, userA, want); err != nil {
			t.Fatal(err)
		}
		got, err := o.get(ctx, userA, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		jsonEq(t, got, want)

		// アーカイブ解除（属性が消える）
		want = o.setArchived(want, nil)
		if err := o.update(ctx, userA, want); err != nil {
			t.Fatal(err)
		}
		got, err = o.get(ctx, userA, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		jsonEq(t, got, want)
	})

	t.Run("存在しない項目の更新は ErrNotFound で、項目を作らない", func(t *testing.T) {
		o := newOps(t)
		wantNotFound(t, o.update(ctx, userA, o.mk("id-1", "x")))
		_, err := o.get(ctx, userA, "id-1")
		wantNotFound(t, err)
		list, err := o.list(ctx, userA)
		if err != nil || len(list) != 0 {
			t.Errorf("list = %+v, %v; want 空", list, err)
		}
	})

	t.Run("同じ ID の作成はエラーで、元の項目は変わらない", func(t *testing.T) {
		o := newOps(t)
		orig := o.mk("id-1", "元")
		if err := o.create(ctx, userA, orig); err != nil {
			t.Fatal(err)
		}
		err := o.create(ctx, userA, o.mk("id-1", "上書き"))
		if err == nil {
			t.Fatal("重複した作成が成功した")
		}
		if errors.Is(err, kakeibo.ErrNotFound) {
			t.Errorf("重複は ErrNotFound ではない普通のエラー: %v", err)
		}
		got, err := o.get(ctx, userA, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		jsonEq(t, got, orig)
	})

	t.Run("一覧は ID の昇順で、アーカイブ済みも含む", func(t *testing.T) {
		o := newOps(t)
		at := t0.Add(time.Hour)
		for _, v := range []T{
			o.mk("id-3", "c"),
			o.setArchived(o.mk("id-1", "a"), &at),
			o.mk("id-2", "b"),
		} {
			if err := o.create(ctx, userA, v); err != nil {
				t.Fatal(err)
			}
		}
		list, err := o.list(ctx, userA)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, v := range list {
			ids = append(ids, o.idOf(v))
		}
		if got := strings.Join(ids, ","); got != "id-1,id-2,id-3" {
			t.Errorf("ids = %s, want id-1,id-2,id-3", got)
		}
	})

	t.Run("1MB を超える一覧も最後まで返す（ページング）", func(t *testing.T) {
		// Query は 1 回で 1MB までしか返さない。名前を大きくして約 2MB にし、ページをまたいでも欠けないことを確かめる。
		// （ストアは名前の長さを検証しない。検証はドメイン層の仕事）
		o := newOps(t)
		const n = 100
		for i := range n {
			v := o.mk(fmt.Sprintf("id-%04d", i), strings.Repeat("a", 20<<10))
			if err := o.create(ctx, userA, v); err != nil {
				t.Fatal(err)
			}
		}
		list, err := o.list(ctx, userA)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != n {
			t.Errorf("len = %d, want %d", len(list), n)
		}
	})

	t.Run("返した値を書き換えても、保存内容は変わらない", func(t *testing.T) {
		o := newOps(t)
		at := t0.Add(time.Hour)
		orig := o.setArchived(o.mk("id-1", "名前"), &at)
		if err := o.create(ctx, userA, orig); err != nil {
			t.Fatal(err)
		}

		// 渡した値のポインタを書き換える
		at = at.Add(24 * time.Hour)

		got, err := o.get(ctx, userA, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		// 取得した値のポインタを書き換える
		*o.archivedAt(got) = o.archivedAt(got).Add(48 * time.Hour)

		list, err := o.list(ctx, userA)
		if err != nil || len(list) != 1 {
			t.Fatalf("list = %+v, %v", list, err)
		}
		*o.archivedAt(list[0]) = o.archivedAt(list[0]).Add(72 * time.Hour)

		again, err := o.get(ctx, userA, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		want := o.setArchived(o.mk("id-1", "名前"), ptr(t0.Add(time.Hour)))
		jsonEq(t, again, want)
	})

	// userId によるデータ分離。ここが破れるとデータが漏れるので、必ず通すこと。
	t.Run("ユーザー分離: 他人の項目は見えず、変えられない", func(t *testing.T) {
		o := newOps(t)
		a := o.mk("id-1", "Aの項目")
		if err := o.create(ctx, userA, a); err != nil {
			t.Fatal(err)
		}

		// B から Get
		_, err := o.get(ctx, userB, "id-1")
		wantNotFound(t, err)

		// B から Update は ErrNotFound で、B の側に項目も作られない
		wantNotFound(t, o.update(ctx, userB, o.mk("id-1", "乗っ取り")))
		_, err = o.get(ctx, userB, "id-1")
		wantNotFound(t, err)

		// B の一覧は空
		list, err := o.list(ctx, userB)
		if err != nil || len(list) != 0 {
			t.Errorf("B の一覧 = %+v, %v; want 空", list, err)
		}

		// A の項目は変わっていない
		got, err := o.get(ctx, userA, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		jsonEq(t, got, a)
		listA, err := o.list(ctx, userA)
		if err != nil || len(listA) != 1 {
			t.Errorf("A の一覧 = %+v, %v", listA, err)
		}
	})

	t.Run("ユーザー分離: 同じ ID でもユーザーごとに別の項目", func(t *testing.T) {
		o := newOps(t)
		a := o.mk("id-1", "A")
		b := o.mk("id-1", "B")
		if err := o.create(ctx, userA, a); err != nil {
			t.Fatal(err)
		}
		if err := o.create(ctx, userB, b); err != nil {
			t.Fatalf("別ユーザーの同じ ID の作成は成功する: %v", err)
		}
		gotA, err := o.get(ctx, userA, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		jsonEq(t, gotA, a)
		gotB, err := o.get(ctx, userB, "id-1")
		if err != nil {
			t.Fatal(err)
		}
		jsonEq(t, gotB, b)
	})
}

func ptr[T any](v T) *T { return &v }
