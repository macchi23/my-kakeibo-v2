#!/usr/bin/env bash
# apply-cfn-exec-app-policy.sh のテスト。偽の aws だけが見える環境（env -i）で、一時リポジトリから実行する。実 AWS には触れない
#   bash scripts/test_apply_cfn_exec_app_policy.sh
# 実の認証情報・実の aws に届かないよう、準備の失敗では即終了し、各ケースの前に command -v aws が偽物を指すことを確かめる
# 本番と同じ形（git show refs/remotes/origin/main:... | bash -s）で、一時リポジトリの bare の origin に push した版を実行する
set -euo pipefail
cd "$(dirname "$0")/.." || exit 2
REPO=$(pwd)
SCRIPT_PATH=scripts/apply-cfn-exec-app-policy.sh
POLICY_PATH=infra/iam/cfn-exec-app-policy.json

command -v jq >/dev/null 2>&1 || { echo "jq が見つからない" >&2; exit 2; }
command -v git >/dev/null 2>&1 || { echo "git が見つからない" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "python3 が見つからない（端末の確認のテストに使う）" >&2; exit 2; }
JQ_DIR=$(dirname "$(command -v jq)")
GIT_DIR_BIN=$(dirname "$(command -v git)")

work=$(mktemp -d) || exit 2
[ -d "$work" ] || exit 2
trap 'rm -rf "$work"' EXIT
mkdir "$work/bin" "$work/home"
: > "$work/aws_config"
: > "$work/aws_credentials"
export AWS_LOG="$work/aws.log"
CREATED_DOC="$work/created.json"
ORIGIN="$work/origin.git"
API_ID=abcde12345

# origin の URL の許可リストだけを、一時リポジトリの bare の origin に差し替えたコピー。
# 本番のスクリプトには差し替えの手段（引数・環境変数）を置かない。差し替えが 1 行だけであることを確かめる
PATCHED="$work/patched.sh"
sed "s|^ORIGIN_URLS=.*|ORIGIN_URLS=\"${ORIGIN}\"|" "$REPO/$SCRIPT_PATH" > "$PATCHED"
[ "$(diff "$REPO/$SCRIPT_PATH" "$PATCHED" | grep -c '^[<>]')" = 2 ] || { echo "致命的: ORIGIN_URLS の差し替えが 1 行ではない" >&2; exit 2; }

# 偽の aws。サブコマンドごとに、環境変数 FAKE_* で決めた出力を返す
cat > "$work/bin/aws" <<'FAKE'
#!/usr/bin/env bash
echo "$*" >> "$AWS_LOG"
svc=""; op=""; doc=""; vid=""
args=("$@")
for ((i = 0; i < ${#args[@]}; i++)); do
  case "${args[$i]}" in
    --profile|--region|--output|--policy-arn|--stack-name|--api-id|--policy-type) i=$((i + 1)) ;;
    --version-id) vid="${args[$((i + 1))]}"; i=$((i + 1)) ;;
    --policy-document) doc="${args[$((i + 1))]#file://}"; i=$((i + 1)) ;;
    --*) ;;
    *) if [ -z "$svc" ]; then svc="${args[$i]}"; elif [ -z "$op" ]; then op="${args[$i]}"; fi ;;
  esac
