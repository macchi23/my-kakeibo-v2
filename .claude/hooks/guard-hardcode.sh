#!/bin/bash
# PreToolUse(Edit|Write|MultiEdit): 秘密情報・環境固有値のハードコードを止める
# NOTE: リポジトリは公開前提（docs/DECISIONS.md）。hook は一次防衛で、最終防衛は gitleaks（scripts/check.sh secrets）
#   - 全ファイル: アクセスキー・秘密鍵・シークレットキーの形は deny
#   - コード・設定: ARN・12桁数字・リージョン名・メールアドレス・Cognito のプール ID は ask
#   - *.md: 12桁数字・メールアドレス・Cognito のプール ID は ask（ARN の形やリージョン名は説明で書くので見ない）
# NOTE: .claude/hooks/ は対象外（テストで偽の値を組み立てるため）
command -v jq >/dev/null 2>&1 || { echo "jq が無いので判定できない（hook は fail-closed）" >&2; exit 2; }

ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')
REL="${FILE_PATH#"$ROOT"/}"

case "$REL" in
  /*|.claude/hooks/*|*/node_modules/*|*/cdk.out/*) exit 0 ;;
esac

CONTENT=$(echo "$INPUT" | jq -r '[.tool_input.content, .tool_input.new_string, (.tool_input.edits[]?.new_string)] | map(select(. != null)) | join("\n")')

# 理由文にキーの値を出さない
if echo "$CONTENT" | grep -qE '(AKIA|ASIA)[0-9A-Z]{16}|-----BEGIN ([A-Z]+ )?PRIVATE KEY-----|[Ss][Ee][Cc][Rr][Ee][Tt][A-Za-z_]*["'"'"'`]?[[:space:]]*[:=][[:space:]]*["'"'"'`]?[A-Za-z0-9/+=]{40}([^A-Za-z0-9/+=]|$)'; then
  jq -cn --arg f "$REL" '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny",
    permissionDecisionReason: ($f + " にアクセスキー・秘密鍵・シークレットキーの形の文字列がある。シークレットは SSM Parameter Store に置き、リポジトリに書かない（CLAUDE.md）。")}}'
  exit 0
fi

# 偽の値は許す: アカウント ID 000000000000 / 111111111111、example.com などのメール
SCAN=$(echo "$CONTENT" | sed -E -e 's/000000000000|111111111111//g' -e 's/[A-Za-z0-9._%+-]+@example\.(com|org|net)//g')

REGION='(us|eu|ap|sa|ca|me|af|il|mx)-(gov-)?(east|west|north|south|central|northeast|southeast|northwest|southwest)-[0-9]'
ACCOUNT='(^|[^0-9])[0-9]{12}([^0-9]|$)'
EMAIL='[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}'
POOL_ID="${REGION}_[A-Za-z0-9]{9}"
case "$REL" in
  *.md) PATTERN="$ACCOUNT|$EMAIL|$POOL_ID" ;;
  *) PATTERN="arn:aws[a-z-]*:|$ACCOUNT|$REGION|$EMAIL" ;;
esac

HITS=$(echo "$SCAN" | grep -oE "$PATTERN" | sed -E 's/^[^0-9A-Za-z]//; s/[^0-9A-Za-z:-]$//' | sort -u | head -5 | paste -sd, -)
[ -z "$HITS" ] && exit 0

jq -cn --arg f "$REL" --arg h "$HITS" '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "ask",
  permissionDecisionReason: ($f + " にハードコードの疑い（" + $h + "）。アカウント ID・リージョン・ARN・メールアドレス・Cognito の ID はリポジトリに書かない（CLAUDE.md）。偽の値など正当な場合だけ承認する。")}}'
exit 0
