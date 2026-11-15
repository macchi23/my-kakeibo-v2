// cmd/api はローカル開発用のサーバー。インメモリのストアと開発用の認証（X-Dev-User ヘッダー）を使う。
// 本番の Lambda は cmd/lambda。こちらは本番に入れない（devauth を含むため）。
//
//	cd backend && go run ./cmd/api
//	curl -H 'X-Dev-User: alice' 127.0.0.1:8080/accounts
//
// NOTE: ローカルの Next（別オリジン）から叩けるようにする CORS は P2 で足す。
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/macchi23/my-kakeibo-v2/backend/internal/devauth"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/httpapi"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/store"
)

func main() {
	// 構造化ログ（JSON）を標準エラーに出す。Python の logging に当たる slog を使う。
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	// ストアはインメモリ。プロセスを止めるとデータは消える。
	// 1 つの mem が口座とカテゴリ両方のインターフェースを満たすので、同じ値を 2 つのフィールドに渡す。
	mem := store.NewMemory()

	// ハンドラーの依存（ストア・時計・ロガー）をまとめて渡して、ルーティング済みの http.Handler を作る。
	api := httpapi.NewHandler(&httpapi.Server{
		Accounts:   mem,
		Categories: mem,
		Now:        func() time.Time { return time.Now().UTC() },
		Log:        logger,
	})

	// 127.0.0.1 だけで待ち受ける（開発用の認証は誰でもなりすませるので、ネットワークに公開しない）
	srv := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           devauth.Middleware(api),
		ReadHeaderTimeout: 10 * time.Second,
	}
	// ListenAndServe は止まるまで戻ってこない。戻ってきた = エラーなので、ログを出して異常終了する。
	logger.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
