import { CfnOutput, Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { Api } from './api';
import { Auth } from './auth';
import { ProjectBudget } from './budget';
import { WITH_HTTP_API_CONTEXT_KEY } from './project-constants';

export class KakeiboStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    const auth = new Auth(this, 'Auth');
    new ProjectBudget(this, 'Budget');

    // CLI の -c は文字列で渡る。'false' と false の両方を false と扱う
    const withHttpApi = String(this.node.tryGetContext(WITH_HTTP_API_CONTEXT_KEY)) !== 'false';
    const api = new Api(this, 'Api', {
      userPool: auth.userPool,
      userPoolClient: auth.userPoolClient,
      withHttpApi,
    });

    // Output の論理 ID を固定するため Construct の外（スタック直下）で作る
    if (api.httpApi) new CfnOutput(this, 'ApiUrl', { value: api.httpApi.apiEndpoint });
    new CfnOutput(this, 'UserPoolId', { value: auth.userPool.userPoolId });
    new CfnOutput(this, 'UserPoolClientId', { value: auth.userPoolClient.userPoolClientId });
  }
}
