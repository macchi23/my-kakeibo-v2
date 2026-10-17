import { CfnOutput, Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { Auth } from './auth';
import { ProjectBudget } from './budget';

export class KakeiboStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    const auth = new Auth(this, 'Auth');
    new ProjectBudget(this, 'Budget');

    new CfnOutput(this, 'UserPoolId', { value: auth.userPool.userPoolId });
    new CfnOutput(this, 'UserPoolClientId', { value: auth.userPoolClient.userPoolClientId });
  }
}
