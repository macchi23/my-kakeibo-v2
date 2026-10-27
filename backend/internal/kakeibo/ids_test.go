package kakeibo

import (
	"errors"
	"testing"
)

func TestNewID(t *testing.T) {
	t.Skip("TODO(L1): 実装したらこの行を消す")

	a, b := NewID(), NewID()
	if a == b {
		t.Fatalf("NewID が重複した: %s", a)
	}
	// UUIDv7 はバージョン nibble（15 文字目）が 7
	if len(a) != 36 || a[14] != '7' {
		t.Errorf("UUIDv7 の文字列ではない: %q", a)
	}
	got, err := ParseID(a)
	if err != nil || got != a {
		t.Errorf("NewID の結果が ParseID を通らない: got=%q err=%v", got, err)
	}
}

func TestParseID(t *testing.T) {
	t.Skip("TODO(L1): 実装したらこの行を消す")

	const canonical = "018f3b0a-0000-7000-8000-000000000000"
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "正規形", in: canonical, want: canonical},
		{name: "大文字は小文字に正規化", in: "018F3B0A-0000-7000-8000-000000000000", want: canonical},
		{name: "urn 形式も正規形にして返す", in: "urn:uuid:" + canonical, want: canonical},
		{name: "波括弧付きも正規形にして返す", in: "{" + canonical + "}", want: canonical},
		{name: "空文字", in: "", wantErr: true},
		{name: "UUID ではない", in: "abc", wantErr: true},
		{name: "キーの区切り文字 # を含む", in: "018f3b0a-0000-7000-8000-00000000000#", wantErr: true},
		{name: "別の項目種別を差し込む", in: "ACCOUNT#" + canonical, wantErr: true},
		{name: "前後の空白", in: " " + canonical, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseID(tt.in)
			if tt.wantErr {
				var ve ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("ValidationError を期待したが err=%v", err)
				}
				if ve.Field != "id" {
					t.Errorf("Field = %q, want %q", ve.Field, "id")
				}
				return
			}
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationErrorMessage(t *testing.T) {
	t.Skip("TODO(L1): 実装したらこの行を消す")

	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{name: "Field あり", err: ValidationError{Field: "name", Message: "必須です"}, want: "name: 必須です"},
		{name: "Field なし", err: ValidationError{Message: "不正です"}, want: "不正です"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
