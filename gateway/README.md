# YourPlace Gateway Deployment

## Overview

YourPlace Gateway runs on AWS EC2 using Docker containers. The gateway binary is built without desktop UI dependencies for headless deployment.

## Build

Build gateway binary (headless, no GTK dependencies):
```bash
cd /Users/nops/Code/YourPlace/Server
make gateway_build
```

Binary location: `target/YourPlace-<version>`

## Local Testing

Run gateway locally:
```bash
./target/YourPlace-0.1.0 -g -d -du
```

Flags:
- `-g` - Gateway mode
- `-d` - Debug logging
- `-du` - Disable UI (don't open browser)

## Docker

### Build Container
```bash
docker build -f gateway/Dockerfile -t yourplace-gateway .
```

### Run Container
```bash
docker run -d \
  --name yourplace-gateway \
  --restart unless-stopped \
  -p 443:443 \
  -v /opt/YourPlace:/opt/YourPlace \
  yourplace-gateway
```

### Inspect Logs
```bash
docker logs yourplace-gateway
docker logs -f yourplace-gateway  # Follow logs
```

### Container Shell
```bash
docker exec -it yourplace-gateway /bin/bash
```

## AWS Deployment

### Prerequisites

#### GitHub Secrets
Configure the following secrets in your GitHub repository:

**Settings → Secrets and variables → Actions → New repository secret**

- `CLOUDFLARE_CERT_PEM` - Cloudflare origin certificate (full PEM content)
- `CLOUDFLARE_CERT_KEY` - Cloudflare origin private key (full key content)
- `GATEWAY_SSH_PUBLIC_KEY` - SSH public key for EC2 access

#### TLS Certificates

Certificates are automatically deployed to `/opt/YourPlace/` on the EC2 instance during GitHub Actions workflow execution.

To manually add/update certificates:
```bash
# On EC2 instance
mkdir -p /opt/YourPlace
echo "$CERT_PEM_CONTENT" > /opt/YourPlace/cert.pem
echo "$CERT_KEY_CONTENT" > /opt/YourPlace/cert.key
chmod 644 /opt/YourPlace/cert.pem
chmod 600 /opt/YourPlace/cert.key
```

### Deploy via GitHub Actions

Trigger deployment:
```bash
# Via GitHub UI: Actions → Gateway → Run workflow
# Or via GitHub CLI:
gh workflow run gateway.yml
```

The workflow:
1. Builds Docker image with pre-built binary from DigitalOcean Spaces
2. Pushes image to AWS ECR
3. Deploys certificates to `/opt/YourPlace/` on EC2
4. Pulls and runs latest container

### SSH to EC2 Instance
```bash
aws ssm start-session --target <instance-id> --region us-east-1
```

### Check Container Status
```bash
# On EC2 instance
docker ps
docker logs yourplace-gateway
docker logs -f yourplace-gateway
```

### Manual Container Restart
```bash
# On EC2 instance
docker stop yourplace-gateway
docker rm yourplace-gateway
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin <ecr-registry>
docker pull <ecr-registry>/yourplace-gateway:latest
docker run -d --name yourplace-gateway --restart unless-stopped \
  -p 443:443 -v /opt/YourPlace:/opt/YourPlace \
  <ecr-registry>/yourplace-gateway:latest
```

### Verify TLS Certificates
```bash
# On EC2 instance
ls -la /opt/YourPlace/
# Should show cert.pem (644) and cert.key (600)

# Check certificate validity
openssl x509 -in /opt/YourPlace/cert.pem -text -noout
```

## Gateway Mode Features

- Data directory: `/opt/YourPlace/` (when `YourPlaceGateway=true`)
- TLS server on port 443 (optional - requires certificates)
- Trusts Cloudflare proxy headers
- Disables file uploads, settings changes, and setup endpoints
- No desktop systray (headless)
- Static binary (CGO disabled)

## Troubleshooting

### Check binary dependencies
```bash
ldd target/YourPlace-0.1.0
# Should only show libc, libpthread, etc. - no GTK
```

### Verify build tags
```bash
go version -m target/YourPlace-0.1.0 | grep build
# Should show: -tags=gateway
```

### Container not starting
```bash
docker logs yourplace-gateway
# Check for port conflicts or permission issues
```

### TLS not working
```bash
# Check if certificates exist
ls -la /opt/YourPlace/cert.{pem,key}

# Check gateway logs for TLS warnings
docker logs yourplace-gateway 2>&1 | grep -i tls

# Gateway will run without TLS if certificates are missing (HTTP only on port 42424)
```
