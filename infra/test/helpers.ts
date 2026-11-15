import { readFileSync } from 'node:fs';

const cdkJson = JSON.parse(readFileSync(new URL('../cdk.json', import.meta.url), 'utf-8'));

// cdk.json の context（feature flags）に、Go のビルドを走らせない指定を足したもの。createApp を呼ぶテストは全部これを使う
export function testContext(extra: Record<string, unknown> = {}): Record<string, unknown> {
  return { ...cdkJson.context, 'aws:cdk:bundling-stacks': [], ...extra };
}
