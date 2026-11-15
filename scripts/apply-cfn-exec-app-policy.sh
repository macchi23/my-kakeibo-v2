#!/usr/bin/env bash
# cfn-exec-app ポリシーに自 API の ID を埋めて反映する。ユーザーが `default`（管理者）で実行する（Claude は実行しない）
# 作業ツリーのこのファイルは実行しない。GitHub の main に push 済みの版を、リポジトリのルートで次の形で実行する（docs/setup.md）:
#   git fetch origin && git show refs/remotes/origin/main:scripts/apply-cfn-exec-app-policy.sh | bash -s              確認だけ（dry-run）
#   git fetch origin && git show refs/remotes/origin/main:scripts/apply-cfn-exec-app-policy.sh | bash -s -- --apply   確認に通ったら反映
# なぜ: 作業ツリーと .git の参照は、Claude を含む書き込める者が改ざんできる。HEAD との一致を見る検査は
#   `update-index --assume-unchanged` で、push 済みの検査は `update-ref refs/remotes/origin/main` ですり抜けられた。
#   そこで、反映する JSON もこのスクリプトが自分で fetch した GitHub の main から取り、作業ツリーのファイルは読まない
# `origin/main` ではなく `refs/remotes/origin/main` と書くのは、ローカルにタグやブランチ `origin/main` を作られるとそちらが優先されるため
# `bash -s` では標準入力がスクリプト本体なので、端末の判定と [y/N] の確認は /dev/tty で行う
#
# ID はスタックの Output ではなく、スタックのリソース（AWS::ApiGatewayV2::Api の PhysicalResourceId）から取る。
# Output はテンプレートで任意の値にできるため、`*` や他 PJ の ID を埋めさせられるおそれがある。
# プロファイル・リージョン・スタック名・origin の URL は固定で、引数や環境変数では変えられない。
set -euo pipefail

# 固定値。引数や環境変数では変えられないようにして、別のアカウント・スタックに向けられないようにしている。
PROFILE=default
REGION=us-east-1
STACK_NAME=MyKakeiboV2
POLICY_NAME=my-kakeibo-v2-cfn-exec-app
POLICY_PATH=infra/iam/cfn-exec-app-policy.json
PLACEHOLDER=__HTTP_API_ID__
API_NAME=my-kakeibo-v2-api
PROJECT_TAG=my-kakeibo-v2
REMOTE_REF=refs/remotes/origin/main
# origin として認める URL（insteadOf を展開した後の値で比べる）。テストはこの 1 行だけを差し替えたコピーで動かす
ORIGIN_URLS="https://github.com/macchi23/my-kakeibo-v2.git https://github.com/macchi23/my-kakeibo-v2 git@github.com:macchi23/my-kakeibo-v2.git git@github.com:macchi23/my-kakeibo-v2 ssh://git@github.com/macchi23/my-kakeibo-v2.git ssh://git@github.com/macchi23/my-kakeibo-v2"
USAGE="usage: git show ${REMOTE_REF}:scripts/apply-cfn-exec-app-policy.sh | bash -s [-- --apply]"

die() { echo "中止: $*" >&2; exit 1; }

# .git/config で設定されたフック・fsmonitor（任意のコマンドを実行できる）を、このスクリプトの git では使わない
g() { git -c core.hooksPath=/dev/null -c core.fsmonitor=false "$@"; }

awsq() { aws --profile "$PROFILE" --region "$REGION" --output json "$@"; }

# doc_to_file <get-policy-version の JSON> <出力ファイル>
# AWS CLI は Document を JSON に復号して返す。復号されない（文字列で返る）場合に備えて URL デコードも扱う
doc_to_file() {
  local encoded
  encoded=$(printf '%s' "$1" | jq -r '.PolicyVersion.Document | if type == "string" then . else empty end')
  if [ -n "$encoded" ]; then
    printf '%b' "$(printf '%s' "$encoded" | sed 's/%\([0-9A-Fa-f][0-9A-Fa-f]\)/\\x\1/g')" > "$2"
  else
    printf '%s' "$1" | jq '.PolicyVersion.Document' > "$2"
  fi
}

