#!/bin/bash
set -e

AWS_REGION="${AWS_REGION:-us-east-1}"

if [ -z "$INSTANCE_ID" ] || [ -z "$ECR_REGISTRY" ] || [ -z "$CLOUDFLARE_CERT_PEM" ] || [ -z "$CLOUDFLARE_CERT_KEY" ] || [ -z "$YOURPLACE_ORIGIN" ] || [ -z "$BASE_RPC_URL" ] || [ -z "$ETHEREUM_RPC_URL" ]; then
  echo "ERROR: Missing required environment variables"
  echo "Required: INSTANCE_ID, ECR_REGISTRY, CLOUDFLARE_CERT_PEM, CLOUDFLARE_CERT_KEY, YOURPLACE_ORIGIN, BASE_RPC_URL, ETHEREUM_RPC_URL"
  exit 1
fi

print_instance_diagnostics() {
  local instance_details instance_status

  instance_details=$(aws ec2 describe-instances \
    --instance-ids "$INSTANCE_ID" \
    --region "$AWS_REGION" \
    --output json 2>/dev/null || echo "{}")
  instance_status=$(aws ec2 describe-instance-status \
    --include-all-instances \
    --instance-ids "$INSTANCE_ID" \
    --region "$AWS_REGION" \
    --output json 2>/dev/null || echo "{}")

  echo "EC2 instance summary:"
  echo "$instance_details" | jq '{
    InstanceId: .Reservations[0].Instances[0].InstanceId,
    State: .Reservations[0].Instances[0].State.Name,
    LaunchTime: .Reservations[0].Instances[0].LaunchTime,
    PublicIpAddress: .Reservations[0].Instances[0].PublicIpAddress,
    PrivateIpAddress: .Reservations[0].Instances[0].PrivateIpAddress,
    SubnetId: .Reservations[0].Instances[0].SubnetId,
    VpcId: .Reservations[0].Instances[0].VpcId,
    IamInstanceProfile: .Reservations[0].Instances[0].IamInstanceProfile.Arn,
    SecurityGroups: .Reservations[0].Instances[0].SecurityGroups
  }'
  echo ""
  echo "EC2 status checks:"
  echo "$instance_status" | jq '{
    InstanceId: .InstanceStatuses[0].InstanceId,
    AvailabilityZone: .InstanceStatuses[0].AvailabilityZone,
    InstanceState: .InstanceStatuses[0].InstanceState.Name,
    SystemStatus: .InstanceStatuses[0].SystemStatus.Status,
    InstanceStatus: .InstanceStatuses[0].InstanceStatus.Status,
    AttachedEbsStatus: .InstanceStatuses[0].AttachedEbsStatus.Status,
    Events: .InstanceStatuses[0].Events
  }'
  echo ""
}

echo "Checking EC2 instance status..."
print_instance_diagnostics

echo "Checking SSM agent status..."
SSM_WAIT_COUNT=0
SSM_MAX_WAIT=12
PING_STATUS="Unknown"
AGENT_VERSION="Unknown"
LAST_PING="Unknown"

while [ $SSM_WAIT_COUNT -lt $SSM_MAX_WAIT ]; do
  SSM_STATUS=$(aws ssm describe-instance-information \
    --filters "Key=InstanceIds,Values=$INSTANCE_ID" \
    --region "$AWS_REGION" \
    --output json 2>/dev/null || echo "{}")

  PING_STATUS=$(echo "$SSM_STATUS" | jq -r '.InstanceInformationList[0].PingStatus // "Unknown"')
  AGENT_VERSION=$(echo "$SSM_STATUS" | jq -r '.InstanceInformationList[0].AgentVersion // "Unknown"')
  LAST_PING=$(echo "$SSM_STATUS" | jq -r '.InstanceInformationList[0].LastPingDateTime // "Unknown"')

  echo "SSM Ping Status: $PING_STATUS"
  echo "SSM Agent Version: $AGENT_VERSION"
  echo "Last SSM Ping: $LAST_PING"

  if [ "$PING_STATUS" = "Online" ]; then
    break
  fi

  SSM_WAIT_COUNT=$((SSM_WAIT_COUNT + 1))
  if [ $SSM_WAIT_COUNT -lt $SSM_MAX_WAIT ]; then
    echo "SSM agent is not online yet, retrying in 10 seconds... ($SSM_WAIT_COUNT/$SSM_MAX_WAIT)"
    sleep 10
  fi
done

