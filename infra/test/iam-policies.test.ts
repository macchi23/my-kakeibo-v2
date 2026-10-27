import { readFileSync } from 'node:fs';
import { Template } from 'aws-cdk-lib/assertions';
import { describe, expect, it } from 'vitest';
import { createApp } from '../lib/app';
import {
  BOOTSTRAP_QUALIFIER,
  BUDGET_EMAIL_PARAMETER,
  BUDGET_NAME,
  PERMISSIONS_BOUNDARY_NAME,
  PROJECT_TAG_VALUE,
  ROLE_PATH,
  STACK_NAME,
  TOOLKIT_STACK_NAME,
} from '../lib/project-constants';

const read = (name: string) => readFileSync(new URL(`../iam/${name}`, import.meta.url), 'utf-8');
const files = ['cfn-exec-policy.json', 'cfn-exec-app-policy.json', 'boundary-policy.json', 'permission-set-inline.json'] as const;
const managed = ['cfn-exec-policy.json', 'cfn-exec-app-policy.json', 'boundary-policy.json'] as const;
const execPolicies = ['cfn-exec-policy.json', 'cfn-exec-app-policy.json'] as const;
const raw: Record<string, string> = Object.fromEntries(files.map((f) => [f, read(f)]));
const docs: Record<string, { Version: string; Statement: Statement[] }> = Object.fromEntries(
  files.map((f) => [f, JSON.parse(raw[f])]),
);

interface Statement {
  Sid: string;
  Effect: 'Allow' | 'Deny';
  Action: string | string[];
  Resource: string | string[];
  Condition?: Record<string, Record<string, string | string[]>>;
}

const list = (v: string | string[]) => (Array.isArray(v) ? v : [v]);
const resourcesOf = (f: string) => docs[f].Statement.flatMap((s) => list(s.Resource));
const conditionValues = (f: string) =>
  docs[f].Statement.flatMap((s) => Object.values(s.Condition ?? {}).flatMap((c) => Object.values(c).flatMap(list)));
const allowed = (f: string) => docs[f].Statement.filter((s) => s.Effect === 'Allow');
const allActions = (f: string, effect: 'Allow' | 'Deny') =>
  docs[f].Statement.filter((s) => s.Effect === effect).flatMap((s) => list(s.Action));
const stripSpaces = (t: string) => t.replace(/\s/g, '');
const globToRegExp = (g: string) => new RegExp(`^${g.replace(/[.+?^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*')}$`, 'i');
const grants = (f: string, action: string) => allActions(f, 'Allow').filter((g) => globToRegExp(g).test(action));
const ROLE_ARN = `arn:aws:iam::*:role${ROLE_PATH}*`;
const BOUNDARY_ARN = `arn:aws:iam::\${aws:PrincipalAccount}:policy/${PERMISSIONS_BOUNDARY_NAME}`;
const PROJECT_MARKER = /my-kakeibo-v2|kakeibov2|MyKakeiboV2/;
const isTagValue = (s: Statement, key: string) => list(s.Condition?.StringEquals?.[key] ?? []).join() === PROJECT_TAG_VALUE;
const tagging = (s: Statement) => list(s.Action).some((a) => /Tag/.test(a)) || list(s.Resource).some((r) => r.includes('/tags/'));
const creating = (s: Statement) =>
  list(s.Action).every((a) => /^[a-z0-9-]+:Create/.test(a)) ||
  (list(s.Action).every((a) => a === 'apigateway:POST') && list(s.Resource).every((r) => r.endsWith('/apis')));
const namedForProject = (s: Statement) => list(s.Resource).every((r) => r !== '*' && PROJECT_MARKER.test(r));
// タグ条件は StringEquals で値が PROJECT_TAG_VALUE のものだけを認める。
// RequestTag は作成系だけ。タグ付け系は ResourceTag かプロジェクト名の Resource を必須にする（RequestTag は呼び出し側が自由に付けられるため）
const scopedToProject = (s: Statement) => {
  const resTag = isTagValue(s, 'aws:ResourceTag/Project');
  if (namedForProject(s)) return true;
  if (tagging(s)) return resTag;
  return creating(s) ? resTag || isTagValue(s, 'aws:RequestTag/Project') : resTag;
};
const hasProjectTagCondition = (s: Statement) => isTagValue(s, 'aws:ResourceTag/Project') || isTagValue(s, 'aws:RequestTag/Project');

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

describe('管理ポリシー', () => {
  it.each(managed)('%s は空白を除いて 6,144 文字以下（管理ポリシーの上限）', (f) => {
    expect(stripSpaces(raw[f]).length).toBeLessThanOrEqual(6144);
  });

  it('us-east-1 以外を Deny する Statement が cfn-exec 系 2 本と権限セットの先頭にある', () => {
    for (const f of [...execPolicies, 'permission-set-inline.json'] as const) {
      const s = docs[f].Statement[0];
      expect(s.Sid).toBe('DenyOutsideUsEast1');
      expect(s.Condition).toEqual({ StringNotEquals: { 'aws:RequestedRegion': 'us-east-1' } });
    }
  });
});

