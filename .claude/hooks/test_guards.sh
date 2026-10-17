#!/bin/bash
# .claude/hooks/guard-*.sh の検証。塞ぐべきもの / 通すべきものを両方見る。
#
#     bash .claude/hooks/test_guards.sh
#
# NOTE: このファイル自身に ".env"、アクセスキーの形、12桁数字をそのまま書かない。
#       ".env" はコマンド行が guard-dotenv に当たり、キーの形は GitHub の push protection に当たるため、文字列を組み立てる

DIR="$(cd "$(dirname "$0")" && pwd)"
DOT=".env"
FAKE_KEY="AKIA""ABCDEFGHIJKLMNOP"
ACCOUNT="1234""5678""9012"
FAKE_SECRET="wJalrXUtnFEMI/K7MDENG""/bPxRfiCYEXAMPLEKEY"  # gitleaks:allow（テスト用の偽値）
ROOT="/proj"
FAIL=0

json() { python3 -c 'import json,sys;print(json.dumps(sys.argv[1]))' "$1"; }

report() {  # report <want> <got> <説明>
  if [ "$1" = "$2" ]; then printf 'OK   '; else printf 'NG   '; FAIL=$((FAIL + 1)); fi
  printf 'want=%s got=%s  %s\n' "$1" "$2" "$3"
}

# 判定: JSON の permissionDecision、exit 2 は block、何も出なければ allow
decision() {  # decision <hook> <入力JSON>
  local out code
  out=$(printf '%s' "$2" | CLAUDE_PROJECT_DIR="$ROOT" "$DIR/$1" 2>/dev/null); code=$?
  if [ "$code" = 2 ]; then echo block; return; fi
  [ -z "$out" ] && { echo allow; return; }
  echo "$out" | jq -r '.hookSpecificOutput.permissionDecision'
}

bash_cmd() {  # bash_cmd <hook> <want> <コマンド> [agent_id]
  local input
  if [ -n "${4:-}" ]; then
    input=$(printf '{"tool_input":{"command":%s},"agent_id":"%s"}' "$(json "$3")" "$4")
  else
    input=$(printf '{"tool_input":{"command":%s}}' "$(json "$3")")
  fi
  report "$2" "$(decision "$1" "$input")" "$1: $3${4:+ (subagent)}"
}

write_file() {  # write_file <want> <相対パス> <内容>
  local input
  input=$(printf '{"tool_input":{"file_path":%s,"content":%s}}' "$(json "$ROOT/$2")" "$(json "$3")")
  report "$1" "$(decision guard-hardcode.sh "$input")" "guard-hardcode: $2 <- $3"
}

echo "=== guard-dotenv ==="
bash_cmd guard-dotenv.sh block "cat $DOT"
bash_cmd guard-dotenv.sh block "grep -c KEY $DOT"
bash_cmd guard-dotenv.sh block "head -5 ./$DOT"
bash_cmd guard-dotenv.sh block "source $DOT"
bash_cmd guard-dotenv.sh block ". $DOT"
bash_cmd guard-dotenv.sh block "ls; cat $DOT"
bash_cmd guard-dotenv.sh block "echo x && cat $DOT"
bash_cmd guard-dotenv.sh block "cp $DOT /tmp/leak"
bash_cmd guard-dotenv.sh allow "cat ${DOT}.example"
bash_cmd guard-dotenv.sh allow "cat .envrc"
bash_cmd guard-dotenv.sh allow "git status --short"
bash_cmd guard-dotenv.sh allow "git commit -m 'cat で読むのを塞ぐ。${DOT}.example は通す'"
bash_cmd guard-dotenv.sh block "cat web/${DOT}.local"
bash_cmd guard-dotenv.sh block "head backend/$DOT"
bash_cmd guard-dotenv.sh block "base64 ${DOT}.production"
bash_cmd guard-dotenv.sh allow "cat web/${DOT}.example"
bash_cmd guard-dotenv.sh block "env -i cat $DOT"
bash_cmd guard-dotenv.sh block "find . -name $DOT -exec cat {} +"
bash_cmd guard-dotenv.sh block "git show HEAD:$DOT"
bash_cmd guard-dotenv.sh block "jq . web/${DOT}.json"
bash_cmd guard-dotenv.sh block "tar cf - $DOT"
bash_cmd guard-dotenv.sh block "cat .ENV"
bash_cmd guard-dotenv.sh block "python3 -c \"print(open('$DOT').read())\""
bash_cmd guard-dotenv.sh allow "git show HEAD:docs/DECISIONS.md"
bash_cmd guard-dotenv.sh allow "python3 scripts/x.py"
bash_cmd guard-dotenv.sh allow "find . -name '*.go'"
bash_cmd guard-dotenv.sh block "cat ${DOT}.example.bak2"