done
case "$svc $op" in
  "sts get-caller-identity") echo '{"Account":"111122223333"}' ;;
  "cloudformation describe-stack-resources")
    case "${FAKE_API_COUNT:-1}" in
      0) echo '{"StackResources":[{"ResourceType":"AWS::DynamoDB::Table","PhysicalResourceId":"t"}]}' ;;
      2) echo '{"StackResources":[{"ResourceType":"AWS::ApiGatewayV2::Api","PhysicalResourceId":"abcde12345"},{"ResourceType":"AWS::ApiGatewayV2::Api","PhysicalResourceId":"fghij67890"}]}' ;;
      *) printf '{"StackResources":[{"ResourceType":"AWS::DynamoDB::Table","PhysicalResourceId":"t"},{"ResourceType":"AWS::ApiGatewayV2::Api","PhysicalResourceId":"%s"}]}\n' "${FAKE_API_ID:-abcde12345}" ;;
    esac ;;
  "apigatewayv2 get-api") printf '{"Name":"%s","Tags":{"Project":"%s"}}\n' "${FAKE_NAME:-my-kakeibo-v2-api}" "${FAKE_TAG:-my-kakeibo-v2}" ;;
  "accessanalyzer validate-policy")
    if [ -n "${FAKE_VALIDATE_EXTRA:-}" ]; then
      echo '{"findings":[{"findingType":"ERROR","issueCode":"INVALID_ACTION","findingDetails":"The action apigateway:TagResource does not exist."},{"findingType":"ERROR","issueCode":"INVALID_ARN","findingDetails":"x"}]}'
    elif [ -n "${FAKE_SECURITY_WARNING:-}" ]; then
      echo '{"findings":[{"findingType":"SECURITY_WARNING","issueCode":"PASS_ROLE_WITH_STAR_IN_RESOURCE","findingDetails":"too broad"}]}'
    else
      echo '{"findings":[{"findingType":"ERROR","issueCode":"INVALID_ACTION","findingDetails":"The action apigateway:TagResource does not exist."},{"findingType":"SUGGESTION","issueCode":"X","findingDetails":"y"}]}'
    fi ;;
  "iam list-policy-versions")
    n="${FAKE_VERSIONS:-2}"
    jq -n --argjson n "$n" '{Versions: [range(0; $n) | {VersionId: ("v" + (. + 1 | tostring)), IsDefaultVersion: (. == ($n - 1)), CreateDate: ("2026-01-0" + (. + 1 | tostring) + "T00:00:00Z")}]}' ;;
  "iam create-policy-version") cp "$doc" "$CREATED_DOC"; echo '{"PolicyVersion":{"VersionId":"v9","IsDefaultVersion":true}}' ;;
  "iam get-policy-version")
    if [ "$vid" != v9 ]; then
      echo '{"PolicyVersion":{"Document":{"Version":"2012-10-17","Statement":[]}}}'
    elif [ -n "${FAKE_GET_PLACEHOLDER:-}" ]; then
      jq -n --slurpfile d infra/iam/cfn-exec-app-policy.json '{PolicyVersion:{Document:$d[0]}}'
    elif [ -n "${FAKE_GET_ENCODED:-}" ]; then
      enc=$(jq -rj '@uri' "$CREATED_DOC")
      jq -n --arg d "$enc" '{PolicyVersion:{Document:$d}}'
    else
      jq -n --slurpfile d "$CREATED_DOC" '{PolicyVersion:{Document:$d[0]}}'
    fi ;;
  *) echo "偽の aws: 想定外の呼び出し: $*" >&2; exit 99 ;;
esac
FAKE
chmod +x "$work/bin/aws"

# 端末（pty）で実行して、確認の入力を流し込む。子は pty を制御端末に持つので /dev/tty が開ける
cat > "$work/pty_run.py" <<'PY'
import os, pty, sys
answer = sys.argv[1]
argv = sys.argv[2:]
pid, fd = pty.fork()
if pid == 0:
    os.execvp(argv[0], argv)
out = b""
sent = False
while True:
    try:
        data = os.read(fd, 4096)
    except OSError:
        break
    if not data:
        break
    out += data
    if not sent and b"[y/N]" in out:
        os.write(fd, (answer + "\n").encode())
        sent = True
sys.stdout.write(out.decode(errors="replace"))
_, status = os.waitpid(pid, 0)
sys.exit(os.waitstatus_to_exitcode(status))
PY

# 制御端末を持たない状態で実行する（新しいセッション）。テストを端末から走らせても /dev/tty が開けないようにする
cat > "$work/notty_run.py" <<'PY'
import os, sys
pid = os.fork()
if pid == 0:
    os.setsid()
    os.execvp(sys.argv[1], sys.argv[1:])
