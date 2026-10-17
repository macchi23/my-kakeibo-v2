import { readFileSync } from 'node:fs';
import { Stack } from 'aws-cdk-lib';
import { Template } from 'aws-cdk-lib/assertions';
import * as iam from 'aws-cdk-lib/aws-iam';
import { describe, expect, it } from 'vitest';
import { createApp } from '../lib/app';
import { PERMISSIONS_BOUNDARY_NAME, ROLE_PATH } from '../lib/project-constants';

const cdkJson = JSON.parse(readFileSync(new URL('../cdk.json', import.meta.url), 'utf-8'));

// 本番スタックには Role が無いので、同じ context と Aspect を持つ app にテスト専用スタックを足して確かめる
function guardedTemplate(): Template {
  const { app } = createApp({ context: cdkJson.context });
  const s = new Stack(app, 'GuardTest');
  new iam.Role(s, 'L2Role', { assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com') });
  new iam.CfnRole(s, 'L1Role', {
    assumeRolePolicyDocument: {
      Version: '2012-10-17',
      Statement: [{ Effect: 'Allow', Principal: { Service: 'lambda.amazonaws.com' }, Action: 'sts:AssumeRole' }],
    },
  });
  return Template.fromStack(s);
}

describe('IAM ガード', () => {
  const roles = Object.entries(guardedTemplate().findResources('AWS::IAM::Role'));

  it('L2 / L1 のロール両方が対象になっている', () => {
    expect(roles.map(([id]) => id).join(',')).toMatch(/L2Role/);
    expect(roles.map(([id]) => id).join(',')).toMatch(/L1Role/);
  });

  it('全ロールの Path が ROLE_PATH', () => {
    for (const [, r] of roles) expect(r.Properties.Path).toBe(ROLE_PATH);
  });

  it('L2 ロールに permissions boundary が付く', () => {
    const l2 = roles.find(([id]) => id.includes('L2Role'));
    expect(l2).toBeDefined();
    const arn = JSON.stringify(l2![1].Properties.PermissionsBoundary);
    expect(arn).toContain(`:policy/${PERMISSIONS_BOUNDARY_NAME}`);
  });
});

describe('cdk.json の permissionsBoundary', () => {
  it('name が PERMISSIONS_BOUNDARY_NAME と一致する', () => {
    expect(cdkJson.context['@aws-cdk/core:permissionsBoundary']).toEqual({ name: PERMISSIONS_BOUNDARY_NAME });
  });
});
