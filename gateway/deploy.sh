#!/bin/bash
set -e

AWS_REGION="${AWS_REGION:-us-east-1}"

if [ -z "$INSTANCE_ID" ] || [ -z "$ECR_REGISTRY" ] || [ -z "$CLOUDFLARE_CERT_PEM" ] || [ -z "$CLOUDFLARE_CERT_KEY" ] || [ -z "$YOURPLACE_ORIGIN" ] || [ -z "$BASE_RPC_URL" ]; then
  echo "ERROR: Missing required environment variables"
  echo "Required: INSTANCE_ID, ECR_REGISTRY, CLOUDFLARE_CERT_PEM, CLOUDFLARE_CERT_KEY, YOURPLACE_ORIGIN, BASE_RPC_URL"
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

echo "Checking for pending SSM commands..."
PENDING_COMMANDS=$(aws ssm list-commands \
  --instance-id "$INSTANCE_ID" \
  --filters "key=Status,value=Pending" "key=Status,value=InProgress" \
  --region "$AWS_REGION" \
  --output json 2>/dev/null || echo '{"Commands":[]}')

PENDING_COUNT=$(echo "$PENDING_COMMANDS" | jq '.Commands | length')
if [ "$PENDING_COUNT" -gt 0 ]; then
  echo "WARNING: Found $PENDING_COUNT pending/in-progress command(s) on instance"
  echo "$PENDING_COMMANDS" | jq -r '.Commands[] | "  - Command: \(.CommandId) Status: \(.Status) Requested: \(.RequestedDateTime)"'
  echo ""
else
  echo "No pending commands found"
  echo ""
fi

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
echo '=== Fetching database credentials from Secrets Manager ==='
MYSQL_DSN_ENV=""
if SECRET_JSON=$(aws secretsmanager get-secret-value --secret-id yourplace/database/gateway --region AWS_REGION_PLACEHOLDER --query SecretString --output text 2>/dev/null); then
  YOURPLACE_MYSQL_DSN=$(echo "$SECRET_JSON" | jq -r '.dsn')
  if [ -n "$YOURPLACE_MYSQL_DSN" ] && [ "$YOURPLACE_MYSQL_DSN" != "null" ]; then
    export YOURPLACE_MYSQL_DSN
    MYSQL_DSN_ENV="-e YOURPLACE_MYSQL_DSN"
    echo 'Database credentials retrieved successfully'
  fi
else
  echo 'No database secret found, using SQLite'
fi
echo '=== Logging into ECR ==='
aws ecr get-login-password --region AWS_REGION_PLACEHOLDER | docker login --username AWS --password-stdin ECR_REGISTRY_PLACEHOLDER
echo '=== Pulling latest image ==='
docker pull ECR_REGISTRY_PLACEHOLDER/yourplace-gateway:latest
echo '=== Stopping existing container ==='
docker stop yourplace-gateway 2>/dev/null || echo 'No existing container'
docker rm yourplace-gateway 2>/dev/null || echo 'No existing container'
echo '=== Starting new container ==='
docker run -d --name yourplace-gateway --restart unless-stopped --log-driver awslogs --log-opt awslogs-region=AWS_REGION_PLACEHOLDER --log-opt awslogs-group=/ec2/yourplace-gateway -p 443:443 -v /opt/YourPlace:/opt/YourPlace -e YOURPLACE_ORIGIN='YOURPLACE_ORIGIN_PLACEHOLDER' -e BASE_RPC_URL='BASE_RPC_URL_PLACEHOLDER' -e BASE_RPC_THROTTLE='BASE_RPC_THROTTLE_PLACEHOLDER' $MYSQL_DSN_ENV ECR_REGISTRY_PLACEHOLDER/yourplace-gateway:latest
echo '=== Verifying container started ==='
docker ps | grep yourplace-gateway
echo '=== Waiting for server to be ready (max 5 minutes) ==='
READY=0
for i in $(seq 1 60); do
  if timeout 5 bash -c 'cat < /dev/null > /dev/tcp/localhost/443' 2>/dev/null; then
    echo "Server is ready on port 443 after $((i * 5)) seconds"
    READY=1
    break
  fi
  echo "Waiting for port 443... ($i/60)"
  sleep 5
done
if [ $READY -eq 0 ]; then
  echo "ERROR: Server did not become ready on port 443 within 300 seconds"
  echo "Container logs:"
  docker logs --tail 50 yourplace-gateway
  exit 1
fi
echo '=== Deployment complete ==='
EOF
)

# Replace placeholders with actual values
# Note: BASE_RPC_URL uses sed with | delimiter because URLs contain / characters
# that break bash's ${//} substitution syntax
SCRIPT="${SCRIPT//CERT_PEM_BASE64_PLACEHOLDER/$CERT_PEM_BASE64}"
SCRIPT="${SCRIPT//CERT_KEY_BASE64_PLACEHOLDER/$CERT_KEY_BASE64}"
SCRIPT="${SCRIPT//AWS_REGION_PLACEHOLDER/$AWS_REGION}"
SCRIPT="${SCRIPT//ECR_REGISTRY_PLACEHOLDER/$ECR_REGISTRY}"
SCRIPT="${SCRIPT//YOURPLACE_ORIGIN_PLACEHOLDER/$YOURPLACE_ORIGIN}"
SCRIPT="${SCRIPT//BASE_RPC_THROTTLE_PLACEHOLDER/$BASE_RPC_THROTTLE}"
SCRIPT=$(echo "$SCRIPT" | sed "s|BASE_RPC_URL_PLACEHOLDER|${BASE_RPC_URL}|g")

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

echo "Waiting for deployment to complete (timeout: 10 minutes)..."
# Wait for command to complete with longer timeout
WAIT_COUNT=0
MAX_WAIT=60  # 60 * 10 seconds = 10 minutes

while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
  STATUS_CHECK=$(aws ssm get-command-invocation \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID" \
    --region "$AWS_REGION" \
    --output json 2>/dev/null || echo "{}")

  CURRENT_STATUS=$(echo "$STATUS_CHECK" | jq -r '.Status // "Unknown"')

  if [ "$CURRENT_STATUS" = "Success" ] || [ "$CURRENT_STATUS" = "Failed" ] || [ "$CURRENT_STATUS" = "Cancelled" ] || [ "$CURRENT_STATUS" = "TimedOut" ]; then
    echo "Command completed with status: $CURRENT_STATUS"
    break
  fi

  if [ $((WAIT_COUNT % 6)) -eq 0 ]; then
    echo "Status: $CURRENT_STATUS (waited $((WAIT_COUNT * 10)) seconds)"
  fi

  sleep 10
  WAIT_COUNT=$((WAIT_COUNT + 1))
done

if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
  echo "WARNING: Command did not complete within timeout"
fi

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
