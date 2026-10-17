#!/bin/bash
# PreToolUse(Bash): USE_REAL_API=1 を含むコマンドをユーザーの承認制にし、記録を残す
# NOTE: 外部 API（Bedrock / jev）は既定でモック。実 API は明示フラグがあり、かつユーザーが承認したときだけ（CLAUDE.md）
# WARN: 文字列として含むだけのコマンド（echo 等）も対象になる。ask なので実害はない
ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""')

echo "$CMD" | grep -qE "USE_REAL_API=[\"']?1" || exit 0

LOG_DIR="$ROOT/.claude/logs"
mkdir -p "$LOG_DIR"
jq -cn --arg ts "$(date '+%Y-%m-%d %H:%M:%S')" --arg agent "$(echo "$INPUT" | jq -r '.agent_type // "main"')" \
  --arg cmd "$CMD" --arg sid "$(echo "$INPUT" | jq -r '.session_id // ""')" \
  '{ts: $ts, agent: $agent, command: $cmd, session: $sid}' >> "$LOG_DIR/real-api.jsonl" 2>/dev/null

jq -cn '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "ask",
  permissionDecisionReason: "実 API（USE_REAL_API=1）を呼ぶコマンド。課金が発生する。呼び出し回数を確認して承認する。"}}'
exit 0
