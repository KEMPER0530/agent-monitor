import * as path from "path";
import * as cdk from "aws-cdk-lib";
import { Duration, RemovalPolicy } from "aws-cdk-lib";
import * as apigateway from "aws-cdk-lib/aws-apigateway";
import * as acm from "aws-cdk-lib/aws-certificatemanager";
import * as cloudfront from "aws-cdk-lib/aws-cloudfront";
import * as origins from "aws-cdk-lib/aws-cloudfront-origins";
import * as cognito from "aws-cdk-lib/aws-cognito";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as route53 from "aws-cdk-lib/aws-route53";
import * as targets from "aws-cdk-lib/aws-route53-targets";
import * as s3 from "aws-cdk-lib/aws-s3";
import * as s3deploy from "aws-cdk-lib/aws-s3-deployment";
import { Construct } from "constructs";

export class AgentMonitorStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // LocalStackでは削除しやすさを優先し、AWS本番ではデータ保持を優先します。
    const isLocalStack = this.node.tryGetContext("localstack") === "true";
    const zoneName = this.node.tryGetContext("zoneName") || "kemper0530.com";
    const dashboardDomain = this.node.tryGetContext("dashboardDomain") || `s3-agent-monitor.${zoneName}`;
    const userPoolId = this.node.tryGetContext("userPoolId") || "ap-northeast-1_7da4pYlPc";
    const cognitoDomainPrefix = this.node.tryGetContext("cognitoDomainPrefix") || "agent-monitor-kemper0530";
    const callbackUrl = `https://${dashboardDomain}/`;

    // CodexとClaudeのイベントは別テーブルに分け、誤混在を防ぎます。
    const codexTable = new dynamodb.Table(this, "CodexEventsTable", {
      partitionKey: { name: "pk", type: dynamodb.AttributeType.STRING },
      sortKey: { name: "sk", type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      tableName: "agent-monitor-codex-events",
      removalPolicy: isLocalStack ? RemovalPolicy.DESTROY : RemovalPolicy.RETAIN,
    });
    const claudeTable = new dynamodb.Table(this, "ClaudeEventsTable", {
      partitionKey: { name: "pk", type: dynamodb.AttributeType.STRING },
      sortKey: { name: "sk", type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      tableName: "agent-monitor-claude-events",
      removalPolicy: isLocalStack ? RemovalPolicy.DESTROY : RemovalPolicy.RETAIN,
    });

    // Lambdaはイベント登録とスナップショット取得の両方を担当する薄いAPI層です。
    const ingest = new lambda.Function(this, "IngestFunction", {
      runtime: lambda.Runtime.PYTHON_3_12,
      handler: "app.handler",
      code: lambda.Code.fromAsset(path.join(__dirname, "../lambda/ingest")),
      timeout: Duration.seconds(10),
      functionName: "agent-monitor-ingest",
      environment: {
        CODEX_EVENTS_TABLE_NAME: codexTable.tableName,
        CLAUDE_EVENTS_TABLE_NAME: claudeTable.tableName,
        CODEX_MONITOR_ENABLED: "true",
        CLAUDE_MONITOR_ENABLED: "true",
        DEFAULT_AGENT: "codex",
        LOCALSTACK_ENDPOINT: isLocalStack ? "http://host.docker.internal:4566" : "",
      },
    });
    codexTable.grantReadWriteData(ingest);
    claudeTable.grantReadWriteData(ingest);

    // 既存User Poolを使い、ダッシュボードとAPIの認証境界にします。
    const userPool = cognito.UserPool.fromUserPoolId(this, "DashboardUserPool", userPoolId);
    const userPoolClient = new cognito.CfnUserPoolClient(this, "DashboardUserPoolClient", {
      userPoolId,
      clientName: "agent-monitor-dashboard",
      generateSecret: false,
      allowedOAuthFlowsUserPoolClient: true,
      allowedOAuthFlows: ["implicit"],
      allowedOAuthScopes: ["openid", "email", "profile"],
      supportedIdentityProviders: ["COGNITO"],
      callbackUrLs: [callbackUrl],
      logoutUrLs: [callbackUrl],
      preventUserExistenceErrors: "ENABLED",
    });
    const userPoolDomain = new cognito.CfnUserPoolDomain(this, "DashboardUserPoolDomain", {
      domain: cognitoDomainPrefix,
      userPoolId,
    });
    const authorizer = new apigateway.CognitoUserPoolsAuthorizer(this, "DashboardApiAuthorizer", {
      cognitoUserPools: [userPool],
    });

    // API GatewayはCloudFrontからの/api/*を受け、Cognito認証後にLambdaへプロキシします。
    const api = new apigateway.RestApi(this, "AgentMonitorApi", {
      deployOptions: { stageName: "v1" },
      defaultCorsPreflightOptions: {
        allowOrigins: apigateway.Cors.ALL_ORIGINS,
        allowMethods: apigateway.Cors.ALL_METHODS,
        allowHeaders: ["Content-Type", "Authorization", "X-Amz-Date", "X-Api-Key", "X-Amz-Security-Token"],
      },
    });
    const apiResource = api.root.addResource("api");
    const events = apiResource.addResource("events");
    events.addMethod("POST", new apigateway.LambdaIntegration(ingest), {
      authorizationType: apigateway.AuthorizationType.COGNITO,
      authorizer,
    });
    const snapshot = apiResource.addResource("snapshot");
    snapshot.addMethod("GET", new apigateway.LambdaIntegration(ingest), {
      authorizationType: apigateway.AuthorizationType.COGNITO,
      authorizer,
    });

    // ダッシュボードはS3に置き、公開はCloudFront経由に限定します。
    const siteBucket = new s3.Bucket(this, "DashboardBucket", {
      bucketName: isLocalStack ? undefined : dashboardDomain,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      enforceSSL: !isLocalStack,
      removalPolicy: isLocalStack ? RemovalPolicy.DESTROY : RemovalPolicy.RETAIN,
      autoDeleteObjects: isLocalStack,
    });

    const originAccessIdentity = new cloudfront.OriginAccessIdentity(this, "DashboardOai");
    siteBucket.grantRead(originAccessIdentity);

    // CloudFrontの独自ドメインにはus-east-1のACM証明書が必要です。
    const hostedZone = isLocalStack ? undefined : route53.HostedZone.fromLookup(this, "HostedZone", { domainName: zoneName });
    const certificate = hostedZone
      ? new acm.DnsValidatedCertificate(this, "DashboardCertificate", {
          domainName: dashboardDomain,
          hostedZone,
          region: "us-east-1",
        })
      : undefined;

    // CloudFrontは静的UIとAPIを同一オリジン風に見せる入口です。
    const distribution = new cloudfront.Distribution(this, "DashboardDistribution", {
      ...(certificate
        ? {
            domainNames: [dashboardDomain],
            certificate,
          }
        : {}),
      defaultBehavior: {
        origin: new origins.S3Origin(siteBucket, { originAccessIdentity }),
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
      },
      additionalBehaviors: {
        "/api/*": {
          origin: new origins.RestApiOrigin(api),
          allowedMethods: cloudfront.AllowedMethods.ALLOW_ALL,
          cachePolicy: cloudfront.CachePolicy.CACHING_DISABLED,
          originRequestPolicy: cloudfront.OriginRequestPolicy.ALL_VIEWER_EXCEPT_HOST_HEADER,
        },
      },
      defaultRootObject: "index.html",
    });

    if (hostedZone) {
      // s3-agent-monitor.kemper0530.comをCloudFrontへ向けます。
      const recordName = dashboardDomain.replace(`.${zoneName}`, "");
      new route53.ARecord(this, "DashboardAliasRecord", {
        zone: hostedZone,
        recordName,
        target: route53.RecordTarget.fromAlias(new targets.CloudFrontTarget(distribution)),
      });
      new route53.AaaaRecord(this, "DashboardAliasIpv6Record", {
        zone: hostedZone,
        recordName,
        target: route53.RecordTarget.fromAlias(new targets.CloudFrontTarget(distribution)),
      });
    }

    // web/配下をそのままS3へ同期し、配信キャッシュも更新します。
    new s3deploy.BucketDeployment(this, "DeployDashboard", {
      destinationBucket: siteBucket,
      sources: [
        s3deploy.Source.asset(path.join(__dirname, "../../web")),
        s3deploy.Source.data(
          "auth-config.js",
          [
            "window.AGENT_MONITOR_AUTH = {",
            `  region: "${this.region}",`,
            `  userPoolId: "${userPoolId}",`,
            `  clientId: "${userPoolClient.ref}",`,
            `  domain: "https://${cognitoDomainPrefix}.auth.${this.region}.amazoncognito.com",`,
            `  redirectUri: "${callbackUrl}",`,
            "};",
          ].join("\n"),
        ),
      ],
      distribution,
      distributionPaths: ["/*"],
    });

    new cdk.CfnOutput(this, "DashboardUrl", {
      value: certificate ? `https://${dashboardDomain}` : `https://${distribution.distributionDomainName}`,
    });
    new cdk.CfnOutput(this, "ApiUrl", {
      value: api.url,
    });
    new cdk.CfnOutput(this, "DashboardBucketName", {
      value: siteBucket.bucketName,
    });
    new cdk.CfnOutput(this, "IngestFunctionName", {
      value: ingest.functionName,
    });
    new cdk.CfnOutput(this, "DistributionId", {
      value: distribution.distributionId,
    });
    new cdk.CfnOutput(this, "CognitoClientId", {
      value: userPoolClient.ref,
    });
    new cdk.CfnOutput(this, "CognitoDomain", {
      value: `https://${cognitoDomainPrefix}.auth.${this.region}.amazoncognito.com`,
    });
  }
}