if [ "$PING_STATUS" != "Online" ]; then
  echo "ERROR: SSM agent is not online on instance $INSTANCE_ID"
  echo "Please ensure:"
  echo "  1. The EC2 instance has the SSM agent installed and running"
  echo "  2. The instance has the AmazonSSMManagedInstanceCore IAM policy"
  echo "  3. The instance has network connectivity to SSM endpoints"
  echo ""
  print_instance_diagnostics
  echo "Last SSM Ping: $LAST_PING"
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
load_instance_profile_credentials() {
  unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_CREDENTIAL_EXPIRATION AWS_PROFILE AWS_DEFAULT_PROFILE AWS_CONTAINER_CREDENTIALS_FULL_URI AWS_CONTAINER_CREDENTIALS_RELATIVE_URI
  export AWS_EC2_METADATA_DISABLED=false
  IMDS_TOKEN=$(curl -fsS -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
  IMDS_ROLE_NAME=$(curl -fsS -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" "http://169.254.169.254/latest/meta-data/iam/security-credentials/")
  IMDS_CREDENTIALS=$(curl -fsS -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" "http://169.254.169.254/latest/meta-data/iam/security-credentials/$IMDS_ROLE_NAME")
  AWS_ACCESS_KEY_ID=$(echo "$IMDS_CREDENTIALS" | jq -r '.AccessKeyId // empty')
  AWS_SECRET_ACCESS_KEY=$(echo "$IMDS_CREDENTIALS" | jq -r '.SecretAccessKey // empty')
  AWS_SESSION_TOKEN=$(echo "$IMDS_CREDENTIALS" | jq -r '.Token // empty')
  AWS_CREDENTIAL_EXPIRATION=$(echo "$IMDS_CREDENTIALS" | jq -r '.Expiration // empty')
  if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ] || [ -z "$AWS_SESSION_TOKEN" ]; then
    echo 'ERROR: Failed to retrieve EC2 instance profile credentials from IMDS'
    echo "$IMDS_CREDENTIALS"
    exit 1
  fi
  export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_CREDENTIAL_EXPIRATION
  echo "Instance profile role: $IMDS_ROLE_NAME"
}
aws_with_instance_profile() {
  aws "$@"
}
echo '=== Using EC2 instance profile credentials ==='
load_instance_profile_credentials
aws_with_instance_profile sts get-caller-identity --region AWS_REGION_PLACEHOLDER --output json
echo '=== Installing TLS certificates ==='
mkdir -p /opt/YourPlace
echo 'CERT_PEM_BASE64_PLACEHOLDER' | base64 -d > /opt/YourPlace/cert.pem
echo 'CERT_KEY_BASE64_PLACEHOLDER' | base64 -d > /opt/YourPlace/cert.key
chmod 644 /opt/YourPlace/cert.pem
chmod 600 /opt/YourPlace/cert.key
echo '=== Fetching database credentials from Secrets Manager ==='
MYSQL_DSN_ENV=""
SECRET_ERROR_FILE=$(mktemp)
if SECRET_JSON=$(aws_with_instance_profile secretsmanager get-secret-value --secret-id yourplace/database/gateway --region AWS_REGION_PLACEHOLDER --query SecretString --output text 2>"$SECRET_ERROR_FILE"); then
  YOURPLACE_MYSQL_DSN=$(echo "$SECRET_JSON" | jq -r '"\(.username):\(.password)@tcp(\(.host):\(.port))/\(.dbname)"')
  if [ -n "$YOURPLACE_MYSQL_DSN" ] && [ "$YOURPLACE_MYSQL_DSN" != "null:null@tcp(null:null)/null" ]; then
    export YOURPLACE_MYSQL_DSN
    MYSQL_DSN_ENV="-e YOURPLACE_MYSQL_DSN"
    echo 'Database credentials retrieved successfully'
  fi
else
  if grep -q 'ResourceNotFoundException' "$SECRET_ERROR_FILE"; then
    echo 'No database secret found, using SQLite'
  else
    echo 'ERROR: Failed to fetch database credentials from Secrets Manager'
    cat "$SECRET_ERROR_FILE"
    rm -f "$SECRET_ERROR_FILE"
    exit 1
  fi
fi
rm -f "$SECRET_ERROR_FILE"
echo '=== Logging into ECR ==='
ECR_PASSWORD=$(aws_with_instance_profile ecr get-login-password --region AWS_REGION_PLACEHOLDER)
printf '%s' "$ECR_PASSWORD" | docker login --username AWS --password-stdin ECR_REGISTRY_PLACEHOLDER
echo '=== Cleaning up old Docker images ==='
docker image prune -af --filter "until=24h" 2>/dev/null || docker image prune -af 2>/dev/null || echo 'Prune skipped'
echo '=== Pulling latest image ==='
docker pull ECR_REGISTRY_PLACEHOLDER/yourplace-gateway:latest
echo '=== Stopping existing container ==='
docker stop yourplace-gateway 2>/dev/null || echo 'No existing container'
docker rm yourplace-gateway 2>/dev/null || echo 'No existing container'
echo '=== Starting new container ==='
docker run -d --name yourplace-gateway --restart unless-stopped --log-driver awslogs --log-opt awslogs-region=AWS_REGION_PLACEHOLDER --log-opt awslogs-group=/ec2/yourplace-gateway -p 443:443 -v /opt/YourPlace:/opt/YourPlace -e BASE_RPC_THROTTLE='BASE_RPC_THROTTLE_PLACEHOLDER' -e BASE_RPC_URL='BASE_RPC_URL_PLACEHOLDER' -e CDP_ONRAMP_KEY_NAME='CDP_ONRAMP_KEY_NAME_PLACEHOLDER' -e CDP_ONRAMP_PRIVATE_KEY='CDP_ONRAMP_PRIVATE_KEY_PLACEHOLDER' -e ETHEREUM_RPC_THROTTLE='ETHEREUM_RPC_THROTTLE_PLACEHOLDER' -e ETHEREUM_RPC_URL='ETHEREUM_RPC_URL_PLACEHOLDER' -e YOURPLACE_GATEWAY_CATCHUP='GATEWAY_CATCHUP_PLACEHOLDER' -e YOURPLACE_IPFS_PINNING_KEY='PINNING_KEY_PLACEHOLDER' -e YOURPLACE_IPFS_PINNING_TYPE='PINNING_TYPE_PLACEHOLDER' -e YOURPLACE_IPFS_PINNING_URL='PINNING_URL_PLACEHOLDER' -e YOURPLACE_ORIGIN='YOURPLACE_ORIGIN_PLACEHOLDER' -e YOURPLACE_SPOTIFY_CLIENT_ID='YOURPLACE_SPOTIFY_CLIENT_ID_PLACEHOLDER' $MYSQL_DSN_ENV ECR_REGISTRY_PLACEHOLDER/yourplace-gateway:latest
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
SCRIPT="${SCRIPT//GATEWAY_CATCHUP_PLACEHOLDER/$GATEWAY_CATCHUP}"
SCRIPT="${SCRIPT//YOURPLACE_ORIGIN_PLACEHOLDER/$YOURPLACE_ORIGIN}"
SCRIPT="${SCRIPT//YOURPLACE_SPOTIFY_CLIENT_ID_PLACEHOLDER/$YOURPLACE_SPOTIFY_CLIENT_ID}"
SCRIPT="${SCRIPT//BASE_RPC_THROTTLE_PLACEHOLDER/$BASE_RPC_THROTTLE}"
SCRIPT=$(echo "$SCRIPT" | sed "s|BASE_RPC_URL_PLACEHOLDER|${BASE_RPC_URL}|g")
SCRIPT="${SCRIPT//CDP_ONRAMP_KEY_NAME_PLACEHOLDER/$CDP_ONRAMP_KEY_NAME}"
SCRIPT="${SCRIPT//CDP_ONRAMP_PRIVATE_KEY_PLACEHOLDER/$CDP_ONRAMP_PRIVATE_KEY}"
SCRIPT="${SCRIPT//ETHEREUM_RPC_THROTTLE_PLACEHOLDER/$ETHEREUM_RPC_THROTTLE}"
SCRIPT=$(echo "$SCRIPT" | sed "s|ETHEREUM_RPC_URL_PLACEHOLDER|${ETHEREUM_RPC_URL}|g")
SCRIPT="${SCRIPT//PINNING_KEY_PLACEHOLDER/$IPFS_PINNING_SHARED_SECRET}"
SCRIPT="${SCRIPT//PINNING_TYPE_PLACEHOLDER/$YOURPLACE_IPFS_PINNING_TYPE}"
SCRIPT=$(echo "$SCRIPT" | sed "s|PINNING_URL_PLACEHOLDER|${YOURPLACE_IPFS_PINNING_URL}|g")

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
