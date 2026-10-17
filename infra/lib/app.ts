import { App, AspectPriority, Aspects, DefaultStackSynthesizer, Tags } from 'aws-cdk-lib';
import { RolePathAspect } from './iam-guards';
import { KakeiboStack } from './kakeibo-stack';
import {
  BOOTSTRAP_QUALIFIER,
  PROJECT_TAG_KEY,
  PROJECT_TAG_VALUE,
  STACK_NAME,
} from './project-constants';

// bin/app.ts とテストの両方がこの関数を使う
// context は通常 CLI が cdk.json から渡す。テストでは cdk.json の context を明示的に渡す
export function createApp(props: { context?: Record<string, unknown> } = {}): {
  app: App;
  stack: KakeiboStack;
} {
  const app = new App({ context: props.context });

  // env は渡さない。アカウント・リージョンは CLI の --profile で決まる
  const stack = new KakeiboStack(app, 'KakeiboStack', {
    stackName: STACK_NAME,
    synthesizer: new DefaultStackSynthesizer({ qualifier: BOOTSTRAP_QUALIFIER }),
  });

  Aspects.of(app).add(new RolePathAspect(), { priority: AspectPriority.MUTATING });
  Tags.of(app).add(PROJECT_TAG_KEY, PROJECT_TAG_VALUE);
  return { app, stack };
}
