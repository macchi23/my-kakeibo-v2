#!/usr/bin/env bash
# 品質チェックを 1 コマンドにまとめる
#   scripts/check.sh [go|infra|web|shell|hooks|secrets|all]   （既定: all）
# 終了コード: 0 = 通過 / 1 = どれかが失敗 / 2 = 実行不能（ツール未導入・依存未インストール・引数）
# NOTE: -e は付けない。失敗しても残りのステップを走らせ、最後にまとめて判定する
set -uo pipefail
cd "$(dirname "$0")/.." || exit 2

# 引数で検査の対象を選ぶ（省略すると all）。failed には、失敗したステップの名前を溜めていく。
target="${1:-all}"
failed=()

# 1 ステップを実行して結果を表示し、失敗したら名前を記録する（止めずに次へ進む）。
step() {  # step <名前> <コマンド...>
  local name=$1; shift
  echo "==> $name"
  if "$@"; then
    echo "ok: $name"
  else
    echo "FAILED: $name"
    failed+=("$name")
  fi
}

need() {  # need <コマンド>: 無ければ実行不能で終了
  command -v "$1" >/dev/null 2>&1 || { echo "check: $1 が見つからない（docs/setup.md の前提を参照）"; exit 2; }
}

# gofmt -l は整形が必要なファイル名を出力する。1 つでもあれば失敗にする。
gofmt_clean() {
  local out
  out=$(gofmt -l .)
  [ -z "$out" ] || { echo "gofmt が必要なファイル:"; echo "$out"; return 1; }
}

# 開発用の認証（X-Dev-User でなりすませる）が本番の Lambda に入っていないことを確かめる
lambda_without_devauth() {
  local deps
  deps=$(go list -deps ./cmd/lambda) || return 1
  if grep -q '/internal/devauth$' <<<"$deps"; then
    echo "cmd/lambda が internal/devauth に依存している"
    return 1
  fi
}

# Go の検査: 整形 → 依存の整合 → vet → テスト → devauth の混入 → Lambda 向け（arm64）のビルド。
# ( ... ) のサブシェルで cd して、元の場所に影響を与えない。
run_go() {
  if [ ! -f backend/go.mod ]; then echo "skip: go（backend/go.mod が無い）"; return; fi
  need go
  (
    cd backend || exit 2
    step gofmt gofmt_clean
    step "go mod tidy" go mod tidy -diff
    step "go vet" go vet ./...
    step "go test" go test ./...
    step "lambda without devauth" lambda_without_devauth
    step "lambda arm64 build" env GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o /dev/null ./cmd/lambda
    [ ${#failed[@]} -eq 0 ]
  ) || failed+=("go")
}

# CDK の検査: 型検査 → テスト → synth（テンプレートを作れること。AWS には何も反映しない）。
run_infra() {
  if [ ! -f infra/package.json ]; then echo "skip: infra（infra/package.json が無い）"; return; fi
  [ -d infra/node_modules ] || { echo "check: infra/node_modules が無い。cd infra && npm ci"; exit 2; }
  (
    cd infra || exit 2
    step "infra tsc" npx tsc --noEmit
    step "infra test" npx vitest run
    step "cdk synth" npx cdk synth -q
    [ ${#failed[@]} -eq 0 ]
  ) || failed+=("infra")
}

# Next.js の検査: lint と型検査。web/ がまだ無ければ skip する。
run_web() {
  if [ ! -f web/package.json ]; then echo "skip: web（web/package.json が無い）"; return; fi
  [ -d web/node_modules ] || { echo "check: web/node_modules が無い。cd web && npm ci"; exit 2; }
  (
    cd web || exit 2
    step "web lint" npm run lint
    step "web tsc" npx tsc --noEmit
    [ ${#failed[@]} -eq 0 ]
  ) || failed+=("web")
}

# 検査対象のシェルスクリプトの一覧（scripts/ と hook）。
shell_files() {
  ls scripts/*.sh .claude/hooks/*.sh 2>/dev/null
}

bash_syntax() {
  local f rc=0
  # macOS 標準の bash 3.2 でも動くことを確かめる
  for f in $(shell_files); do /bin/bash -n "$f" || rc=1; done
  return $rc
}

# シェルの検査: 構文 → shellcheck → scripts 自体のテスト。
run_shell() {
  need shellcheck
  step "bash -n" bash_syntax
  # shellcheck disable=SC2046
  step shellcheck shellcheck -S warning $(shell_files)
  need jq
  step "script tests" bash scripts/test_apply_cfn_exec_app_policy.sh
}

# hook（.claude/hooks）の判定が期待どおりかのテスト。
run_hooks() {
  need jq
  step "hook tests" bash .claude/hooks/test_guards.sh
}

# NOTE: hook は一次防衛で、公開リポジトリへの秘密情報の混入はここが最終防衛
#   - 履歴: コミット済みの全履歴
#   - 作業ツリー: 追跡中と未追跡（.gitignore 対象外）のファイル。コミット前に混入を見つけるため
scan_history() {
  git rev-parse --verify -q HEAD >/dev/null || { echo "コミットが無いので履歴は skip"; return 0; }
  gitleaks git --redact --no-banner .
}

scan_worktree() {
  # 対象のファイルだけを一時ディレクトリに写して検査する（.gitignore 済みの .env や notes を巻き込まないため）。
  local tmp rc
  tmp=$(mktemp -d) || return 1
  git ls-files -co --exclude-standard -z | while IFS= read -r -d '' f; do
    [ -f "$f" ] || continue
    mkdir -p "$tmp/$(dirname "$f")" && cp "$f" "$tmp/$f"
  done
  gitleaks dir --redact --no-banner "$tmp"; rc=$?
  rm -rf "$tmp"
  return $rc
}

run_secrets() {
  need gitleaks
  step "gitleaks history" scan_history
  step "gitleaks worktree" scan_worktree
}

# 引数に応じて、検査を選んで実行する。
case "$target" in
  go) run_go ;;
  infra) run_infra ;;
  web) run_web ;;
  shell) run_shell ;;
  hooks) run_hooks ;;
  secrets) run_secrets ;;
  all) run_go; run_infra; run_web; run_shell; run_hooks; run_secrets ;;
  *) echo "usage: scripts/check.sh [go|infra|web|shell|hooks|secrets|all]"; exit 2 ;;
esac

# 失敗が 1 つでもあれば終了コード 1、無ければ 0。
if [ ${#failed[@]} -eq 0 ]; then
  echo "check: ok ($target)"
  exit 0
fi
echo "check: FAILED ($target): ${failed[*]}"
exit 1