describe('Resource "*" の Allow は許可リストだけ', () => {
  const wildcardOk: Record<string, string[]> = {
    'cfn-exec-policy.json': ['CognitoCreateTaggedUserPool'],
    'cfn-exec-app-policy.json': ['LogsDescribeWithoutResourceLevel'],
    'boundary-policy.json': [],
    'permission-set-inline.json': ['FindTaggedResources'],
  };
  it.each(files)('%s', (f) => {
    const sids = allowed(f)
      .filter((s) => list(s.Resource).includes('*'))
      .map((s) => s.Sid);
    expect(sids.sort()).toEqual([...wildcardOk[f]].sort());
  });
});

describe('scopedToProject の判定', () => {
  const base = { Sid: 'x', Effect: 'Allow' as const };
  it.each([
    ['/tags/* に RequestTag だけ', { Action: 'apigateway:PUT', Resource: 'arn:aws:apigateway:us-east-1::/tags/*', Condition: { StringEquals: { 'aws:RequestTag/Project': PROJECT_TAG_VALUE } } }],
    ['StringLike で値が *', { Action: 'apigateway:GET', Resource: 'arn:aws:apigateway:us-east-1::/apis/*', Condition: { StringLike: { 'aws:ResourceTag/Project': '*' } } }],
    ['StringNotEquals', { Action: 'apigateway:GET', Resource: 'arn:aws:apigateway:us-east-1::/apis/*', Condition: { StringNotEquals: { 'aws:ResourceTag/Project': PROJECT_TAG_VALUE } } }],
    ['条件なし', { Action: 'apigateway:GET', Resource: 'arn:aws:apigateway:us-east-1::/apis/*/routes*' }],
    ['別の値', { Action: 'apigateway:GET', Resource: 'arn:aws:apigateway:us-east-1::/apis/*', Condition: { StringEquals: { 'aws:ResourceTag/Project': 'other' } } }],
  ])('拒否する: %s', (_n, partial) => {
    expect(scopedToProject({ ...base, ...partial } as Statement)).toBe(false);
  });
  it('作成系の RequestTag と、ResourceTag は通す', () => {
    const create = { Action: 'apigateway:POST', Resource: 'arn:aws:apigateway:us-east-1::/apis', Condition: { StringEquals: { 'aws:RequestTag/Project': PROJECT_TAG_VALUE } } };
    const manage = { Action: 'apigateway:GET', Resource: 'arn:aws:apigateway:us-east-1::/apis/*', Condition: { StringEquals: { 'aws:ResourceTag/Project': PROJECT_TAG_VALUE } } };
    expect(scopedToProject({ ...base, ...create } as Statement)).toBe(true);
    expect(scopedToProject({ ...base, ...manage } as Statement)).toBe(true);
  });
});

describe('Allow の Resource がプロジェクトのものに限られる', () => {
  // 請求ビューは名前を持たず、Budget のハンドラーが要求する
  const noProjectResource = new Set(['BudgetsReadBillingView']);
  it.each(files)('%s: Resource にプロジェクト固有の名前があるか、プロジェクトのタグ条件が付く', (f) => {
    for (const s of allowed(f).filter((x) => !noProjectResource.has(x.Sid) && !list(x.Resource).includes('*'))) {
      expect(scopedToProject(s), s.Sid).toBe(true);
    }
  });

  it.each(files)('%s: apigateway の Allow にはプロジェクトのタグ条件が必ずある', (f) => {
    for (const s of allowed(f).filter((x) => list(x.Action).some((a) => a.startsWith('apigateway:')))) {
      expect(hasProjectTagCondition(s) && scopedToProject(s), s.Sid).toBe(true);
    }
  });
});

