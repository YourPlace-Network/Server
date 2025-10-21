#!/bin/bash
set -e

AWS_REGION="${AWS_REGION:-us-east-1}"

if [ -z "$INSTANCE_ID" ] || [ -z "$ECR_REGISTRY" ] || [ -z "$SSH_PUBLIC_KEY" ] || [ -z "$CLOUDFLARE_CERT_PEM" ] || [ -z "$CLOUDFLARE_CERT_KEY" ]; then
  echo "ERROR: Missing required environment variables"
  echo "Required: INSTANCE_ID, ECR_REGISTRY, SSH_PUBLIC_KEY, CLOUDFLARE_CERT_PEM, CLOUDFLARE_CERT_KEY"
  exit 1
fi

echo "Encoding certificates..."
CERT_PEM_BASE64=$(echo "$CLOUDFLARE_CERT_PEM" | base64 -w 0 2>/dev/null || echo "$CLOUDFLARE_CERT_PEM" | base64)
CERT_KEY_BASE64=$(echo "$CLOUDFLARE_CERT_KEY" | base64 -w 0 2>/dev/null || echo "$CLOUDFLARE_CERT_KEY" | base64)

echo "Building deployment command..."
COMMAND_JSON=$(cat <<EOF
{
  "commands": [
    "set -e",
    "echo '=== Configuring SSH key ==='",
    "mkdir -p /root/.ssh",
    "echo '$SSH_PUBLIC_KEY' > /root/.ssh/authorized_keys",
    "chmod 600 /root/.ssh/authorized_keys",
    "chmod 700 /root/.ssh",
    "echo '=== Installing TLS certificates ==='",
    "mkdir -p /opt/YourPlace",
    "echo '$CERT_PEM_BASE64' | base64 -d > /opt/YourPlace/cert.pem",
    "echo '$CERT_KEY_BASE64' | base64 -d > /opt/YourPlace/cert.key",
    "chmod 644 /opt/YourPlace/cert.pem",
    "chmod 600 /opt/YourPlace/cert.key",
    "echo '=== Stopping existing container ==='",
    "docker stop yourplace-gateway 2>/dev/null || echo 'No existing container'",
    "docker rm yourplace-gateway 2>/dev/null || echo 'No existing container'",
    "echo '=== Logging into ECR ==='",
    "aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $ECR_REGISTRY",
    "echo '=== Pulling latest image ==='",
    "docker pull $ECR_REGISTRY/yourplace-gateway:latest",
    "echo '=== Starting new container ==='",
    "docker run -d --name yourplace-gateway --restart unless-stopped -p 443:443 -v /opt/YourPlace:/opt/YourPlace $ECR_REGISTRY/yourplace-gateway:latest",
    "echo '=== Verifying container ==='",
    "docker ps | grep yourplace-gateway",
    "echo '=== Deployment complete ==='"
  ]
}
EOF
)

echo "Sending deployment command to instance $INSTANCE_ID..."
COMMAND_ID=$(aws ssm send-command \
  --instance-ids "$INSTANCE_ID" \
  --document-name "AWS-RunShellScript" \
  --parameters "$COMMAND_JSON" \
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

echo ""
echo "=== Output ==="
echo "$RESULT" | jq -r '.StandardOutputContent'

if [ -n "$(echo "$RESULT" | jq -r '.StandardErrorContent')" ]; then
  echo ""
  echo "=== Errors ==="
  echo "$RESULT" | jq -r '.StandardErrorContent'
fi

if [ "$STATUS" != "Success" ]; then
  echo ""
  echo "ERROR: Deployment failed"
  exit 1
fi

echo ""
echo "SUCCESS: Gateway deployed successfully"
