# YourPlace Snapshot Service

A standalone blockchain indexing service for YourPlace that fetches and caches on-chain data from Base blockchain.

## Overview

The Snapshot Service runs independently from the main YourPlace server to:
- Index blockchain data every 2 minutes
- Export database snapshots every hour
- Upload snapshots to S3-compatible storage
- Store data in `~/YourPlaceSnapshot/`

## Deployment Architecture

### Infrastructure (Infra Repository)
The infrastructure is managed via Terraform in the `Infra` repository at `terraform/projects/snapshot/`:
- **ECS Fargate** - Runs containerized snapshot service
- **ECR** - Stores Docker images
- **S3** - Stores exported database snapshots (auto-cleanup after 10 versions)
- **EFS** - Persistent storage for `/root/YourPlaceSnapshot` data
- **IAM Roles** - ECS execution + task role with S3 permissions
- **GitHub OIDC** - Enables CI/CD deployment without AWS credentials

### Continuous Deployment (GitHub Actions)
Deployments are automated via `.github/workflows/snapshot.yml`:
1. Triggered on push to `main` when `snapshot/**` files change
2. Builds Docker image from repository code
3. Pushes image to ECR with commit SHA and `latest` tags
4. Registers new ECS task definition with environment variables from GitHub secrets
5. Forces ECS service redeployment
6. Waits for service stability before completing

## Configuration

### Environment Variables

| Variable | Description | Source | Required |
|----------|-------------|--------|----------|
| `BASE_RPC_URL` | Base blockchain RPC endpoint URL with credentials | GitHub Secret | Yes |
| `BASE_RPC_THROTTLE` | Request throttle in seconds between RPC calls | GitHub Variable | No (default: 5) |
| `S3_BUCKET_NAME` | S3 bucket name for snapshot uploads | Terraform output | Yes (auto-set) |
| `S3_ENDPOINT` | S3 endpoint URL | Terraform output | Yes (auto-set) |
| `S3_ACCESS_KEY` | S3 access key (optional for DigitalOcean Spaces) | Manual | No |
| `S3_SECRET_KEY` | S3 secret key (optional for DigitalOcean Spaces) | Manual | No |

**Note**: When running on AWS ECS, S3 credentials are provided automatically via IAM role. Manual credentials are only needed for non-AWS S3-compatible storage (e.g., DigitalOcean Spaces).

### GitHub Setup

1. **Add Repository Secrets** (Settings → Secrets → Actions):
   - `BASE_RPC_URL` - Your Base RPC endpoint (e.g., from Alchemy, Infura, QuickNode)

2. **Add Repository Variables** (Settings → Variables → Actions):
   - `BASE_RPC_THROTTLE` - Default: `"5"` (lower for paid RPCs: `"1"` or `"2"`)

## Deployment Guide

### Initial Setup (One-Time)

1. **Configure GitHub Secrets/Variables** (see above)

2. **Deploy Infrastructure**:
```bash
cd /path/to/Infra
make snapshot-apply
```

3. **Trigger First Deployment**:
```bash
cd /path/to/Server
git push origin main  # Push changes to snapshot/ directory
```

### Updating the Service

Any changes to `snapshot/` automatically deploy when pushed to `main`. Manual deployments can be triggered via:
- GitHub Actions tab → "Snapshot Service" → "Run workflow"

### Infrastructure Updates

To modify AWS resources (CPU, memory, S3 settings, etc.):
```bash
cd /path/to/Infra/terraform/projects/snapshot
# Edit main.tf, variables.tf, etc.
make snapshot-plan   # Review changes
make snapshot-apply  # Apply changes
```

## Local Development

### Usage

```bash
# Build
make snapshot_build

# Run
make snapshot_run

# Run with debug logging
make snapshot_dbg_run
```

### Data Storage

- Database: `~/YourPlaceSnapshot/yourplacesnapshot.sqlite.db`
- Snapshots: `~/YourPlaceSnapshot/snapshots/`
- Logs: `~/YourPlaceSnapshot/yourplacesnapshot.log`

### Command Line Flags

- `-d` - Enable debug mode for verbose logging

## Architecture Notes

### S3 Upload Strategy
- Snapshots uploaded every hour after database export
- Keeps last 10 snapshot versions (automatically deletes older ones)
- Uses IAM role credentials on AWS ECS (no keys needed)
- Supports static credentials for DigitalOcean Spaces or other S3-compatible storage

### Container Build
- Multi-stage Dockerfile using Ubuntu 24.04
- Builds from local repository code (not GitHub clone)
- Go 1.25.1 for compilation
- Final image includes only binary and runtime dependencies