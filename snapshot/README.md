# YourPlace Snapshot Service

A standalone blockchain indexing service for YourPlace that fetches and caches on-chain data from Base blockchain.

## Overview

The Snapshot Service runs independently from the main YourPlace server to:
- Index blockchain data every 2 minutes
- Export database snapshots every hour
- Store data in `~/YourPlaceSnapshot/`

## Configuration

Set these environment variables before running:

| Variable | Description | Default |
|----------|-------------|---------|
| `BASE_RPC_URL` | Base blockchain RPC endpoint URL | Public RPC (slow) |
| `BASE_RPC_THROTTLE` | Request throttle in seconds between RPC calls | 5 (slow) |

## Usage

```bash
# Build
make snapshot_build

# Run
make snapshot_run

# Run with debug logging
make snapshot_dbg_run
```

## Data Storage

- Database: `~/YourPlaceSnapshot/yourplacesnapshot.db`
- Snapshots: `~/YourPlaceSnapshot/snapshots/`
- Logs: `~/YourPlaceSnapshot/yourplacesnapshot.log`

## Command Line Flags

- `-d` - Enable debug mode for verbose logging