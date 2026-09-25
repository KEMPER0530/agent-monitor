import { CloudFrontClient, CreateInvalidationCommand } from "@aws-sdk/client-cloudfront";

process.env.TZ = "Asia/Tokyo";

const cloudFront = new CloudFrontClient({ region: "ap-northeast-1" });
const distributionId = process.env.DIST_ID;

export const handler = async (event) => {
  console.log("event:", JSON.stringify(event));

  if (!distributionId) {
    throw new Error("DIST_ID is required");
  }
  if (!event.Records || event.Records.length === 0) {
    console.log("Records not found. Skip event.");
    return;
  }

  const paths = getPathsFromEvent(event);
  if (paths.Quantity === 0) {
    console.log("Invalidation paths not found. Skip event.");
    return;
  }

  await cloudFront.send(
    new CreateInvalidationCommand({
      DistributionId: distributionId,
      InvalidationBatch: {
        CallerReference: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
        Paths: paths,
      },
    }),
  );

  console.log("invalidate:", JSON.stringify(paths.Items));
};

function getPathsFromEvent(event) {
  const items = event.Records
    .filter((record) => record.s3?.object?.key)
    .map((record) => {
      const key = decodeURIComponent(record.s3.object.key.replace(/\+/g, " "));
      return `/${key}`;
    });

  return {
    Quantity: items.length,
    Items: items,
  };
}
