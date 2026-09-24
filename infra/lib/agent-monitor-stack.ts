import * as path from "path";
import * as cdk from "aws-cdk-lib";
import { Duration, RemovalPolicy } from "aws-cdk-lib";
import * as apigateway from "aws-cdk-lib/aws-apigateway";
import * as cloudfront from "aws-cdk-lib/aws-cloudfront";
import * as origins from "aws-cdk-lib/aws-cloudfront-origins";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as s3 from "aws-cdk-lib/aws-s3";
import * as s3deploy from "aws-cdk-lib/aws-s3-deployment";
import { Construct } from "constructs";

export class AgentMonitorStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const isLocalStack = this.node.tryGetContext("localstack") === "true";

    const table = new dynamodb.Table(this, "EventsTable", {
      partitionKey: { name: "pk", type: dynamodb.AttributeType.STRING },
      sortKey: { name: "sk", type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: isLocalStack ? RemovalPolicy.DESTROY : RemovalPolicy.RETAIN,
    });

    const ingest = new lambda.Function(this, "IngestFunction", {
      runtime: lambda.Runtime.PYTHON_3_12,
      handler: "app.handler",
      code: lambda.Code.fromAsset(path.join(__dirname, "../lambda/ingest")),
      timeout: Duration.seconds(10),
      environment: {
        TABLE_NAME: table.tableName,
        LOCALSTACK_ENDPOINT: isLocalStack ? "http://host.docker.internal:4566" : "",
      },
    });
    table.grantReadWriteData(ingest);

    const api = new apigateway.RestApi(this, "AgentMonitorApi", {
      deployOptions: { stageName: "v1" },
      defaultCorsPreflightOptions: {
        allowOrigins: apigateway.Cors.ALL_ORIGINS,
        allowMethods: apigateway.Cors.ALL_METHODS,
      },
    });
    const events = api.root.addResource("events");
    events.addMethod("POST", new apigateway.LambdaIntegration(ingest));
    const snapshot = api.root.addResource("snapshot");
    snapshot.addMethod("GET", new apigateway.LambdaIntegration(ingest));

    const siteBucket = new s3.Bucket(this, "DashboardBucket", {
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      enforceSSL: !isLocalStack,
      removalPolicy: isLocalStack ? RemovalPolicy.DESTROY : RemovalPolicy.RETAIN,
      autoDeleteObjects: isLocalStack,
    });

    const originAccessIdentity = new cloudfront.OriginAccessIdentity(this, "DashboardOai");
    siteBucket.grantRead(originAccessIdentity);

    const distribution = new cloudfront.Distribution(this, "DashboardDistribution", {
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

    new s3deploy.BucketDeployment(this, "DeployDashboard", {
      destinationBucket: siteBucket,
      sources: [s3deploy.Source.asset(path.join(__dirname, "../../web"))],
      distribution,
      distributionPaths: ["/*"],
    });

    new cdk.CfnOutput(this, "DashboardUrl", {
      value: `https://${distribution.distributionDomainName}`,
    });
    new cdk.CfnOutput(this, "ApiUrl", {
      value: api.url,
    });
  }
}