_, status = os.waitpid(pid, 0)
sys.exit(os.waitstatus_to_exitcode(status))
PY

gitc() { git -C "$REPO_COPY" -c user.name=t -c user.email=t@example.com -c commit.gpgsign=false "$@"; }

# 一時リポジトリと bare の origin。PRE_PUSH で JSON を変えてから push する（origin の main が反映元になる）
#   ORIGIN_MODE: patched（許可リストを origin に差し替えたスクリプト）/ real（本番のまま。origin は一時パス）/ insteadof（本番のまま。origin は GitHub の URL を insteadOf で一時パスに書き換え）
REPO_COPY="$work/repo"
GITHUB_URL="https://github.com/macchi23/my-kakeibo-v2.git"
make_repo() {
  rm -rf "$REPO_COPY" "$ORIGIN" "$work/origin.gone"
  mkdir -p "$REPO_COPY/scripts" "$REPO_COPY/infra/iam"
  if [ "$ORIGIN_MODE" = patched ]; then cp "$PATCHED" "$REPO_COPY/$SCRIPT_PATH"; else cp "$REPO/$SCRIPT_PATH" "$REPO_COPY/$SCRIPT_PATH"; fi
  cp "$REPO/$POLICY_PATH" "$REPO_COPY/$POLICY_PATH"
  if [ -n "$PRE_PUSH" ]; then "$PRE_PUSH"; fi
  git init -q --bare "$ORIGIN"
  git -C "$REPO_COPY" init -q -b main
  gitc add -A
  gitc commit -q -m init
  if [ "$ORIGIN_MODE" = insteadof ]; then
    git -C "$REPO_COPY" remote add origin "$GITHUB_URL"
    git -C "$REPO_COPY" config "url.${ORIGIN}.insteadOf" "$GITHUB_URL"
  else
    git -C "$REPO_COPY" remote add origin "$ORIGIN"
  fi
  git -C "$REPO_COPY" push -q origin main
  # 手順書の `git fetch origin` に当たる
  git -C "$REPO_COPY" fetch -q origin
}

# JSON の書き換え（jq の引数と式を、指定のファイルに当てる）
edit_json() {  # edit_json <ファイル> [--arg 名前 値]... <jq の式>
  local f=$1 t
  shift
  t=$(mktemp)
  jq "$@" "$f" > "$t" && mv "$t" "$f"
}
apigw_prefix() { jq -r '.Statement[] | select(.Sid == "ApiGatewayCreateTaggedHttpApi") | .Resource' "$REPO/$POLICY_PATH"; }
APIS=$(apigw_prefix)
[ -n "$APIS" ] || { echo "致命的: ApiGatewayCreateTaggedHttpApi の Resource を読めない" >&2; exit 2; }
TAGS="${APIS%/apis}/tags/*"

# origin に push する前の書き換え（PRE_PUSH）
pre_apis_wildcard() { edit_json "$REPO_COPY/$POLICY_PATH" --arg r "${APIS}/??????????/*" '(.Statement[] | select(.Sid == "ApiGatewayManageProjectHttpApi") | .Resource) += [$r]'; }
pre_tags_wildcard() { edit_json "$REPO_COPY/$POLICY_PATH" --arg r "$TAGS" '(.Statement[] | select(.Sid == "ApiGatewayTagOnCreateHttpApi") | .Resource) |= [., $r]'; }
pre_other_stage_id() { edit_json "$REPO_COPY/$POLICY_PATH" --arg r "${APIS}/zzzzzzzzzz/stages" '(.Statement[] | select(.Sid == "ApiGatewayManageProjectStage") | .Resource) |= [$r, .[1]]'; }
pre_new_apigw_sid() { edit_json "$REPO_COPY/$POLICY_PATH" --arg r "${APIS}/*" '.Statement += [{Sid: "Extra", Effect: "Allow", Action: "APIGateway:GET", Resource: $r}]'; }
pre_not_resource() { edit_json "$REPO_COPY/$POLICY_PATH" '.Statement += [{Sid: "Extra", Effect: "Allow", Action: "s3:GetObject", NotResource: "arn:aws:s3:::x"}]'; }
pre_action_star() { edit_json "$REPO_COPY/$POLICY_PATH" '.Statement += [{Sid: "Extra", Effect: "Allow", Action: "*", Resource: "arn:aws:s3:::x"}]'; }
pre_service_star() { edit_json "$REPO_COPY/$POLICY_PATH" '.Statement += [{Sid: "Extra", Effect: "Allow", Action: ["s3:GetObject", "iam:*"], Resource: "arn:aws:s3:::x"}]'; }

