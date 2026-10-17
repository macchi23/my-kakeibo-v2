#!/bin/bash
# PreToolUse(Bash): .env の読み取りをブロックする
# NOTE: permissions.deny の Read(.env) は Read ツールしか塞がないので、シェル経由の読み取りをここで止める
# NOTE: .env.local など接尾辞付き・サブディレクトリの .env も対象。.env.example / .sample / .template は追跡するファイルなので対象外
# NOTE: 読み取りコマンドはコマンドの位置（行頭・パイプ・; & ( の直後）に限る。
#       どこでも拾う形だと、`.`（シェルの . コマンド）がドットを含む文章に当たってコミットメッセージまで止まる（別プロジェクトでの失敗）
# WARN: python / node でファイルを開く経路は塞げない。最終防衛は gitleaks（scripts/check.sh secrets）
command -v jq >/dev/null 2>&1 || { echo "jq が無いので判定できない（hook は fail-closed）" >&2; exit 2; }

INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""')

PROBE=$(echo "$CMD" | sed -E 's/\.env\.(example|sample|template)([^A-Za-z0-9_.-]|$)/__ENVDOC__\2/g')

READERS='cat|bat|less|more|head|tail|grep|rg|ag|awk|sed|cut|sort|uniq|wc|strings|xxd|od|nl|tee|cp|mv|open|code|vim|nano|source|base64|diff|jq|tar|zip|find|xargs|python3?|node|env|git[[:space:]]+(show|cat-file)|\.'

# NOTE: .ENV のような大文字も同じファイルとして扱う（macOS は大文字小文字を区別しない）
if echo "$PROBE" | grep -qE "(^|[|;&(]|&&|\|\|)[[:space:]]*($READERS)[[:space:]][^|;&]*\.[Ee][Nn][Vv](\.[A-Za-z0-9_-]+)*($|[^A-Za-z0-9_.-])"; then
  cat <<'MSG' >&2
.env を読もうとしている。このファイルは読まない（CLAUDE.md）。
キー名の一覧や grep -c での有無確認も含めて触らないこと。
設定の書き方を確認したいときは .env.example を読む。
MSG
  exit 2
fi

exit 0