echo
echo "=== guard-deploy ==="
bash_cmd guard-deploy.sh ask "npx cdk deploy --profile my-kakeibo-v2"
bash_cmd guard-deploy.sh ask "cd infra && npx cdk bootstrap"
bash_cmd guard-deploy.sh ask "cdk destroy"
bash_cmd guard-deploy.sh deny "npx cdk deploy" "agent-123"
bash_cmd guard-deploy.sh allow "npx cdk synth -q"
bash_cmd guard-deploy.sh allow "npx cdk diff --profile my-kakeibo-v2"
bash_cmd guard-deploy.sh allow "echo 'cdk deploy はユーザーが行う'"
bash_cmd guard-deploy.sh allow "grep -r 'cdk deploy' docs"
bash_cmd guard-deploy.sh allow "grep -rn \"cdk deploy\\|cdk bootstrap\" docs" "agent-123"
bash_cmd guard-deploy.sh allow "echo 'x | cdk deploy'" "agent-123"
bash_cmd guard-deploy.sh deny "cd infra && npx cdk deploy -c budgetEmail='a@example.com'" "agent-123"
bash_cmd guard-deploy.sh ask "npx cdk --profile my-kakeibo-v2 deploy"
bash_cmd guard-deploy.sh ask "npx cdk watch"
bash_cmd guard-deploy.sh ask "./node_modules/.bin/cdk deploy"
bash_cmd guard-deploy.sh ask "npm exec cdk deploy"
bash_cmd guard-deploy.sh ask "npx aws-cdk deploy"
bash_cmd guard-deploy.sh ask "npx cdk gc"
bash_cmd guard-deploy.sh ask "npm run deploy"
bash_cmd guard-deploy.sh ask "aws cognito-idp admin-create-user --user-pool-id x --username y"
bash_cmd guard-deploy.sh ask "aws --profile my-kakeibo-v2 cloudformation delete-stack --stack-name x"
bash_cmd guard-deploy.sh ask "aws ssm put-parameter --name /x --value y"
bash_cmd guard-deploy.sh ask "aws s3 rm s3://bucket/key"
bash_cmd guard-deploy.sh deny "aws cloudformation deploy --template-file t.json" "agent-123"
bash_cmd guard-deploy.sh allow "aws cloudformation list-stacks --profile my-kakeibo"
bash_cmd guard-deploy.sh allow "aws cognito-idp describe-user-pool --user-pool-id x"
bash_cmd guard-deploy.sh allow "aws s3 ls"
bash_cmd guard-deploy.sh allow "aws sso login --profile my-kakeibo-v2"
bash_cmd guard-deploy.sh allow "AWS_PROFILE=x aws sts get-caller-identity" "agent-123"
bash_cmd guard-deploy.sh allow "git commit -m 'guard: aws の create 系を止める'"
# ラッパー・前置コマンド経由（引用符を消す処理で素通りになった経路）
bash_cmd guard-deploy.sh deny "bash -c \"cd infra && npx cdk deploy\"" "agent-123"
bash_cmd guard-deploy.sh deny "sh -c 'npx cdk deploy'" "agent-123"
bash_cmd guard-deploy.sh deny "eval \"npx cdk destroy\"" "agent-123"
bash_cmd guard-deploy.sh deny "time npx cdk deploy" "agent-123"
bash_cmd guard-deploy.sh deny "nohup env -i npx cdk deploy" "agent-123"
bash_cmd guard-deploy.sh deny "npx --yes cdk deploy" "agent-123"
bash_cmd guard-deploy.sh deny "npx -p aws-cdk cdk deploy" "agent-123"
bash_cmd guard-deploy.sh deny "npx cdk@2 deploy" "agent-123"
bash_cmd guard-deploy.sh deny "npm exec -- cdk deploy" "agent-123"
bash_cmd guard-deploy.sh deny "node node_modules/aws-cdk/bin/cdk deploy" "agent-123"
bash_cmd guard-deploy.sh deny "npm run deploy:prod" "agent-123"
bash_cmd guard-deploy.sh deny "aws dynamodb batch-write-item --request-items x" "agent-123"
bash_cmd guard-deploy.sh deny "aws cloudformation execute-change-set --change-set-name x" "agent-123"
bash_cmd guard-deploy.sh deny "aws ssm get-parameter --name /x --with-decryption" "agent-123"
bash_cmd guard-deploy.sh deny "aws secretsmanager get-secret-value --secret-id x" "agent-123"
bash_cmd guard-deploy.sh allow "aws ssm get-parameter --name /x" "agent-123"
bash_cmd guard-deploy.sh allow "bash -c \"npx cdk synth -q\"" "agent-123"
bash_cmd guard-deploy.sh allow "npx vitest run" "agent-123"
bash_cmd guard-deploy.sh allow "npm run build" "agent-123"

