#!/bin/bash
set -e

AWS_REGION="${AWS_REGION:-us-east-1}"

if [ -z "$INSTANCE_ID" ] || [ -z "$ECR_REGISTRY" ] || [ -z "$CLOUDFLARE_CERT_PEM" ] || [ -z "$CLOUDFLARE_CERT_KEY" ]; then
  echo "ERROR: Missing required environment variables"
  echo "Required: INSTANCE_ID, ECR_REGISTRY, CLOUDFLARE_CERT_PEM, CLOUDFLARE_CERT_KEY"
  exit 1
fi

echo "Checking SSM agent status..."
SSM_STATUS=$(aws ssm describe-instance-information \
  --filters "Key=InstanceIds,Values=$INSTANCE_ID" \
  --region "$AWS_REGION" \
  --output json 2>/dev/null || echo "{}")

PING_STATUS=$(echo "$SSM_STATUS" | jq -r '.InstanceInformationList[0].PingStatus // "Unknown"')
AGENT_VERSION=$(echo "$SSM_STATUS" | jq -r '.InstanceInformationList[0].AgentVersion // "Unknown"')

echo "SSM Ping Status: $PING_STATUS"
echo "SSM Agent Version: $AGENT_VERSION"

if [ "$PING_STATUS" != "Online" ]; then
  echo "ERROR: SSM agent is not online on instance $INSTANCE_ID"
  echo "Please ensure:"
  echo "  1. The EC2 instance has the SSM agent installed and running"
  echo "  2. The instance has the AmazonSSMManagedInstanceCore IAM policy"
  echo "  3. The instance has network connectivity to SSM endpoints"
  echo ""
  echo "Full SSM status:"
  echo "$SSM_STATUS" | jq '.'
  exit 1
fi

echo "SSM agent is online and ready"
echo ""

echo "Encoding certificates..."
CERT_PEM_BASE64=$(echo "$CLOUDFLARE_CERT_PEM" | base64 -w 0 2>/dev/null || echo "$CLOUDFLARE_CERT_PEM" | base64)
CERT_KEY_BASE64=$(echo "$CLOUDFLARE_CERT_KEY" | base64 -w 0 2>/dev/null || echo "$CLOUDFLARE_CERT_KEY" | base64)

echo "Building deployment command..."
# Build commands as a single shell script for AWS SSM
SCRIPT=$(cat <<'EOF'
set -e
echo '=== Installing TLS certificates ==='
mkdir -p /opt/YourPlace
echo 'CERT_PEM_BASE64_PLACEHOLDER' | base64 -d > /opt/YourPlace/cert.pem
echo 'CERT_KEY_BASE64_PLACEHOLDER' | base64 -d > /opt/YourPlace/cert.key
chmod 644 /opt/YourPlace/cert.pem
chmod 600 /opt/YourPlace/cert.key
echo '=== Logging into ECR ==='
aws ecr get-login-password --region AWS_REGION_PLACEHOLDER | docker login --username AWS --password-stdin ECR_REGISTRY_PLACEHOLDER
echo '=== Pulling latest image ==='
docker pull ECR_REGISTRY_PLACEHOLDER/yourplace-gateway:latest
echo '=== Stopping existing container ==='
docker stop yourplace-gateway 2>/dev/null || echo 'No existing container'
docker rm yourplace-gateway 2>/dev/null || echo 'No existing container'
echo '=== Starting new container ==='
docker run -d --name yourplace-gateway --restart unless-stopped -p 443:443 -v /opt/YourPlace:/opt/YourPlace ECR_REGISTRY_PLACEHOLDER/yourplace-gateway:latest
echo '=== Verifying container ==='
docker ps | grep yourplace-gateway
echo '=== Deployment complete ==='
EOF
)

# Replace placeholders with actual values
SCRIPT="${SCRIPT//CERT_PEM_BASE64_PLACEHOLDER/$CERT_PEM_BASE64}"
SCRIPT="${SCRIPT//CERT_KEY_BASE64_PLACEHOLDER/$CERT_KEY_BASE64}"
SCRIPT="${SCRIPT//AWS_REGION_PLACEHOLDER/$AWS_REGION}"
SCRIPT="${SCRIPT//ECR_REGISTRY_PLACEHOLDER/$ECR_REGISTRY}"

# Create proper JSON parameters structure
PARAMETERS=$(jq -n \
  --arg script "$SCRIPT" \
  '{commands: [$script]}')

# Check parameter size (AWS SSM limit is 48KB for parameters)
PARAM_SIZE=$(echo "$PARAMETERS" | wc -c)
echo "Parameter size: $PARAM_SIZE bytes"
if [ "$PARAM_SIZE" -gt 48000 ]; then
  echo "WARNING: Parameters size ($PARAM_SIZE bytes) approaching SSM limit (48KB)"
fi

echo "Sending deployment command to instance $INSTANCE_ID..."
COMMAND_ID=$(aws ssm send-command \
  --instance-ids "$INSTANCE_ID" \
  --document-name "AWS-RunShellScript" \
  --parameters "$PARAMETERS" \
  --region "$AWS_REGION" \
  --output text \
  --query 'Command.CommandId')

echo "Command ID: $COMMAND_ID"

echo "Waiting for deployment to complete..."
aws ssm wait command-executed \
  --command-id "$COMMAND_ID" \
  --instance-id "$INSTANCE_ID" \
  --region "$AWS_REGION" \
  2>&1 || true

echo ""
echo "=== Deployment Results ==="
RESULT=$(aws ssm get-command-invocation \
  --command-id "$COMMAND_ID" \
  --instance-id "$INSTANCE_ID" \
  --region "$AWS_REGION" \
  --output json)

STATUS=$(echo "$RESULT" | jq -r '.Status')
echo "Status: $STATUS"

STATUS_DETAILS=$(echo "$RESULT" | jq -r '.StatusDetails // "No details"')
echo "Status Details: $STATUS_DETAILS"

echo ""
echo "=== Output ==="
OUTPUT=$(echo "$RESULT" | jq -r '.StandardOutputContent // "No output"')
echo "$OUTPUT"

echo ""
echo "=== Errors ==="
ERRORS=$(echo "$RESULT" | jq -r '.StandardErrorContent // "No errors"')
echo "$ERRORS"

# Show full result for debugging if failed with no output
if [ "$STATUS" != "Success" ] && [ "$OUTPUT" = "No output" ]; then
  echo ""
  echo "=== Full Result (Debug) ==="
  echo "$RESULT" | jq '.'
fi

if [ "$STATUS" != "Success" ]; then
  echo ""
  echo "ERROR: Deployment failed"
  exit 1
fi

echo ""
echo "SUCCESS: Gateway deployed successfully"
