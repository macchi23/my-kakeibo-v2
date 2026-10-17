---
name: step
description: 方針が決まった 1 ステップ分を「implementer → verifier → レビュー → 差し戻し → verifier → コミット」で進める。計画の各ステップを実装するとき、ユーザーが /step を指示したときに使う。Go の本体をユーザーが書くステップには使わない。
---

方針と完了条件が決まっている 1 ステップを、CLAUDE.md の「標準の流れ」の 4〜8 に沿って進める。調査や設計判断が残っているなら先に `Explore` / `researcher` を使い、このスキルは使わない。
Go の本体をユーザーが書くステップ（学習の虫食い）は、このスキルではなく CLAUDE.md の「Go の学習」に従う。

## 1. implementer に渡す

- モデルは CLAUDE.md の「委譲」で決める
- ブリーフに書くこと: 目的 / 完了条件 / 対象ファイル / 仕様（判断済みの値まで）/ 既知の制約と除外済みの仮説 / 検証方法 / 返却形式
- 実機での確認（コマンドの実行、synth 結果）が完了条件なら、手順と期待値をブリーフに書く
- 同じファイルを触る implementer を並列に走らせない

## 2. verifier

- `scripts/check.sh all`。implementer の check 結果だけで済ませない

## 3. レビュー

- `code-reviewer` に仕様と diff の範囲だけを渡す。implementer の説明は渡さない
- 認証・認可、IAM、ユーザー入力の受け口、シークレットに触れたら `security-reviewer` も並列で
- 課金リソースの追加、外部 API の呼び出し経路、S3 イベントや Lambda の連鎖に触れたら `cost-reviewer` も並列で
- CLAUDE.md の「fable への格上げ」に当たるか、メインが判断する。上げたら報告に理由を 1 行書く
- verifier とレビューは並列に走らせてよい（レビューは読み取りのみ）

## 4. 差し戻し（最大 2 往復）

- 重大・中は必ず戻す。軽微は安く直せるなら戻す。数行の自明な修正ならメインが直してよい
- 2 往復目でも残る軽微な指摘は、報告に残してコミットへ進む
- 同じ指摘で 2 回差し戻したら implementer を opus に上げる

## 5. コミット前の確認

- implementer が「一時的に壊して戻した」と報告した変更は、`git diff` と実際のコマンド実行で戻っていることを確かめる
- 修正後に verifier をもう一度通す
- `git status --short` で、コミットしてはいけないファイル（`.env*`、`cdk.out/`、`.claude/logs/` など）が出ていないか確かめ、`scripts/check.sh secrets` を通す

## 6. コミット（ユーザーの指示があったとき）

- そのステップのファイルだけを `git add <path>` する（`git add -A` は使わない。他ステップの未コミット変更を混ぜない）
- メッセージは日本語で 1 行目に `<種別>: <内容>`（feat / fix / docs / refactor / test / chore）、本文に理由を 2〜3 行
- ドキュメント（`notes/handover.md`、DECISIONS.md）の更新は、最後のステップか別のドキュメント用コミットにまとめる

## 6.5 AWS への反映（infra/ を変えたステップ）

- `deploy-reviewer` に、そのステップの仕様（何を変える deploy か）だけを渡す。判定が「deploy 可」なら、メインが `npx cdk deploy --profile my-kakeibo-v2` を実行する
- 「先にユーザーの作業が要る」（実行ロールのポリシーの更新など）なら、コマンドを示して止まる。「deploy 不可」なら差し戻しに戻る
- 広い権限（`default` と共用の SSO プロファイル）が要る操作はユーザーが行う（CLAUDE.md の必須ルール）

## 7. 報告と台帳

- ユーザーには、コミット ID、check の結果、レビュー結果（重大の有無と、対応した指摘、fable に上げたならその理由）、ブリーフから外れた判断を短く伝える
- サブエージェントの「ハーネスへの気づき」は `notes/harness-backlog.md` に転記する