echo
echo "=== guard-real-api ==="
bash_cmd guard-real-api.sh ask "USE_REAL_API=1 go run ./cmd/probe"
bash_cmd guard-real-api.sh ask "env USE_REAL_API='1' go test ./..."
bash_cmd guard-real-api.sh allow "go test ./..."
bash_cmd guard-real-api.sh allow "USE_REAL_API=0 go test ./..."

echo
echo "=== guard-hardcode ==="
write_file deny "infra/lib/auth.ts" "const key = '$FAKE_KEY'"
write_file deny "backend/internal/x.go" "-----BEGIN RSA PRIVATE KEY-----"
write_file ask "infra/lib/auth.ts" "const account = '$ACCOUNT'"
write_file ask "infra/lib/auth.ts" "const arn = 'arn:aws:s3:::bucket'"
write_file ask "backend/cmd/api/main.go" "region := \"us-east-1\""
write_file ask ".github/workflows/ci.yml" "aws-region: ap-northeast-1"
write_file allow "infra/test/auth.test.ts" "const account = '000000000000'"
write_file allow "backend/internal/x_test.go" "account := \"111111111111\""
write_file ask "infra/test/auth.test.ts" "const account = '$ACCOUNT'"
write_file allow "infra/lib/auth.ts" "Stack.of(this).region"
write_file allow "infra/lib/auth.ts" "const amount = 1234567890123"
write_file allow "docs/DECISIONS.md" "リージョンは us-east-1"
write_file allow ".claude/hooks/x.sh" "region=us-east-1"
write_file ask "docs/handover.md" "account $ACCOUNT"
write_file ask "docs/handover.md" "UserPoolId: us-east-1_AbCdEf123"
write_file ask "docs/articles/1-0.md" "--username someone@mail.test"
write_file allow "docs/handover.md" "--username you@example.com"
write_file allow "docs/handover.md" "arn:aws:iam::<アカウント ID>:role/x"
write_file ask "infra/cdk.json" "\"budgetEmail\": \"someone@mail.test\""
write_file ask ".claude/settings.json" "\"region\": \"us-east-1\""
write_file deny ".claude/settings.json" "\"AWS_SECRET_ACCESS_KEY\": \"$FAKE_SECRET\""
write_file deny "infra/lib/x.ts" "const secret = '$FAKE_SECRET'"
write_file deny "docs/handover.md" "export AWS_SECRET_ACCESS_KEY=$FAKE_SECRET"
write_file deny "docs/handover.md" "aws_secret_access_key = $FAKE_SECRET"
write_file deny "web/src/x.ts" "const clientSecret = \`$FAKE_SECRET\`"

echo
if [ "$FAIL" -eq 0 ]; then echo "OK: 全部期待どおり"; else echo "NG: $FAIL 件ずれている"; fi
exit "$FAIL"