# push の後の、作業ツリーと .git の改ざん（MUTATE）。どれも反映される内容を変えられないこと
tamper_worktree_json() { edit_json "$REPO_COPY/$POLICY_PATH" '.Statement[0].Sid = "TAMPERED"'; }
mutate_worktree() { tamper_worktree_json; }
mutate_assume_unchanged() { tamper_worktree_json; git -C "$REPO_COPY" update-index --assume-unchanged "$POLICY_PATH"; }
mutate_local_commit() { tamper_worktree_json; gitc commit -q -am tampered; }
mutate_update_ref() {
  gitc checkout -q -b evil
  tamper_worktree_json
  gitc commit -q -am tampered
  git -C "$REPO_COPY" update-ref refs/remotes/origin/main evil
}
mutate_tag_shadow() {
  mutate_update_ref
  git -C "$REPO_COPY" tag origin/main evil
  git -C "$REPO_COPY" update-ref refs/remotes/origin/main "$(git -C "$ORIGIN" rev-parse main)"
}
mutate_fetch_fails() { mv "$ORIGIN" "$work/origin.gone"; }

# 反映された文書が、origin の main の JSON に ID を埋めたものと一致すること
created_matches_origin() {
  [ -f "$CREATED_DOC" ] || { echo "    create-policy-version に渡された文書が無い"; return 1; }
  ! grep -q TAMPERED "$CREATED_DOC" || { echo "    改ざんした内容が反映された"; return 1; }
  local want got
  want=$(git -C "$ORIGIN" cat-file blob "main:$POLICY_PATH" | sed "s/__HTTP_API_ID__/${API_ID}/g" | jq -S .)
  got=$(jq -S . "$CREATED_DOC")
  [ "$want" = "$got" ] || { echo "    反映された文書が origin の main と違う"; return 1; }
}

pass=0
fail=0
FAKE_VARS="FAKE_API_COUNT FAKE_API_ID FAKE_NAME FAKE_TAG FAKE_VALIDATE_EXTRA FAKE_SECURITY_WARNING FAKE_VERSIONS FAKE_GET_PLACEHOLDER FAKE_GET_ENCODED"
SAFE_PATH="$work/bin:$JQ_DIR:$GIT_DIR_BIN:/usr/bin:/bin"
# 本番の手順書と同じ実行形
RUNNER='git show refs/remotes/origin/main:scripts/apply-cfn-exec-app-policy.sh | bash -s -- "$@"'

# ケースごとの設定（run の後に既定へ戻す）
reset_case() { PRE_PUSH=""; MUTATE=""; POSTCHECK=""; ORIGIN_MODE=patched; RUN_SUBDIR=""; }
reset_case

