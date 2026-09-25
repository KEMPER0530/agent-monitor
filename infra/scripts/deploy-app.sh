#!/usr/bin/env bash
set -euo pipefail

# CDK作成後の通常更新では、静的UIとLambdaだけを差し替えます。
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AWS_REGION="${AWS_REGION:-ap-northeast-1}"
STACK_NAME="${STACK_NAME:-AgentMonitorStack}"

output_value() {
  local key="$1"
  aws cloudformation describe-stacks \
    --stack-name "${STACK_NAME}" \
    --region "${AWS_REGION}" \
    --query "Stacks[0].Outputs[?OutputKey=='${key}'].OutputValue | [0]" \
    --output text
}

bucket_name="${DASHBOARD_BUCKET_NAME:-$(output_value DashboardBucketName)}"
function_name="${INGEST_FUNCTION_NAME:-$(output_value IngestFunctionName)}"
distribution_id="${CLOUDFRONT_DISTRIBUTION_ID:-$(output_value DistributionId)}"
zip_file="$(mktemp -t agent-monitor-lambda.XXXXXX.zip)"
rm -f "${zip_file}"

# Lambdaは依存なしの単一ファイルなのでzip化して直接更新します。
(cd "${ROOT_DIR}/infra/lambda/ingest" && zip -q "${zip_file}" app.py)
aws lambda update-function-code \
  --function-name "${function_name}" \
  --zip-file "fileb://${zip_file}" \
  --region "${AWS_REGION}" >/dev/null

# ダッシュボードはS3へ同期し、古いファイルは削除します。
aws s3 sync "${ROOT_DIR}/web" "s3://${bucket_name}" \
  --delete \
  --exclude "auth-config.js" \
  --region "${AWS_REGION}"

# S3更新後はCloudFrontキャッシュを明示的に破棄します。
aws cloudfront create-invalidation \
  --distribution-id "${distribution_id}" \
  --paths "/*" >/dev/null

rm -f "${zip_file}"
echo "Application assets deployed and CloudFront invalidation requested."
