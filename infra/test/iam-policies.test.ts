import { readFileSync } from 'node:fs';
import { Template } from 'aws-cdk-lib/assertions';
import { describe, expect, it } from 'vitest';
import { createApp } from '../lib/app';
import {
  BOOTSTRAP_QUALIFIER,
  BUDGET_EMAIL_PARAMETER,
  BUDGET_NAME,
  PROJECT_TAG_VALUE,
  STACK_NAME,
  TOOLKIT_STACK_NAME,
} from '../lib/project-constants';

const read = (name: string) => readFileSync(new URL(`../iam/${name}`, import.meta.url), 'utf-8');
const files = ['cfn-exec-policy.json', 'permission-set-inline.json'] as const;
const raw: Record<string, string> = Object.fromEntries(files.map((f) => [f, read(f)]));
const docs: Record<string, { Version: string; Statement: Statement[] }> = Object.fromEntries(
  files.map((f) => [f, JSON.parse(raw[f])]),
);

interface Statement {
  Sid: string;
  Effect: 'Allow' | 'Deny';
  Action: string | string[];
  Resource: string | string[];
  Condition?: Record<string, Record<string, string>>;
}

const list = (v: string | string[]) => (Array.isArray(v) ? v : [v]);
const resourcesOf = (f: string) => docs[f].Statement.flatMap((s) => list(s.Resource));
const conditionValues = (f: string) =>
  docs[f].Statement.flatMap((s) => Object.values(s.Condition ?? {}).flatMap((c) => Object.values(c)));

describe('本番スタックのリソース型', () => {
  it('cfn-exec-policy.json が許可しているリソース型だけを使う', () => {
    const { stack } = createApp({ context: JSON.parse(readFileSync(new URL('../cdk.json', import.meta.url), 'utf-8')).context });
    const types = new Set(Object.values(Template.fromStack(stack).toJSON().Resources).map((r) => (r as { Type: string }).Type));
    const allowed = new Set(['AWS::Cognito::UserPool', 'AWS::Cognito::UserPoolClient', 'AWS::Budgets::Budget', 'AWS::CDK::Metadata']);
    const unexpected = [...types].filter((t) => !allowed.has(t));
    expect(
      unexpected,
      `新しいリソース型を足したら infra/iam/cfn-exec-policy.json を更新する（未対応: ${unexpected.join(', ')}）`,
    ).toEqual([]);
  });
});

describe.each(files)('infra/iam/%s', (f) => {
  it('JSON として読め、Version が 2012-10-17', () => {
    expect(docs[f].Version).toBe('2012-10-17');
    expect(docs[f].Statement.length).toBeGreaterThan(0);
  });

  it('12 桁の数字（アカウント ID）を含まない', () => {
    expect(raw[f]).not.toMatch(/\d{12}/);
  });

  it('Allow に Action "*" / "<service>:*" が無い', () => {
    for (const s of docs[f].Statement.filter((x) => x.Effect === 'Allow')) {
      for (const a of list(s.Action)) {
        expect(a, `${s.Sid}: ${a}`).not.toMatch(/^\*$|:\*$/);
      }
    }
  });

  it('Sid が重複しない', () => {
    const sids = docs[f].Statement.map((s) => s.Sid);
    expect(new Set(sids).size).toBe(sids.length);
  });
});

describe('permission-set-inline.json', () => {
  it('deploy-role / lookup-role を引き受けさせない', () => {
    expect(raw['permission-set-inline.json']).not.toContain('deploy-role');
    expect(raw['permission-set-inline.json']).not.toContain('lookup-role');
  });
});

describe.each(files)('定数との整合: %s', (f) => {
  it('ARN 中の bootstrap qualifier が BOOTSTRAP_QUALIFIER', () => {
    const quals = new Set<string>();
    for (const v of [...resourcesOf(f), ...conditionValues(f)]) {
      const m = v.match(/(?:cdk-|cdk-bootstrap\/)([a-z0-9]+)(?:-[a-z-]+-role|-assets|\/version)/);
      if (m) quals.add(m[1]);
    }
    expect([...quals]).toEqual([BOOTSTRAP_QUALIFIER]);
  });

  it('スタック名・toolkit スタック名が定数と一致する', () => {
    const stackNames = new Set<string>();
    for (const v of [...resourcesOf(f), ...conditionValues(f)]) {
      const m = v.match(/:stack\/([^/]+)\//);
      if (m) stackNames.add(m[1]);
    }
    const expected = f === 'permission-set-inline.json' ? [STACK_NAME, TOOLKIT_STACK_NAME] : [];
    expect([...stackNames].sort()).toEqual(expected.sort());
  });

  it('Budget の ARN 接頭辞が BUDGET_NAME の接頭辞', () => {
    const prefixes = resourcesOf(f)
      .map((r) => r.match(/:budget\/(.+)\*$/)?.[1])
      .filter((p): p is string => p !== undefined);
    expect(prefixes.length).toBeGreaterThan(0);
    for (const p of prefixes) expect(BUDGET_NAME.startsWith(p)).toBe(true);
  });

  it('タグ条件の値が PROJECT_TAG_VALUE', () => {
    for (const s of docs[f].Statement) {
      for (const [k, v] of Object.entries(s.Condition?.StringEquals ?? {})) {
        if (k.includes('Tag/')) expect(v).toBe(PROJECT_TAG_VALUE);
      }
    }
  });
});

describe('cfn-exec-policy.json の SSM', () => {
  it('budget-email のパラメータ名が BUDGET_EMAIL_PARAMETER', () => {
    const arns = resourcesOf('cfn-exec-policy.json').filter((r) => r.includes(':parameter/my-kakeibo-v2/'));
    expect(arns.map((a) => a.split(':parameter')[1])).toEqual([BUDGET_EMAIL_PARAMETER]);
  });
});
