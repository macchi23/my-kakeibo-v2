import { createApp } from '../lib/app';

// CDK のエントリポイント。cdk コマンドがこのファイルを実行して、スタックの定義（CloudFormation テンプレート）を作る。
// 中身は lib/app.ts にあり、テストからも同じ関数を呼べるようにしている。
createApp();