describe('IAM の囲い', () => {
  const doc = 'cfn-exec-policy.json';
  const stmt = (sid: string) => docs[doc].Statement.find((s) => s.Sid === sid);

  it('boundary 無しの CreateRole / boundary 外し / ポリシー書き換えを Deny する', () => {
    expect(list(stmt('DenyRoleWithoutProjectBoundary')?.Action ?? [])).toEqual(
      expect.arrayContaining(['iam:CreateRole', 'iam:PutRolePermissionsBoundary']),
    );
    expect(stmt('DenyRoleWithoutProjectBoundary')?.Resource).toBe('*');
    expect(stmt('DenyRoleWithoutProjectBoundary')?.Condition).toEqual({ ArnNotEquals: { 'iam:PermissionsBoundary': BOUNDARY_ARN } });
    expect(stmt('DenyBoundaryRemoval')?.Action).toBe('iam:DeleteRolePermissionsBoundary');
    const edit = stmt('DenyProjectPolicyEdit');
    expect(edit?.Effect).toBe('Deny');
    expect(list(edit?.Resource ?? [])).toEqual(
      expect.arrayContaining([
        'arn:aws:iam::*:policy/my-kakeibo-v2-boundary',
        'arn:aws:iam::*:policy/my-kakeibo-v2-cfn-exec',
        'arn:aws:iam::*:policy/my-kakeibo-v2-cfn-exec-app',
      ]),
    );
  });

  it('ロールの操作は path 配下に限り、boundary 条件が付く', () => {
    const roleMutating = [
      'iam:CreateRole', 'iam:DeleteRole', 'iam:UpdateRole', 'iam:UpdateRoleDescription', 'iam:UpdateAssumeRolePolicy',
      'iam:PutRolePolicy', 'iam:DeleteRolePolicy', 'iam:AttachRolePolicy', 'iam:DetachRolePolicy', 'iam:PutRolePermissionsBoundary',
    ];
    const roleStmts = allowed(doc).filter((s) => list(s.Action).some((g) => roleMutating.some((a) => globToRegExp(g).test(a))));
    expect(roleStmts.length).toBeGreaterThan(0);
    for (const s of roleStmts) {
      expect(s.Resource, s.Sid).toBe(ROLE_ARN);
      expect(s.Condition?.ArnEquals?.['iam:PermissionsBoundary'], s.Sid).toBe(BOUNDARY_ARN);
    }
  });

  it('AttachRolePolicy は AWSLambdaBasicExecutionRole だけ', () => {
    const attach = allowed(doc).filter((s) => list(s.Action).includes('iam:AttachRolePolicy'));
    expect(attach.map((s) => s.Sid)).toEqual(['IamAttachOnlyLambdaBasicExecution']);
    expect(attach[0].Condition?.ArnEquals?.['iam:PolicyARN']).toBe('arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole');
  });

  it('PassRole は PassedToService 条件付き', () => {
    for (const f of files) {
      for (const s of allowed(f).filter((x) => list(x.Action).includes('iam:PassRole'))) {
        expect(s.Condition?.StringEquals?.['iam:PassedToService'], `${f}: ${s.Sid}`).toBeTruthy();
      }
    }
    expect(stmt('IamPassProjectRoleToLambdaOnly')?.Condition?.StringEquals?.['iam:PassedToService']).toBe('lambda.amazonaws.com');
  });

});

describe('許可してはいけない操作', () => {
  // Allow の Action はワイルドカード（dynamodb:Delete* など）を展開して判定する
  const banned = [
    'dynamodb:DeleteTable',
    'lambda:InvokeFunction',
    'lambda:InvokeFunctionUrl',
    'iam:CreatePolicy',
    'iam:CreatePolicyVersion',
    'iam:SetDefaultPolicyVersion',
    'iam:DeletePolicy',
    'iam:DeletePolicyVersion',
    'iam:DeleteRolePermissionsBoundary',
    'cloudformation:DeleteStack',
  ];
  it.each(files)('%s', (f) => {
    for (const b of banned) expect(grants(f, b), b).toEqual([]);
  });

  it('boundary は logs:CreateLogGroup を許可しない', () => {
    expect(grants('boundary-policy.json', 'logs:CreateLogGroup')).toEqual([]);
  });

  it('cfn-exec-app-policy.json に iam: の Action が無い', () => {
    expect(allActions('cfn-exec-app-policy.json', 'Allow').filter((a) => a.startsWith('iam:'))).toEqual([]);
    expect(allActions('cfn-exec-app-policy.json', 'Deny').filter((a) => a.startsWith('iam:'))).toEqual([]);
  });

  it('権限セットは DynamoDB を書き込めず、logs:DescribeLogGroups も持たない', () => {
    for (const a of ['dynamodb:PutItem', 'dynamodb:UpdateItem', 'dynamodb:DeleteItem', 'dynamodb:BatchWriteItem', 'logs:DescribeLogGroups']) {
      expect(grants('permission-set-inline.json', a), a).toEqual([]);
    }
  });

  it('権限セットはリソースのインポートを Deny する', () => {
    const s = docs['permission-set-inline.json'].Statement.find((x) => x.Sid === 'DenyResourceImport');
    expect(s?.Effect).toBe('Deny');
    expect(s?.Action).toBe('cloudformation:CreateChangeSet');
    expect(s?.Condition).toEqual({ Null: { 'cloudformation:ImportResourceTypes': 'false' } });
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
    expect([...quals]).toEqual(f === 'boundary-policy.json' ? [] : [BOOTSTRAP_QUALIFIER]);
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

  it.runIf(f === 'cfn-exec-policy.json')('Budget の ARN 接頭辞が BUDGET_NAME の接頭辞', () => {
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
