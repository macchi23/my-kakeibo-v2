// cmd/lambda は本番用の Lambda。DynamoDB のストアと JWT の検証を使う。
// internal/devauth を import してはいけない（scripts/check.sh が検査する）。
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/auth"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/httpapi"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/lambdahttp"
	"github.com/macchi23/my-kakeibo-v2/backend/internal/store"
)

func main() {
	// 構造化ログ（JSON）を標準エラーに出す。Lambda では CloudWatch Logs に集まる。
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	// テーブル名は CDK が環境変数で渡す。無いまま起動すると、リクエストのたびに失敗するので起動時に落とす。
	table := os.Getenv("TABLE_NAME")
	if table == "" {
		logger.Error("TABLE_NAME が設定されていない")
		os.Exit(1)
	}

	// リージョンと認証情報は SDK の標準解決（Lambda の実行ロールと環境変数）に任せる。
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Error("AWS 設定の読み込みに失敗", "error", err)
		os.Exit(1)
	}
	// DynamoDB のクライアントを作り、ストア（本番の実装）に渡す。
	db := store.NewDynamo(dynamodb.NewFromConfig(cfg), table)

	// ハンドラーの依存をまとめて渡す。cmd/api とは、ストアが DynamoDB である点だけが違う。
	api := httpapi.NewHandler(&httpapi.Server{
		Accounts:   db,
		Categories: db,
		Now:        func() time.Time { return time.Now().UTC() },
		Log:        logger,
	})

	// 内側から順に、api（業務処理）→ JWT の検証 → API Gateway のイベントを http.Handler に変換するアダプタ。
	// lambda.Start はイベントの待ち受けを始め、戻ってこない。
	lambda.Start(lambdahttp.Wrap(auth.RequireJWT(api)))
}
