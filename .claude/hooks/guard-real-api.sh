#!/bin/bash
# PreToolUse(Bash): USE_REAL_API=1 を含むコマンドをユーザーの承認制にし、記録を残す
# NOTE: 外部 API（Bedrock / jev）は既定でモック。実 API は明示フラグがあり、かつユーザーが承認したときだけ（CLAUDE.md）
# WARN: 文字列として含むだけのコマンド（echo 等）も対象になる。ask なので実害はない
ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
# hook は標準入力に JSON でツール呼び出しの内容を受け取る。ここから実行しようとしているコマンドを取り出す。
INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""')

# USE_REAL_API=1 が無ければ何もせず通す（exit 0 で出力なし = 判定に関与しない）。
echo "$CMD" | grep -qE "USE_REAL_API=[\"']?1" || exit 0

# 誰がいつ何を実行しようとしたかを、後から見返せるよう JSON Lines で記録する。
LOG_DIR="$ROOT/.claude/logs"
mkdir -p "$LOG_DIR"
jq -cn --arg ts "$(date '+%Y-%m-%d %H:%M:%S')" --arg agent "$(echo "$INPUT" | jq -r '.agent_type // "main"')" \
  --arg cmd "$CMD" --arg sid "$(echo "$INPUT" | jq -r '.session_id // ""')" \
  '{ts: $ts, agent: $agent, command: $cmd, session: $sid}' >> "$LOG_DIR/real-api.jsonl" 2>/dev/null

# 実行は止めず、ユーザーに承認を求める（permissionDecision: ask）。
jq -cn '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "ask",
  permissionDecisionReason: "実 API（USE_REAL_API=1）を呼ぶコマンド。課金が発生する。呼び出し回数を確認して承認する。"}}'
exit 0
