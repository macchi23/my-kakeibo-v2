#!/bin/bash
# PreToolUse(Bash): 実 AWS への書き込み（cdk deploy / bootstrap / destroy 等、aws CLI の作成・変更・削除）を止める
# NOTE: 実 AWS への書き込みはユーザーが行う（CLAUDE.md）
#   - サブエージェント: 常に deny
#   - メイン: ask（ユーザーが Claude に実行させると決めた場合だけ承認する）
# NOTE: コマンドを区切り（&& || ; | ( ）で分け、各区切りの先頭が cdk / aws のものだけを見る。echo や grep の引数として含むだけのものは除く
# NOTE: 引用符の中身は先に消す（grep "cdk deploy\|cdk bootstrap" の `|` を区切りと誤認した）。ただし bash -c / eval の中身は消す前に取り出して判定する（消すだけだと素通りになった）
# NOTE: SSM / Secrets Manager の秘密を復号して読む操作も同じ扱い（値が会話ログに入るため）
command -v jq >/dev/null 2>&1 || { echo "jq が無いので判定できない（hook は fail-closed）" >&2; exit 2; }

INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""')
# bash -c "..." / sh -c '...' / eval "..." の中身は、引用符を消す前に取り出して別のコマンドとしても判定する
INNER=$(echo "$CMD" | grep -oE "(-c|eval)[[:space:]]+(\"[^\"]*\"|'[^']*')" | sed -E "s/^(-c|eval)[[:space:]]+//; s/^[\"']//; s/[\"']\$//")
PROBE=$(printf '%s\n%s\n' "$CMD" "$INNER" | sed -E "s/'[^']*'//g; s/\"[^\"]*\"//g")

CDK_ACTIONS='deploy|bootstrap|destroy|watch|import|rollback|gc'
AWS_WRITE='(create|delete|put|update|remove|terminate|admin|deploy|attach|detach|tag|untag|set|enable|disable|modify|invoke|cancel|reset|restore|import|batch|execute|continue|converse|start|stop|send|publish|run|assume|transact|register|deregister|associate|disassociate|revoke|authorize|change|copy|replace|rotate)(-[a-z0-9-]+)?'

set -f
found=""
while IFS= read -r seg; do
  # 先頭の空白、環境変数の代入（FOO=bar cmd）、前置コマンド（time / nohup / env / sudo など）を落とす
  prev=""
  while [ "$seg" != "$prev" ]; do
    prev="$seg"
    seg=$(echo "$seg" | sed -E 's/^[[:space:]]+//; s/^[A-Za-z_][A-Za-z0-9_]*=[^[:space:]]*[[:space:]]*//; s/^(time|nohup|sudo|exec|command|builtin)([[:space:]]+|$)//; s/^env([[:space:]]+-[^[:space:]]+)*([[:space:]]+|$)//')
  done
  first=${seg%% *}
  # shellcheck disable=SC2086
  set -- $seg
  case "$first" in
    npx|bunx|yarn|pnpm|npm|node|cdk|aws-cdk|*/cdk|*/aws-cdk)
      if [ "$first" = npm ] && [ "${2:-}" = run ] && echo "${3:-}" | grep -qE '^(deploy|bootstrap|destroy)(:|$)'; then
        found="npm run $3"; break
      fi
      seen=""
      for t in "$@"; do
        if [ -z "$seen" ]; then
          echo "$t" | grep -qE '^([^[:space:]]*/)?(cdk|aws-cdk)(@[^[:space:]]*)?$' && seen=1
        elif echo "$t" | grep -qE "^($CDK_ACTIONS)\$"; then
          found="cdk $t"; break 2
        fi
      done ;;
    aws)
      words=" $* "
      if echo "$words" | grep -qE ' get-secret-value |( get-parameters?(-by-path)? .*--with-decryption)'; then
        found="aws の秘密の復号読み取り"; break
      fi
      op=$(echo "$words" | grep -oE " $AWS_WRITE " | head -1 | tr -d ' ')
      if [ -z "$op" ] && echo "$words" | grep -qE ' s3 '; then
        op=$(echo "$words" | grep -oE ' (cp|mv|rm|sync|mb|rb) ' | head -1 | tr -d ' ')
      fi
      [ -n "$op" ] && { found="aws $op"; break; } ;;
  esac
done <<EOF
$(echo "$PROBE" | awk '{ gsub(/&&|\|\||[;|(]/, "\n"); print }')
EOF
set +f
[ -z "$found" ] && exit 0

decide() {
  jq -cn --arg d "$1" --arg r "$2" '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: $d, permissionDecisionReason: $r}}'
  exit 0
}

# メインスレッドでは agent_id が来ない
if [ -n "$(echo "$INPUT" | jq -r '.agent_id // ""')" ]; then
  decide deny "${found} はサブエージェントから実行しない（CLAUDE.md: 実 AWS への書き込みと秘密の読み取りはユーザーが行う）。"
fi

decide ask "${found} は実 AWS への書き込みか秘密の読み取り。ユーザーが実行する方針（CLAUDE.md）。Claude に実行させる場合だけ承認する。"
