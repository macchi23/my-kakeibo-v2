import { CfnResource, IAspect } from 'aws-cdk-lib';
import { IConstruct } from 'constructs';
import { ROLE_PATH } from './project-constants';

// L2 / L1 を問わず、CDK 内部のカスタムリソース用ロールを含む全 AWS::IAM::Role の Path を ROLE_PATH にする。
// CFN 実行ロールのポリシーが path で CreateRole を絞るため
export class RolePathAspect implements IAspect {
  public visit(node: IConstruct): void {
    if (CfnResource.isCfnResource(node) && node.cfnResourceType === 'AWS::IAM::Role') {
      node.addPropertyOverride('Path', ROLE_PATH);
    }
  }
}
