import { Duration, RemovalPolicy } from 'aws-cdk-lib';
import { AccountRecovery, Mfa, UserPool, UserPoolClient } from 'aws-cdk-lib/aws-cognito';
import { Construct } from 'constructs';

export class Auth extends Construct {
  public readonly userPool: UserPool;
  public readonly userPoolClient: UserPoolClient;

  constructor(scope: Construct, id: string) {
    super(scope, id);

    // 個人利用なので管理者がユーザーを作る。ユーザーデータ保護のため RETAIN + 削除保護
    this.userPool = new UserPool(this, 'UserPool', {
      selfSignUpEnabled: false,
      signInAliases: { email: true },
      accountRecovery: AccountRecovery.EMAIL_ONLY,
      mfa: Mfa.REQUIRED,
      mfaSecondFactor: { otp: true, sms: false },
      removalPolicy: RemovalPolicy.RETAIN,
      deletionProtection: true,
    });

    // P2 で amplify の Authenticator を SRP で使う。Hosted UI / OAuth は不要
    this.userPoolClient = this.userPool.addClient('WebClient', {
      generateSecret: false,
      disableOAuth: true,
      authFlows: { userSrp: true },
      preventUserExistenceErrors: true,
    });
  }
}
