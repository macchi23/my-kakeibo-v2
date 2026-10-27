package kakeibo

import (
	"errors"
	"strings"
	"testing"
)

func TestAccountInputValidate(t *testing.T) {
	t.Skip("TODO(L1): 実装したらこの行を消す")

	tests := []struct {
		name      string
		in        AccountInput
		wantField string // 空なら成功を期待する
	}{
		{name: "正常", in: AccountInput{Name: "三井住友", Kind: AccountKindBank}},
		{name: "全部の kind", in: AccountInput{Name: "x", Kind: AccountKindEMoney}},
		{name: "名前は 50 文字ちょうど（英字）", in: AccountInput{Name: strings.Repeat("a", 50), Kind: AccountKindCash}},
		{name: "名前は 50 文字ちょうど（日本語。バイト数ではなく rune 数）", in: AccountInput{Name: strings.Repeat("あ", 50), Kind: AccountKindCash}},
		{name: "名前は 51 文字", in: AccountInput{Name: strings.Repeat("あ", 51), Kind: AccountKindCash}, wantField: "name"},
		{name: "名前が空", in: AccountInput{Name: "", Kind: AccountKindBank}, wantField: "name"},
		{name: "名前が空白だけ", in: AccountInput{Name: " 　\t", Kind: AccountKindBank}, wantField: "name"},
		{name: "前後の空白は除いて数える", in: AccountInput{Name: " " + strings.Repeat("a", 50) + " ", Kind: AccountKindBank}},
		{name: "kind が空", in: AccountInput{Name: "x"}, wantField: "kind"},
		{name: "kind が列挙外", in: AccountInput{Name: "x", Kind: "bank"}, wantField: "kind"},
		{name: "マイナスの開始残高は許す（カードの未払い）", in: AccountInput{Name: "カード", Kind: AccountKindCard, OpeningBalance: -3000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("予期しないエラー: %v", err)
				}
				return
			}
			var ve ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("ValidationError を期待したが err=%v", err)
			}
			if ve.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", ve.Field, tt.wantField)
			}
		})
	}
}