# 置換後の文書の検査。問題があれば理由を 1 行ずつ出す（無ければ何も出さない）
#   - どの文にも NotAction / NotResource / NotPrincipal が無い
#   - Deny 以外の文の Action に `*` だけ、`<service>:*`、サービス部分のワイルドカード、`:` の無いものが無い
#   - apigateway の操作を含む Deny 以外の文は、Sid と Resource の集合が下の期待値と完全に一致する（足すことも減らすことも許さない）
policy_problems() {  # policy_problems <文書> <API ID>
  local p="arn:aws:apigateway:${REGION}::"
  jq -r --arg p "$p" --arg r "$REGION" --arg id "$2" '
    def arr: if type == "array" then . else [.] end;
    ($p + "/apis/??????????") as $h
    | {
        "ApiGatewayCreateTaggedHttpApi": [$p + "/apis"],
        "ApiGatewayTagOnCreateHttpApi": [$p + "/tags/arn%3Aaws%3Aapigateway%3A" + $r + "%3A%3A%2Fv2%2Fapis%2F${*}"],
        "ApiGatewayTagProjectStage": [$p + "/apis/" + $id + "/stages", $p + "/apis/" + $id + "/stages/*"],
        "ApiGatewayManageProjectStage": [$p + "/apis/" + $id + "/stages", $p + "/apis/" + $id + "/stages/*"],
        "ApiGatewayManageProjectHttpApi":
          ([$h] + ([("routes", "integrations", "authorizers", "deployments") | ($h + "/" + .), ($h + "/" + . + "/*")]))
      } as $want
    | (.Statement | arr) as $st
    | ($st[]
       | select(has("NotAction") or has("NotResource") or has("NotPrincipal"))
       | "NotAction / NotResource / NotPrincipal がある（Sid: \(.Sid // "-")）"),
      ($st[]
       | select(.Effect != "Deny")
       | select(.Action | arr | any(type != "string" or . == "*" or (contains(":") | not)
                                   or (split(":")[0] | test("[*?]")) or (split(":")[1] == "*")))
       | "Action に全体またはサービス単位のワイルドカードがある（Sid: \(.Sid // "-")）"),
      ([$st[] | select(.Effect != "Deny")
        | select(.Action | arr | any(type == "string" and (ascii_downcase | startswith("apigateway:"))))] as $got
       | if ($got | map(.Sid // "") | sort) != ($want | keys | sort) then
           "apigateway の文の Sid が期待する集合と違う: \($got | map(.Sid // "-") | sort | join(","))"
         else
           ($got[] | select((.Resource | arr | sort) != ($want[.Sid] | sort))
            | "apigateway の Resource が期待する集合と違う（Sid: \(.Sid)）: \(.Resource | arr | join(" "))")
         end)
  ' "$1"
}

# 全体の流れ: 引数の確認 → origin の確認と main の取得 → スタックから API ID を取得 → 実在の API の確認 →
# ID を埋めて検査 → validate-policy → 版の上限 → 差分表示 → （--apply のとき）確認して反映。
main() {
  local apply=0
  case "${1:-}" in
    "") ;;
    --apply) apply=1 ;;
    *) echo "$USAGE" >&2; exit 2 ;;
  esac
  [ $# -le 1 ] || { echo "$USAGE" >&2; exit 2; }

  # 非対話では反映させない。bash -s では標準入力が端末にならないので、制御端末（/dev/tty）が開けるかで判定する
  if [ "$apply" = 1 ] && ! { : < /dev/tty; } 2>/dev/null; then
    die "--apply は端末（対話）からだけ実行できる（/dev/tty を開けない）"
  fi

  command -v jq >/dev/null 2>&1 || die "jq が見つからない"
  command -v git >/dev/null 2>&1 || die "git が見つからない"

  # 1. リポジトリのルートで、origin が GitHub の本リポジトリであること
  local top
  top=$(g rev-parse --show-toplevel 2>/dev/null) || die "git のリポジトリではない（リポジトリのルートで実行する）"
  [ "$(cd "$top" && pwd -P)" = "$(pwd -P)" ] || die "リポジトリのルートで実行する（今の場所: $(pwd -P)）"
  local url ok=0 u
  # get-url は url.<base>.insteadOf を展開した値を返すので、書き換えによる差し替えもここで止まる
  url=$(g remote get-url origin 2>/dev/null) || die "origin が無い"
  for u in $ORIGIN_URLS; do [ "$url" = "$u" ] && ok=1; done
  [ "$ok" = 1 ] || die "origin の URL が想定と違う: ${url}"

  # 2. GitHub の main を取り直す。ローカルの update-ref による偽装は、強制更新（+）で上書きされる。
  #    refspec を明示するのは、.git/config の remote.origin.fetch を書き換えられても、取る先と書く先を変えられないようにするため
  g fetch --quiet --no-tags origin "+refs/heads/main:${REMOTE_REF}" || die "git fetch origin に失敗（ネットワークか認証を確かめて再実行する）"
  local commit
  commit=$(g rev-parse --verify -q "${REMOTE_REF}^{commit}") || die "${REMOTE_REF} を解決できない"
  echo "反映元: GitHub の main $(g log -1 --format='%h %s' "$commit")"

  local src tmp cur_file new_file doc_file
  src=$(mktemp); tmp=$(mktemp); cur_file=$(mktemp); new_file=$(mktemp); doc_file=$(mktemp)
  # shellcheck disable=SC2064  # 一時ファイルの名前はここで確定させる
  trap "rm -f '$src' '$tmp' '$cur_file' '$new_file' '$doc_file'" EXIT
  # 以降の読み取りは、取り直した時点のコミット（ハッシュ）に固定する
  g cat-file blob "${commit}:${POLICY_PATH}" > "$src" || die "${POLICY_PATH} が GitHub の main に無い"

  local account policy_arn
  account=$(awsq sts get-caller-identity | jq -r '.Account') || die "sts get-caller-identity に失敗"
  printf '%s\n' "$account" | grep -Eqx '[0-9]{12}' || die "アカウント ID の形が違う"
  policy_arn="arn:aws:iam::${account}:policy/${POLICY_NAME}"

  # 3. ID はスタックのリソースから取る（Output は使わない）
  local resources n_api api_id api
  resources=$(awsq cloudformation describe-stack-resources --stack-name "$STACK_NAME") \
    || die "describe-stack-resources に失敗（スタック $STACK_NAME が無い、または権限が無い）"
  n_api=$(printf '%s' "$resources" | jq '[.StackResources[] | select(.ResourceType == "AWS::ApiGatewayV2::Api")] | length')
  [ "$n_api" = 1 ] || die "AWS::ApiGatewayV2::Api がちょうど 1 件ではない（${n_api} 件）。API の作り直しの手順は docs/setup.md"
  api_id=$(printf '%s' "$resources" | jq -r '.StackResources[] | select(.ResourceType == "AWS::ApiGatewayV2::Api") | .PhysicalResourceId')
  printf '%s\n' "$api_id" | grep -Eqx '[a-z0-9]{10}' || die "API ID の形が違う（10 文字の英小文字と数字ではない）: ${api_id}"

  # 4. 実在の API が自 PJ のものか
  api=$(awsq apigatewayv2 get-api --api-id "$api_id") || die "get-api に失敗"
  [ "$(printf '%s' "$api" | jq -r '.Name // ""')" = "$API_NAME" ] || die "API の Name が $API_NAME ではない"
  [ "$(printf '%s' "$api" | jq -r '.Tags.Project // ""')" = "$PROJECT_TAG" ] || die "API の Tags.Project が $PROJECT_TAG ではない"

  # 5. 埋めて、許可リストと照らす
  sed "s/${PLACEHOLDER}/${api_id}/g" "$src" > "$tmp"
  ! grep -q "$PLACEHOLDER" "$tmp" || die "プレースホルダーが残っている"
  local problems
  problems=$(policy_problems "$tmp" "$api_id") || die "ポリシーの文書を JSON として検査できない"
  [ -z "$problems" ] || die "ポリシーの検査に通らない: ${problems}"

  # 6. validate-policy。既知の ERROR（apigateway:TagResource の INVALID_ACTION）以外があれば止める
  local findings unexpected
  findings=$(awsq accessanalyzer validate-policy --policy-type IDENTITY_POLICY --policy-document "file://${tmp}") \
    || die "validate-policy に失敗"
  echo "validate-policy の findings（全件）:"
  printf '%s' "$findings" | jq -r '(.findings // [])[] | "  \(.findingType) \(.issueCode): \(.findingDetails // "")"'
  [ "$(printf '%s' "$findings" | jq '[(.findings // [])[] | select(.findingType == "SECURITY_WARNING")] | length')" = 0 ] \
    || die "validate-policy に SECURITY_WARNING がある"
  unexpected=$(printf '%s' "$findings" | jq -c '
    [(.findings // [])[]
     | select(.findingType == "ERROR")
     | select((.issueCode == "INVALID_ACTION" and ((.findingDetails // "") | contains("apigateway:TagResource"))) | not)
    ]')
  [ "$(printf '%s' "$unexpected" | jq 'length')" = 0 ] || die "validate-policy に想定外の ERROR がある: ${unexpected}"

  # 7. 版の上限（5）。消さずに案内して止める
  local versions oldest
  versions=$(awsq iam list-policy-versions --policy-arn "$policy_arn") \
    || die "list-policy-versions に失敗（ポリシー $POLICY_NAME が無い場合は docs/setup.md の「cfn-exec-app の反映」の初回の手順）"
  if [ "$(printf '%s' "$versions" | jq '.Versions | length')" -ge 5 ]; then
    oldest=$(printf '%s' "$versions" | jq -r '[.Versions[] | select(.IsDefaultVersion | not)] | sort_by(.CreateDate) | .[0].VersionId')
    die "版が 5 つある。既定でない最古の版を消してから再実行する: aws iam delete-policy-version --profile ${PROFILE} --policy-arn ${policy_arn} --version-id ${oldest}"
  fi

  # 8. 現在の既定の版との差分
  local cur_id cur
  cur_id=$(printf '%s' "$versions" | jq -r '[.Versions[] | select(.IsDefaultVersion)] | .[0].VersionId // ""')
  [ -n "$cur_id" ] || die "既定の版が見つからない"
  cur=$(awsq iam get-policy-version --policy-arn "$policy_arn" --version-id "$cur_id") || die "現在の既定の版の get-policy-version に失敗"
  doc_to_file "$cur" "$doc_file"
  jq -S . "$doc_file" > "$cur_file" || die "現在の既定の版の文書を JSON として読めない"
  jq -S . "$tmp" > "$new_file"
  echo "現在の既定の版（${cur_id}）との差分:"
  diff -u --label "現在の既定の版 ${cur_id}" --label "反映する内容" "$cur_file" "$new_file" || true

  echo "確認 OK: API ID ${api_id}（Name=${API_NAME}, Project=${PROJECT_TAG}）、${policy_arn}"
  if [ "$apply" != 1 ]; then
    echo "dry-run のため反映していない。反映するには --apply を付ける"
    exit 0
  fi
  local answer
  printf '%s' "この内容で反映しますか [y/N] " > /dev/tty
  read -r answer < /dev/tty || die "確認の入力を読めない"
  [ "$answer" = y ] || die "反映しなかった（y 以外が入力された）"

  # 9. 反映と、反映後の文書が送った内容と一致することの確認
  local created version_id fetched
  created=$(awsq iam create-policy-version --set-as-default --policy-arn "$policy_arn" --policy-document "file://${tmp}") \
    || die "create-policy-version に失敗"
  version_id=$(printf '%s' "$created" | jq -r '.PolicyVersion.VersionId')
  fetched=$(awsq iam get-policy-version --policy-arn "$policy_arn" --version-id "$version_id") \
    || die "get-policy-version に失敗（反映はされた。版 ${version_id} を確認する）"
  doc_to_file "$fetched" "$doc_file"
  ! grep -q "$PLACEHOLDER" "$doc_file" || die "反映後の文書にプレースホルダーが残っている（版 ${version_id}）"
  jq -S . "$doc_file" > "$cur_file" || die "反映後の文書を JSON として読めない（版 ${version_id}）"
  cmp -s "$cur_file" "$new_file" || die "反映後の文書が送った内容と違う（版 ${version_id}）"
  echo "反映した: 版 ${version_id}（既定）"
}

# 関数の定義を読み切ってから実行する。bash -s では標準入力がスクリプト本体なので、
# 子のコマンドが残りを読み込まないよう、標準入力を /dev/null に替えて呼ぶ
main "$@" < /dev/null
