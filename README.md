# my-kakeibo-v2

個人用の家計簿アプリ。Go 未経験者が Claude Code と一緒に Go（AWS Lambda）を学びながら作っている。

- バックエンド: Go（Lambda, `provided.al2023` arm64）+ API Gateway HTTP API + DynamoDB
- 認証: Amazon Cognito
- IaC: AWS CDK（TypeScript）
- フロントエンド: Next.js（静的書き出し）+ Amplify Hosting
- レシート読み取り: Amazon Bedrock、品目のカテゴリ分類: jev（TypeSafe）

## ドキュメント

| ファイル | 内容 |
|---|---|
| [docs/DECISIONS.md](docs/DECISIONS.md) | 設計判断とその理由 |
| [docs/setup.md](docs/setup.md) | セットアップと再構築の手順 |
| [docs/go/](docs/go/) | Go の学習資料と進捗 |
| [CLAUDE.md](CLAUDE.md) | Claude Code 向けのルール（ハーネス） |

## 品質チェック

```sh
scripts/check.sh all   # go / infra / web / shell / hooks
```