# run <名前> <ok|stop> <create-policy-version を呼ぶか: yes|no> <期待するメッセージの一部（stop のとき）> <tty|notty> <答え> <FAKE_*=値...> -- <引数...>
run() {
  local name=$1 expect=$2 creates=$3 msg=$4 mode=$5 answer=$6; shift 6
  local envs=()
  while [ "$1" != "--" ]; do envs+=("$1"); shift; done
  shift
  make_repo
  if [ -n "$MUTATE" ]; then "$MUTATE"; fi
  : > "$AWS_LOG"; rm -f "$CREATED_DOC"
  local v
  for v in $FAKE_VARS; do unset "$v"; done
  local base=(env -i "PATH=$SAFE_PATH" "HOME=$work/home" "AWS_CONFIG_FILE=$work/aws_config" "AWS_SHARED_CREDENTIALS_FILE=$work/aws_credentials" AWS_EC2_METADATA_DISABLED=true "AWS_LOG=$AWS_LOG" "CREATED_DOC=$CREATED_DOC")
  # 実の aws に届かないことを、各ケースの前に確かめる
  local found
  found=$("${base[@]}" bash -c 'command -v aws') || { echo "致命的: aws が見つからない" >&2; exit 2; }
  [ "$found" = "$work/bin/aws" ] || { echo "致命的: aws が偽物を指していない: $found" >&2; exit 2; }
  local dir="$REPO_COPY${RUN_SUBDIR:+/$RUN_SUBDIR}"
  local out rc=0
  if [ "$mode" = tty ]; then
    out=$(cd "$dir" && python3 "$work/pty_run.py" "$answer" "${base[@]}" "${envs[@]+"${envs[@]}"}" bash -c "$RUNNER" _ "$@" 2>&1) || rc=$?
  else
    out=$(cd "$dir" && python3 "$work/notty_run.py" "${base[@]}" "${envs[@]+"${envs[@]}"}" bash -c "$RUNNER" _ "$@" 2>&1 < /dev/null) || rc=$?
  fi
  local ok=1 post=""
  if [ "$expect" = ok ] && [ "$rc" -ne 0 ]; then ok=0; fi
  if [ "$expect" = stop ] && [ "$rc" -eq 0 ]; then ok=0; fi
  if [ "$expect" = stop ] && ! printf '%s' "$out" | grep -qF -- "$msg"; then ok=0; fi
  if [ "$creates" = yes ] && ! grep -q 'iam create-policy-version' "$AWS_LOG"; then ok=0; fi
  if [ "$creates" = no ] && grep -q 'iam create-policy-version' "$AWS_LOG"; then ok=0; fi
  if [ -n "$POSTCHECK" ] && ! post=$("$POSTCHECK"); then ok=0; fi
  if [ "$ok" = 1 ]; then
    pass=$((pass + 1)); echo "ok: $name"
  else
    fail=$((fail + 1)); echo "FAILED: $name (rc=$rc)"; echo "$out" | sed 's/^/    /'; [ -z "$post" ] || echo "$post"
  fi
  reset_case
}

POSTCHECK=created_matches_origin
run "正常系: --apply（端末で y）は origin の main の JSON に ID を埋めて反映する" ok yes "" tty y -- --apply
run "正常系: dry-run は create-policy-version を呼ばず、差分を表示する" ok no "" notty "" --
run "正常系: Document が URL エンコードで返っても確認できる" ok yes "" tty y FAKE_GET_ENCODED=1 -- --apply
run "止まる: --apply で確認が n なら呼ばない" stop no "反映しなかった" tty n -- --apply
run "止まる: --apply で確認が空なら呼ばない" stop no "反映しなかった" tty "" -- --apply
run "止まる: 端末の無い --apply は拒否される" stop no "端末（対話）からだけ" notty "" -- --apply

# 作業ツリー・.git の改ざんは反映される内容を変えない
MUTATE=mutate_worktree; POSTCHECK=created_matches_origin
run "改ざん: 作業ツリーの JSON を変えても、反映されるのは origin の main" ok yes "" tty y -- --apply
MUTATE=mutate_assume_unchanged; POSTCHECK=created_matches_origin
run "改ざん: assume-unchanged で変更を隠しても、反映されるのは origin の main" ok yes "" tty y -- --apply
MUTATE=mutate_local_commit; POSTCHECK=created_matches_origin
run "改ざん: push していないコミットは反映されない" ok yes "" tty y -- --apply
MUTATE=mutate_update_ref; POSTCHECK=created_matches_origin
run "改ざん: update-ref で origin/main を偽装しても、fetch で正しい値に戻る" ok yes "" tty y -- --apply
MUTATE=mutate_tag_shadow; POSTCHECK=created_matches_origin
run "改ざん: ローカルのタグ origin/main は使われない" ok yes "" tty y -- --apply

