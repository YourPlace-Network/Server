# Blockchain Indexer Architecture

The blockchain indexer (`src/core/db/blockchain/indexer.go`) is YourPlace's core component for processing on-chain data from blockchain networks and storing YourPlace-specific transactions in the local database.

## Overview

The indexer scans blockchain networks (primarily Base, with support for Algorand and Ethereum) to identify and process YourPlace protocol transactions, extracting posts, metadata, follows, and other social media interactions encoded in transaction data.

## Architecture Components

### 1. Job Management System

**Job States**: `pending`, `running`, `complete`, `failed`

**Job Types**:
- **FullFill**: Complete blockchain scan from latest to earliest block (initial sync)
- **FrontFill**: Scan from last processed block to latest block (incremental sync)  
- **BackFill**: Scan from earliest processed block backward to fill gaps

### 2. Core Data Structures

#### SequentialBlockTracker
```go
type SequentialBlockTracker struct {
    mu                sync.Mutex
    processedBlocks   map[int64]bool
    nextExpectedBlock int64
    uuid              string
    database          *db.Database
    direction         string
}
```
Ensures blocks are processed sequentially even when received out-of-order from parallel workers.

#### RequestTracker
```go
type RequestTracker struct {
    mu           sync.Mutex
    requestTimes []time.Time
    windowSize   time.Duration
}
```
Monitors RPC request rates for dynamic throttling and rate limit compliance.

### 3. Worker Pool Architecture

- **Worker Count**: 10 concurrent threads by default
- **Batch Processing**: Up to 25 blocks per RPC batch call
- **Rate Limiting**: Dynamic throttling based on RPC node limits
- **Error Handling**: Exponential backoff with 120 retry limit

### 4. Dynamic Throttle Control

The indexer implements adaptive rate limiting:
- Monitors actual vs. target request rates
- Adjusts multiplier (0.1x to 100x) based on performance
- Prevents rate limit violations while maximizing throughput
- Updates every 30 seconds with 10% tolerance

## Protocol Transaction Processing

### YourPlace Protocol Format
```
yp/{version}/{action}:{payload}
```

**Supported Actions (v1)**:
- `p`: Post creation
- `pa`: Post with attachments
- `f`: Follow user
- `fu`: Unfollow user
- `mn`: Set display name
- `ma`: Set avatar
- `mb`: Set banner
- `mbd`: Set birthdate
- `ml`: Set location
- `mw`: Set website
- `md`: Set description

### Transaction Validation

Each transaction undergoes:
1. **Protocol Format Validation**: Regex matching against YourPlace protocol
2. **Payload Parsing**: JSON unmarshaling and type checking
3. **Security Validation**: Address validation, sanitization, fraud prevention
4. **History Filtering**: Timestamp-based expiration (configurable history days)

## Key Constants & Configuration

```go
const (
    reportInterval = 5000  // Progress reporting frequency
    saveInterval   = 100   // Database save frequency
    throttleOffset = 4     // RPC headroom for frontend requests
    batchSizeLimit = 25    // Max blocks per batch
    workerCount    = 10    // Parallel worker threads
)
```

## Operational Flow

### 1. Preflight Checks
- Verify indexer is enabled (`indexerRunning` setting)
- Check for existing jobs or create new job UUID
- Validate blockchain connectivity
- Set throttle defaults for known nodes

### 2. Job Dispatch
Based on previous job status:
- **Pending/New**: Start FullFill from latest block
- **Failed**: Resume from last checkpoint
- **Complete**: Run FrontFill for new blocks

### 3. Block Processing
1. Calculate optimal batch size based on throttle settings
2. Create worker pool with staggered startup
3. Queue block batches for processing  
4. Workers make batch RPC calls with rate limiting
5. Process transactions and update sequential tracker
6. Save progress at configured intervals

### 4. Error Handling
- RPC failures trigger exponential backoff
- Worker errors propagated to main thread
- Job marked as `failed` for recovery on restart
- Graceful cancellation via `indexerCancel` channel

## Database Integration

The indexer calls specific database methods for each transaction type:
- `OnchainP()`: Store posts
- `OnchainPA()`: Store posts with attachments  
- `OnchainF()/OnchainFU()`: Store follow/unfollow actions
- `OnchainMN/MA/MB/etc()`: Store metadata updates

## Performance Optimizations

1. **Batch RPC Calls**: Multiple blocks per request
2. **Parallel Processing**: 10 concurrent workers
3. **Dynamic Rate Limiting**: Adaptive throttling
4. **Sequential Tracking**: Out-of-order completion handling
5. **Memory Efficient**: Streaming block processing
6. **Checkpoint System**: Regular progress saves

## Monitoring & Logging

- Progress reporting every 5,000 blocks
- Request rate monitoring and adjustment logging
- Detailed debug logging for transaction processing
- Error tracking with retry counts and backoff timing

## Custom Blockchain Extensions

The architecture supports additional blockchains through:
- Pluggable RPC client interfaces
- Blockchain-specific earliest block detection
- Chain-specific transaction format handlers
- Independent job tracking per blockchain

This indexer design ensures reliable, efficient processing of YourPlace social media data across multiple blockchain networks while maintaining data integrity and system performance.