export const PROJECT_TAG_KEY = 'Project';
export const PROJECT_TAG_VALUE = 'my-kakeibo-v2';
export const BOOTSTRAP_QUALIFIER = 'kakeibov2';
export const TOOLKIT_STACK_NAME = 'CDKToolkit-my-kakeibo-v2';
export const STACK_NAME = 'MyKakeiboV2';
// Budget の通知先メールアドレスを入れる SSM パラメータ（String）。デプロイ時に解決する
export const BUDGET_EMAIL_PARAMETER = '/my-kakeibo-v2/budget-email';
// CFN 実行ロールのポリシーが名前の接頭辞で絞るので名前を固定する。名前と通知設定はどちらも createOnly で、
// 同名のまま通知を変えると置換時に名前が重複して失敗する。通知先・しきい値を変えるときは末尾の版を上げる
export const BUDGET_NAME = 'my-kakeibo-v2-monthly-v1';
// IAM ロールの path。実行ロールのポリシーが path で CreateRole を絞るため、全ロールをこの配下に置く
export const ROLE_PATH = '/my-kakeibo-v2/';
// 全ロールに付ける permissions boundary の名前。cdk.json の context と一致させる（名前のみ。アカウントは CDK が解決する）
export const PERMISSIONS_BOUNDARY_NAME = 'my-kakeibo-v2-boundary';
// CFN 実行ロールのポリシーが名前の接頭辞で絞るので、API 関連の名前は明示する。
// テーブルはキー設計を変えると置換になり RETAIN で名前が残るため、作り直すときは末尾の版を上げる
export const TABLE_NAME = 'my-kakeibo-v2-data-v1';
export const FUNCTION_NAME = 'my-kakeibo-v2-api';
export const LOG_GROUP_NAME = '/aws/lambda/my-kakeibo-v2-api';
export const HTTP_API_NAME = 'my-kakeibo-v2-api';
// deploy を 2 回に分けるための context。false のとき HTTP API を作らない
export const WITH_HTTP_API_CONTEXT_KEY = 'kakeibo:withHttpApi';
// 既定ルートの設定が「ルートごと」に効くか一次資料で確かめられなかったため、ルートごとと仮定して絞る（12 ルートで最大 24 rps）
export const THROTTLING_RATE_LIMIT = 2;
export const THROTTLING_BURST_LIMIT = 10;