# origin・場所・fetch
ORIGIN_MODE=real
run "止まる: origin の URL が GitHub の本リポジトリでない" stop no "origin の URL が想定と違う" tty y -- --apply
ORIGIN_MODE=insteadof
run "止まる: insteadOf で origin の取得先を書き換えている" stop no "origin の URL が想定と違う" tty y -- --apply
MUTATE=mutate_fetch_fails
run "止まる: git fetch に失敗" stop no "git fetch origin に失敗" tty y -- --apply
RUN_SUBDIR=scripts
run "止まる: リポジトリのルート以外から実行" stop no "リポジトリのルートで実行する" tty y -- --apply

# 置換後の文書の許可リスト
PRE_PUSH=pre_apis_wildcard
run "止まる: /apis/??????????/* を足した" stop no "apigateway の Resource が期待する集合と違う（Sid: ApiGatewayManageProjectHttpApi）" tty y -- --apply
PRE_PUSH=pre_tags_wildcard
run "止まる: /tags/* を足した" stop no "apigateway の Resource が期待する集合と違う（Sid: ApiGatewayTagOnCreateHttpApi）" tty y -- --apply
PRE_PUSH=pre_other_stage_id
run "止まる: ステージの Resource が他の ID" stop no "apigateway の Resource が期待する集合と違う（Sid: ApiGatewayManageProjectStage）" tty y -- --apply
PRE_PUSH=pre_new_apigw_sid
run "止まる: apigateway の文を足した（大文字の Action）" stop no "apigateway の文の Sid が期待する集合と違う" tty y -- --apply
PRE_PUSH=pre_not_resource
run "止まる: NotResource を含む" stop no "NotAction / NotResource / NotPrincipal がある（Sid: Extra）" tty y -- --apply
PRE_PUSH=pre_action_star
run "止まる: Action が *" stop no "Action に全体またはサービス単位のワイルドカードがある（Sid: Extra）" tty y -- --apply
PRE_PUSH=pre_service_star
run "止まる: Action に <service>:*" stop no "Action に全体またはサービス単位のワイルドカードがある（Sid: Extra）" tty y -- --apply

# AWS 側の確認
run "止まる: validate に SECURITY_WARNING" stop no "SECURITY_WARNING" tty y FAKE_SECURITY_WARNING=1 -- --apply
run "止まる: Api が 0 件" stop no "ちょうど 1 件ではない（0 件）" tty y FAKE_API_COUNT=0 -- --apply
run "止まる: Api が 2 件" stop no "ちょうど 1 件ではない（2 件）" tty y FAKE_API_COUNT=2 -- --apply
run "止まる: ID が *" stop no "API ID の形が違う" tty y 'FAKE_API_ID=*' -- --apply
run "止まる: ID が 11 文字" stop no "API ID の形が違う" tty y FAKE_API_ID=abcde123456 -- --apply
run "止まる: ID が大文字を含む" stop no "API ID の形が違う" tty y FAKE_API_ID=ABCDE12345 -- --apply
run "止まる: get-api の Name 違い" stop no "Name が" tty y FAKE_NAME=other-api -- --apply
run "止まる: Tags.Project 違い" stop no "Tags.Project が" tty y FAKE_TAG=other -- --apply
run "止まる: validate に別の ERROR" stop no "想定外の ERROR" tty y FAKE_VALIDATE_EXTRA=1 -- --apply
run "止まる: 版が 5 件" stop no "版が 5 つある" tty y FAKE_VERSIONS=5 -- --apply
run "止まる: 反映後の文書にプレースホルダーが残る" stop yes "プレースホルダーが残っている" tty y FAKE_GET_PLACEHOLDER=1 -- --apply
run "止まる: 不明な引数" stop no "usage" notty "" -- --force

echo "テスト: ${pass} 通過 / ${fail} 失敗"
[ "$fail" -eq 0 ]
