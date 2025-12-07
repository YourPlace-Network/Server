package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/time/rate"
)

// 0 --- Earliest --- Tail --- Head --- Latest
// Job States - Running, Complete, Failed, Pending

const (
	burnAddressETH       = "0x0000000000000000000000000000000000000000"
	entryPointV06Address = "0x5ff137d4b0fdcd49dca30c7cf57e578a026d2789"
	handleOpsSelector    = "1fad948c"
	executeBatchSelector = "34fcd5be"
	reportInterval       = 5000
	saveInterval         = 100
	throttleOffset       = 4
	batchSizeLimit       = 25
	workerCount          = 10
)

var (
	indexerCancel             chan bool
	IndexerMutex              sync.Mutex
	IsIndexing                bool
	_Blockchain               *Blockchain
	_Database                 *db.Database
	dynamicThrottleMultiplier = 1.0
	throttleControlMutex      sync.RWMutex    // Protects dynamicThrottleMultiplier
	globalRequestTracker      *RequestTracker // Global request tracker to monitor request rates across all workers
	rateLimiterMutex          sync.Mutex      // Serializes rate limiter token acquisition across workers
	activeRequestsCount       int64           // Atomic counter for currently active RPC requests across all workers
	progressLogMutex          sync.Mutex      // Prevents duplicate progress logs from multiple workers
	lastProgressBlock         int64           // Last block number we logged progress for
	totalRequestsCount        int64           // Atomic counter for total RPC requests processed across all workers
)

type SequentialBlockTracker struct {
	mu                sync.RWMutex
	processedBlocks   map[int64]bool
	nextExpectedBlock int64
	uuid              string
	database          *db.Database
	direction         string
}
type RequestTracker struct {
	mu           sync.Mutex
	requestTimes []time.Time
	windowSize   time.Duration
}
type smartWalletPayload struct {
	fromAddress string
	toAddress   string
	payload     string
}
type userOperation struct { // userOperation represents an ERC-4337 UserOperation structure
	sender               []byte
	nonce                *big.Int
	initCode             []byte
	callData             []byte
	callGasLimit         *big.Int
	verificationGasLimit *big.Int
	preVerificationGas   *big.Int
	maxFeePerGas         *big.Int
	maxPriorityFeePerGas *big.Int
	paymasterAndData     []byte
	signature            []byte
}

func NewSequentialBlockTracker(startBlock int64, uuid string, database *db.Database, direction string) *SequentialBlockTracker {
	return &SequentialBlockTracker{
		processedBlocks:   make(map[int64]bool),
		nextExpectedBlock: startBlock,
		uuid:              uuid,
		database:          database,
		direction:         direction,
	}
}
func (sbt *SequentialBlockTracker) MarkBlockProcessed(blockNumber int64, direction string) {
	sbt.mu.Lock()
	defer sbt.mu.Unlock()
	sbt.processedBlocks[blockNumber] = true
	// Update nextExpectedBlock to the next unprocessed sequential block
	if direction == "forward" {
		for sbt.processedBlocks[sbt.nextExpectedBlock] {
			delete(sbt.processedBlocks, sbt.nextExpectedBlock)
			sbt.nextExpectedBlock++
			if sbt.nextExpectedBlock%saveInterval == 0 {
				sbt.database.IndexerUpdateHeadBlock(sbt.uuid, uint64(sbt.nextExpectedBlock))
			}
		}
	} else {
		for sbt.processedBlocks[sbt.nextExpectedBlock] {
			delete(sbt.processedBlocks, sbt.nextExpectedBlock)
			sbt.nextExpectedBlock--
			if sbt.nextExpectedBlock%saveInterval == 0 {
				sbt.database.IndexerUpdateTailBlock(sbt.uuid, uint64(sbt.nextExpectedBlock))
			}
		}
	}
}
func (sbt *SequentialBlockTracker) GetNextExpectedBlock() int64 {
	sbt.mu.RLock()
	defer sbt.mu.RUnlock()
	return sbt.nextExpectedBlock
}
func (sbt *SequentialBlockTracker) HasPendingBlocks() bool {
	sbt.mu.RLock()
	defer sbt.mu.RUnlock()
	return len(sbt.processedBlocks) > 0
}

func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		requestTimes: make([]time.Time, 0),
		windowSize:   time.Minute, // Track requests over the last minute
	}
}
func (rt *RequestTracker) RecordRequests(count int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rt.windowSize)
	rt.cleanupOldRequests(cutoff)
	// Add the current time for each request
	for i := 0; i < count; i++ {
		rt.requestTimes = append(rt.requestTimes, now)
	}
}
func (rt *RequestTracker) GetRequestsPerSecond() float64 {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rt.windowSize) // Look at the last N-seconds for more responsive measurement
	// Count requests in the last minute
	validRequests := 0
	for _, requestTime := range rt.requestTimes {
		if requestTime.After(cutoff) {
			validRequests++
		}
	}
	return float64(validRequests) / rt.windowSize.Seconds()
}
func (rt *RequestTracker) cleanupOldRequests(cutoff time.Time) {
	writeIndex := 0
	for readIndex, requestTime := range rt.requestTimes {
		if requestTime.After(cutoff) {
			rt.requestTimes[writeIndex] = rt.requestTimes[readIndex]
			writeIndex++
		}
	}
	rt.requestTimes = rt.requestTimes[:writeIndex] // Trim the slice to the new length
}

// --- Indexer Main Method --- //
func BaseIndexerFetchData(database *db.Database, blockchain *Blockchain) bool {
	chainName := "base"
	_Blockchain = blockchain
	_Database = database
	databaseStatus, uuid, chainLatestBlock, databaseTailBlock, databaseHeadBlock, chainEarliestBlock := indexerPreflight(chainName)
	if uuid == "" || databaseStatus == "" {
		return false // bail out if the preflight bails out (indexer already running or other issue)
	}
	_ = databaseHeadBlock
	switch databaseStatus { // Post fill job dispatch, based on last job status
	case "pending":
		core.LogDebug("[Base] Starting pending job from the beginning")
		IndexerBaseFullFill(blockchain.Base, uuid, chainLatestBlock, database)
	case "failed":
		core.LogDebug("[Base] Restarting failed job from where it left off")
		if databaseTailBlock == 0 { // If a full fill job started, but failed before the tail block was written, start all over
			BaseIndexerRestartJobs(_Database, chainName)
			IndexerBaseFullFill(blockchain.Base, uuid, chainLatestBlock, database)
			return true
		}
		if databaseTailBlock > chainEarliestBlock.Uint64() { // if a backfill job failed, restart it
			IndexerBaseBackFill(blockchain.Base, uuid, chainLatestBlock, database)
		} else { // if a front fill job failed, restart it
			IndexerBaseFrontFill(blockchain.Base, uuid, chainLatestBlock, database)
		}
	case "complete": // If everything is backfilled, then just process the newest blocks
		core.LogDebug("[Base] Last job completed successfully. Getting new blocks")
		IndexerBaseFrontFill(blockchain.Base, uuid, chainLatestBlock, database)
	}
	return true
}

// --- Base Indexer Functions --- //
func IndexerBaseFrontFill(base *Base, uuid string, baseLatestBlock *big.Int, database *db.Database) {
	// (old) head block <----- latest block (starting traversal @ latest block)
	// head block -----> latest block (starting traversal @ head block)
	core.LogDebug("[Base] --- IndexerBaseFrontFill()")
	direction := "forward"
	_Database.IndexerUpdateJobStatus(uuid, "running")
	headBlock := _Database.IndexerGetHeadBlock(uuid)
	if headBlock <= 0 {
		core.LogWarn("[Base] IndexerBaseFrontFill(): Head block is <= 0 - aborting")
		return
	}
	targetLatestBlock := baseLatestBlock
	targetEarliestBlock := headBlock
	core.LogDebug("[Base] Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("[Base] Target Earliest Block: " + strconv.Itoa(int(targetEarliestBlock)))
	targetEarliestBlockBigInt := big.NewInt(int64(targetEarliestBlock))
	databaseHistoryDaysInt, _ := strconv.Atoi(_Database.SettingsGetValue("historyDays"))
	baseThrottle, _ := strconv.Atoi(_Database.SettingsGetValue("baseThrottle"))
	core.LogDebug("[Base] Throttle: " + strconv.Itoa(baseThrottle))
	batchSize := calculateOptimalBatchSize(baseThrottle)
	core.LogDebug("[Base] Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, targetEarliestBlockBigInt) // figure out how many blocks we need to fetch
	if blockCount.Int64() <= 0 {
		core.LogError("[Base] Block count is negative or zero")
		return
	}
	core.LogDebug("[Base] Number of Blocks: " + blockCount.String())
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("[Base] Batch Count: " + batchCount.String())
	rateLimiter := configureRateLimiter(baseThrottle)
	batchStartBlock := new(big.Int).Set(targetEarliestBlockBigInt) // Start at the earliest block
	txnCount := core.NewThreadSafeCounter()
	batchJobQueue := core.NewThreadSafeQueue()
	sequentialTracker := NewSequentialBlockTracker(targetEarliestBlockBigInt.Int64(), uuid, _Database, direction)
	globalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, workerCount)
	// Reset atomic counters for new indexing session
	atomic.StoreInt64(&activeRequestsCount, 0)
	atomic.StoreInt64(&totalRequestsCount, 0)
	atomic.StoreInt64(&lastProgressBlock, 0)

	go startThrottleController(uuid, baseThrottle, rateLimiter, database) // Start the throttle controller in a separate goroutine

	// Start worker threads to process the batch jobs
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			// Stagger worker startup: 500ms base + up to 500ms jitter = 500ms-1000ms total
			baseDelay := time.Duration(workerID) * 500 * time.Millisecond
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			time.Sleep(baseDelay + jitter)
			defer wg.Done()
			err := workerThread(uuid, rateLimiter, base, batchJobQueue, sequentialTracker, globalRequestTracker, txnCount, databaseHistoryDaysInt, targetEarliestBlockBigInt, targetLatestBlock, batchSize, "forward")
			if err != nil {
				core.LogError("[Base] Worker thread failed: " + err.Error())
				errorChan <- err // Send the error to the error channel
			}
		}(i)
	}
	// Producer goroutine: Feed batches to queue without loading everything into memory
	go func() {
		for i := 1; i <= int(batchCount.Int64()); i++ { // Loop over batches of blocks
			if breakPoint(uuid) {
				return
			}
			batchEndBlock := new(big.Int).Add(batchStartBlock, batchSize)
			if batchEndBlock.Cmp(targetLatestBlock) == 1 { // Stop at the latest block allowed
				batchEndBlock = targetLatestBlock
			}
			if batchStartBlock.Cmp(batchEndBlock) >= 0 { // Break if the start block is ahead of or equal to the end block
				break
			}
			// Batch up blocks into one RPC call
			var batchBlockNumbers []big.Int
			for j := new(big.Int).Set(batchStartBlock); j.Cmp(batchEndBlock) == -1; j = new(big.Int).Add(j, big.NewInt(1)) {
				batchBlockNumbers = append(batchBlockNumbers, *j)
			}
			batchJobQueue.Enqueue(batchBlockNumbers)
			batchStartBlock = new(big.Int).Add(batchStartBlock, batchSize) // Move to the next batch
		}
	}()
	// Wait for completion or error
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done) // Close the done channel when all workers are done
	}()
	select {
	case err := <-errorChan:
		core.LogError("[Base] Worker thread failed: " + err.Error())
		_Database.IndexerUpdateJobStatus(uuid, "failed")
		return
	case <-done:
		break
	}
	// Update final status
	finalBlockIndex := sequentialTracker.GetNextExpectedBlock()
	_Database.IndexerUpdateHeadBlock(uuid, uint64(finalBlockIndex))
	_Database.IndexerUpdateJobStatus(uuid, "complete")
}
func IndexerBaseBackFill(base *Base, uuid string, baseLatestBlock *big.Int, database *db.Database) {
	// earliest block <----- tail block (starting traversal @ tail block)—
	core.LogDebug("[Base] --- IndexerBaseBackFill() ---")
	direction := "backward"
	_Database.IndexerUpdateJobStatus(uuid, "running")
	databaseTailBlock := big.NewInt(int64(_Database.IndexerGetTailBlock(uuid)))
	if databaseTailBlock.Cmp(big.NewInt(0)) == 0 { // if databaseTailBlock = 0, then the job is new
		core.LogDebug("[Base] Database Tail Block is 0 - setting to Head Block")
		headBlockInt := _Database.IndexerGetHeadBlock(uuid)
		databaseTailBlock = big.NewInt(int64(headBlockInt))
		core.LogDebug("[Base] Database Tail Block: " + databaseTailBlock.String())
	}
	targetLatestBlock := databaseTailBlock
	targetEarliestBlock := &base.EarliestBlock
	if targetLatestBlock.Cmp(targetEarliestBlock) == 0 {
		core.LogDebug("[Base] Target latest block is equal to target earliest block - completing")
		_Database.IndexerUpdateJobStatus(uuid, "complete")
		return
	}
	if targetLatestBlock.Int64() == 0 { // if targetLatestBlock = 0, then the job is new
		targetLatestBlock = baseLatestBlock // set the widest possible target for a new backfill
		targetEarliestBlock = &base.EarliestBlock
	}
	core.LogDebug("[Base] Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("[Base] Target Earliest Block: " + targetEarliestBlock.String())
	targetEarliestBlockBigInt := targetEarliestBlock
	databaseHistoryDaysInt, _ := strconv.Atoi(_Database.SettingsGetValue("historyDays"))
	baseThrottle, _ := strconv.Atoi(_Database.SettingsGetValue("baseThrottle"))
	core.LogDebug("[Base] Throttle: " + strconv.Itoa(baseThrottle))
	batchSize := calculateOptimalBatchSize(baseThrottle)
	core.LogDebug("[Base] Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, targetEarliestBlockBigInt) // figure out how many blocks we need to fetch
	core.LogDebug("[Base] Block Count: " + blockCount.String())
	if blockCount.Int64() <= 0 {
		core.LogError("[Base] Block count is negative or zero")
		return
	}
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("[Base] Batch Count: " + batchCount.String())
	rateLimiter := configureRateLimiter(baseThrottle)                                                     // Configure rate limiter based on throttle and batch size
	batchStartBlock := targetLatestBlock                                                                  // Start at the latest block
	txnCount := core.NewThreadSafeCounter()                                                               // Count the number of transactions processed
	batchJobQueue := core.NewThreadSafeQueue()                                                            // Queue to hold batch jobs
	sequentialTracker := NewSequentialBlockTracker(targetLatestBlock.Int64(), uuid, _Database, direction) // Sequential block tracker starting from the highest block
	globalRequestTracker = NewRequestTracker()                                                            // Request tracker to monitor request rates
	errorChan := make(chan error, workerCount)                                                            // Channel to handle errors from workers
	// Reset atomic counters for a new indexing session
	atomic.StoreInt64(&activeRequestsCount, 0)
	atomic.StoreInt64(&totalRequestsCount, 0)

	// Start the throttle controller in a separate goroutine
	go startThrottleController(uuid, baseThrottle, rateLimiter, database)

	// Start worker threads to process the batch jobs
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1) // Add a worker to the wait group
		go func(workerID int) {
			// Stagger worker startup: 500ms base + up to 500ms jitter = 500ms-1000ms total
			baseDelay := time.Duration(workerID) * 500 * time.Millisecond
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			time.Sleep(baseDelay + jitter)
			defer wg.Done()
			err := workerThread(uuid, rateLimiter, base, batchJobQueue, sequentialTracker, globalRequestTracker, txnCount, databaseHistoryDaysInt, targetEarliestBlock, targetLatestBlock, batchSize, "backward")
			if err != nil {
				core.LogError("[Base] Worker encountered an error: " + err.Error())
				errorChan <- err // Send the error to the error channel
			}
		}(i)
	}
	// Producer goroutine: Feed batches to queue without loading everything into memory
	go func() {
		for i := 1; i <= int(batchCount.Int64()); i++ { // Loop over batches of blocks
			if breakPoint(uuid) {
				return
			}
			batchEndBlock := new(big.Int).Sub(batchStartBlock, batchSize)
			if batchEndBlock.Cmp(targetEarliestBlockBigInt) == -1 { // stop at the earliest block allowed
				batchEndBlock = targetEarliestBlockBigInt
			}
			if batchStartBlock.Cmp(batchEndBlock) <= 0 { // break if the start block is behind the end block
				break
			}
			// Batch up blocks
			var batchBlockNumbers []big.Int
			for j := batchStartBlock; j.Cmp(batchEndBlock) == 1; j = new(big.Int).Sub(j, big.NewInt(1)) {
				batchBlockNumbers = append(batchBlockNumbers, *j)
			}
			batchJobQueue.Enqueue(batchBlockNumbers)                       // Add the batch of blocks to the queue
			batchStartBlock = new(big.Int).Sub(batchStartBlock, batchSize) // Move to the next batch
		}
	}()
	// Wait for completion or error
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done) // Close the done channel when all workers are done
	}()
	select {
	case err := <-errorChan:
		core.LogError("[Base] Worker thread failed: " + err.Error())
		_Database.IndexerUpdateJobStatus(uuid, "failed")
		return
	case <-done:
		break
	}

	// Update final status
	finalBlockIndex := sequentialTracker.GetNextExpectedBlock()
	_Database.IndexerUpdateTailBlock(uuid, uint64(finalBlockIndex))
	_Database.IndexerUpdateJobStatus(uuid, "complete")
}
func IndexerBaseFullFill(base *Base, uuid string, baseLatestBlock *big.Int, database *db.Database) {
	// earliest block <----- latest block (starting traversal @ latest block)
	core.LogDebug("[Base] --- IndexerBaseFullFill()")
	direction := "backward"
	_Database.IndexerUpdateJobStatus(uuid, "running")
	targetLatestBlock := baseLatestBlock
	core.LogDebug("[Base] Target Latest Block: " + targetLatestBlock.String())
	targetEarliestBlock := BaseGetEarliestBlock()
	core.LogDebug("[Base] Target Earliest Block: " + targetEarliestBlock.String())
	targetEarliestBlockBigInt := targetEarliestBlock
	_Database.IndexerUpdateHeadBlock(uuid, targetLatestBlock.Uint64())
	databaseHistoryDaysInt, _ := strconv.Atoi(_Database.SettingsGetValue("historyDays"))
	baseThrottle, _ := strconv.Atoi(_Database.SettingsGetValue("baseThrottle"))
	core.LogDebug("[Base] Throttle: " + strconv.Itoa(baseThrottle))
	batchSize := calculateOptimalBatchSize(baseThrottle)
	core.LogDebug("[Base] Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, &targetEarliestBlockBigInt) // figure out how many blocks we need to fetch
	if blockCount.Int64() <= 0 {
		core.LogError("[Base] Block count is negative or zero")
		return
	}
	core.LogDebug("[Base] Number of Blocks: " + blockCount.String())
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("[Base] Batch Count: " + batchCount.String())
	rateLimiter := configureRateLimiter(baseThrottle)                                                     // Configure a rate limiter based on throttle and batch size
	batchStartBlock := targetLatestBlock                                                                  // Start at the latest block
	txnCount := core.NewThreadSafeCounter()                                                               // Count the number of transactions processed
	batchJobQueue := core.NewThreadSafeQueue()                                                            // Queue to hold batch jobs
	sequentialTracker := NewSequentialBlockTracker(targetLatestBlock.Int64(), uuid, _Database, direction) // Sequential block tracker starting from the highest block
	globalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, workerCount) // Channel to handle errors from workers
	// Reset atomic counters for new indexing session
	atomic.StoreInt64(&activeRequestsCount, 0)
	atomic.StoreInt64(&totalRequestsCount, 0)
	atomic.StoreInt64(&lastProgressBlock, 0)

	go startThrottleController(uuid, baseThrottle, rateLimiter, database) // Start the throttle controller in a separate goroutine

	// Start worker threads to process the batch jobs
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			// Stagger worker startup: 500ms base + up to 500ms jitter = 500ms-1000ms total
			baseDelay := time.Duration(workerID) * 500 * time.Millisecond
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			time.Sleep(baseDelay + jitter)
			defer wg.Done()
			err := workerThread(uuid, rateLimiter, base, batchJobQueue, sequentialTracker, globalRequestTracker, txnCount, databaseHistoryDaysInt, &targetEarliestBlockBigInt, targetLatestBlock, batchSize, "backward")
			if err != nil {
				core.LogError("[Base] Worker encountered an error: " + err.Error())
				errorChan <- err
			}
		}(i)
	}
	// Producer goroutine: Feed batches to queue without loading everything into memory
	go func() {
		for i := 1; i <= int(batchCount.Int64()); i++ { // Loop over batches of blocks
			if breakPoint(uuid) {
				return
			}
			batchEndBlock := new(big.Int).Sub(batchStartBlock, batchSize)
			if batchEndBlock.Cmp(&targetEarliestBlockBigInt) == -1 { // stop at the earliest block allowed
				batchEndBlock = &targetEarliestBlockBigInt
			}
			if batchStartBlock.Cmp(batchEndBlock) < 0 { // break if the start block is behind the end block
				break
			}
			// Batch up blocks into one RPC call
			var batchBlockNumbers []big.Int
			for j := batchStartBlock; j.Cmp(batchEndBlock) == 1; j = new(big.Int).Sub(j, big.NewInt(1)) {
				batchBlockNumbers = append(batchBlockNumbers, *j)
			}
			batchJobQueue.Enqueue(batchBlockNumbers)
			batchStartBlock = new(big.Int).Sub(batchStartBlock, batchSize) // Move to the next batch
		}
	}()
	// Wait for completion or error
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case err := <-errorChan:
		core.LogError("[Base] Worker thread failed: " + err.Error())
		_Database.IndexerUpdateJobStatus(uuid, "failed")
		return
	case <-done:
		break
	}
	// Update final status
	finalBlockIndex := sequentialTracker.GetNextExpectedBlock()
	_Database.IndexerUpdateTailBlock(uuid, uint64(finalBlockIndex))
	_Database.IndexerUpdateJobStatus(uuid, "complete")
}

// --- Helper Functions --- //
func indexerPreflight(chainName string) (string, string, *big.Int, uint64, uint64, *big.Int) {
	// Handle indexer mutex and exit channel
	indexerCancel = make(chan bool, 1)
	IndexerMutex.Lock()
	if IsIndexing {
		IndexerMutex.Unlock()
		return "", "", nil, 0, 0, nil // Already running
	}
	IsIndexing = true
	IndexerMutex.Unlock()
	defer func() { // Cleanup mutex when we're done
		IndexerMutex.Lock()
		IsIndexing = false
		IndexerMutex.Unlock()
	}()
	indexerRunning := _Database.SettingsGetValue("indexerRunning")
	baseIndexerRunning := _Database.SettingsGetValue("baseIndexerRunning")
	if indexerRunning != "true" || baseIndexerRunning == "false" {
		return "", "", nil, 0, 0, nil
	}
	uuid := _Database.IndexerGetJobUUID(chainName) // Lookup the UUID of the blockchain job
	if uuid == "" {                                // If no job exists, create one
		uuid = createIndexerJob(chainName)
	}
	databaseStatus := _Database.IndexerGetJobStatus(uuid) // Get the status of the job
	// ---- Job Status Dispatch ---- //
	if databaseStatus == "running" { // Only 1 post caching job running at a time
		return "", "", nil, 0, 0, nil // bail out
	}
	switch _Blockchain.Base.RpcUrl { // Set throttle defaults for known public nodes
	case DefaultBlockchainNodes["base"][0]:
		_Database.SettingsUpdateValue("baseThrottle", DefaultBlockchainNodes["base"][1]) // default rate limit for default nodes
	}
	chainLatestBlock, err := _Blockchain.GetLatestBlock(chainName) // Get the latest block number from the blockchain RPC node
	if err != nil {
		core.LogDebug("[Base] Could not get latest block number: GetLatestBlock returned error: " + err.Error())
		return "", "", nil, 0, 0, nil // bail out
	}
	if chainLatestBlock.Cmp(big.NewInt(0)) == 0 { // Error checking the latest block number we got from the RPC node
		core.LogDebug("[Base] Could not get latest block number: " + chainLatestBlock.String())
		return "", "", nil, 0, 0, nil // bail out
	}
	chainEarliestBlock := _Blockchain.GetEarliestBlock(chainName)                  // Get the earliest block number that a YourPlace post existed (YourPlace genesis block)
	databaseHeadBlock := _Database.IndexerGetHeadBlock(uuid)                       // Get the head block from the database (latest block processed)
	databaseTailBlock := _Database.IndexerGetTailBlock(uuid)                       // Get the tail block from the database (earliest block processed)
	if databaseTailBlock < chainEarliestBlock.Uint64() && databaseTailBlock != 0 { // Check that the tail block is ahead of the earliest block
		core.LogDebug("[Base] Database tail block is too far back - resetting to EarliestBlock")
		cutoffTimestamp := _Blockchain.Base.GetBlockTimestamp(chainEarliestBlock)
		if cutoffTimestamp > 0 {
			_Database.OnchainDeleteExpired(chainName, cutoffTimestamp)
		}
		_Database.IndexerUpdateTailBlock(uuid, chainEarliestBlock.Uint64()) // If not, reset the tail block to the earliest block
		databaseTailBlock = chainEarliestBlock.Uint64()
	}
	core.LogDebug("[Base] --- BaseIndexerFetchData(): Fetching posts for " + chainName + " ---")
	core.LogDebug("[Base] Chain Latest Block: " + chainLatestBlock.String())
	core.LogDebug("[Base] Database Head Block: " + strconv.Itoa(int(databaseHeadBlock)))
	core.LogDebug("[Base] Database Tail Block: " + strconv.Itoa(int(databaseTailBlock)))
	core.LogDebug("[Base] Chain Earliest Block: " + chainEarliestBlock.String())
	return databaseStatus, uuid, chainLatestBlock, databaseTailBlock, databaseHeadBlock, chainEarliestBlock
}
func dispatchTransaction(block map[string]interface{}, transaction map[string]interface{}, databaseHistoryDaysInt *int, blockIndex *big.Int, blockchain string, base *Base) int {
	// ret 0 == success == transaction was a YP txn and was processed
	// ret 1 == skipped == transaction was not a YP txn
	// ret 2 == expired == transaction is older than the cached history limit
	txHash := strings.ToLower(transaction["hash"].(string))
	//fromAddr := strings.ToLower(transaction["from"].(string))
	if transaction["to"] == nil { // Skip transactions with no recipient
		return 1
	}
	toAddr := strings.ToLower(transaction["to"].(string))
	if transaction["input"] == nil { // Skip transactions with no data payload
		return 1
	}
	inputHex := transaction["input"].(string)
	//amountHexStr := transaction["value"].(string)[2:]
	//amountInt, _ := strconv.ParseUint(amountHexStr, 16, 64)
	//parentTxHash := "" // todo - figure out comment logic hierarchy
	timestampHexStr := block["timestamp"].(string)[2:]
	timestamp, _ := strconv.ParseUint(timestampHexStr, 16, 64)
	/*if isTimestampExpired(int64(*databaseHistoryDaysInt), int64(timestamp)) { // skip transactions older than the cached history limit
		return 2
	}*/
	if toAddr == entryPointV06Address {
		payloads := extractSmartWalletPayloads(inputHex)
		if len(payloads) > 0 {
			for _, p := range payloads {
				core.LogDebug("[Base] Smart wallet YourPlace transaction found: " + txHash + " from " + p.fromAddress + " to " + p.toAddress)
				syntheticTxn := map[string]interface{}{
					"hash":  transaction["hash"],
					"from":  p.fromAddress,
					"to":    p.toAddress,
					"input": "0x" + hex.EncodeToString([]byte(p.payload)),
					"value": "0x0",
				}
				tokenizeYourPlaceTransaction(blockchain, syntheticTxn, timestamp, blockIndex.Uint64())
			}
			return 0
		}
	}
	data := inputHex[2:]
	decodedDataBytes, _ := hex.DecodeString(data)
	decodedDataStr := string(decodedDataBytes)
	if strings.HasPrefix(decodedDataStr, services.YpPrefix) { // Is the txn a YourPlace post
		core.LogDebug("[Base] YourPlace transaction found: " + txHash)
		tokenizeYourPlaceTransaction(blockchain, transaction, timestamp, blockIndex.Uint64())
		return 0
	} else {
		return 1
	}
}
func isTimestampExpired(databaseHistoryDaysInt int64, timestamp int64) bool {
	now := time.Now()
	diff := now.Sub(time.Unix(timestamp, 0))
	return diff > time.Duration(databaseHistoryDaysInt)*24*time.Hour
}
func parseAllUserOperationsFromHandleOps(data []byte) ([]*userOperation, error) {
	// parseAllUserOperationsFromHandleOps parses all UserOperations from handleOps calldata
	if len(data) < 4 {
		return nil, fmt.Errorf("data too short")
	}
	data = data[4:] // Skip the 4-byte selector (handleOps)
	if len(data) < 64 {
		return nil, fmt.Errorf("data too short for handleOps params")
	}
	// First param is offset to UserOperation[] array
	opsOffset := new(big.Int).SetBytes(data[0:32]).Uint64()
	if opsOffset+32 > uint64(len(data)) {
		return nil, fmt.Errorf("invalid ops offset")
	}
	// At the offset, we have the array length
	arrayLen := new(big.Int).SetBytes(data[opsOffset : opsOffset+32]).Uint64()
	if arrayLen == 0 {
		return nil, fmt.Errorf("empty ops array")
	}
	var ops []*userOperation
	for i := uint64(0); i < arrayLen; i++ {
		opOffsetPos := opsOffset + 32 + (i * 32)
		if opOffsetPos+32 > uint64(len(data)) {
			continue
		}
		opOffset := new(big.Int).SetBytes(data[opOffsetPos : opOffsetPos+32]).Uint64()
		opStart := opsOffset + 32 + opOffset
		if opStart+32*11 > uint64(len(data)) {
			continue
		}
		op := &userOperation{}
		op.sender = data[opStart+12 : opStart+32]
		op.nonce = new(big.Int).SetBytes(data[opStart+32 : opStart+64])
		initCodeOffset := new(big.Int).SetBytes(data[opStart+64 : opStart+96]).Uint64()
		callDataOffset := new(big.Int).SetBytes(data[opStart+96 : opStart+128]).Uint64()
		op.callGasLimit = new(big.Int).SetBytes(data[opStart+128 : opStart+160])
		op.verificationGasLimit = new(big.Int).SetBytes(data[opStart+160 : opStart+192])
		op.preVerificationGas = new(big.Int).SetBytes(data[opStart+192 : opStart+224])
		op.maxFeePerGas = new(big.Int).SetBytes(data[opStart+224 : opStart+256])
		op.maxPriorityFeePerGas = new(big.Int).SetBytes(data[opStart+256 : opStart+288])
		paymasterAndDataOffset := new(big.Int).SetBytes(data[opStart+288 : opStart+320]).Uint64()
		signatureOffset := new(big.Int).SetBytes(data[opStart+320 : opStart+352]).Uint64()
		op.initCode = parseDynamicBytes(data, opStart, initCodeOffset)
		op.callData = parseDynamicBytes(data, opStart, callDataOffset)
		op.paymasterAndData = parseDynamicBytes(data, opStart, paymasterAndDataOffset)
		op.signature = parseDynamicBytes(data, opStart, signatureOffset)
		ops = append(ops, op)
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("no valid ops parsed")
	}
	return ops, nil
}
func parseDynamicBytes(data []byte, baseOffset uint64, relativeOffset uint64) []byte {
	// parseDynamicBytes extracts dynamic bytes from ABI-encoded data
	absOffset := baseOffset + relativeOffset
	if absOffset+32 > uint64(len(data)) {
		return nil
	}
	length := new(big.Int).SetBytes(data[absOffset : absOffset+32]).Uint64()
	if absOffset+32+length > uint64(len(data)) {
		return nil
	}
	return data[absOffset+32 : absOffset+32+length]
}
func getSenderFromUserOp(op *userOperation) string {
	// getSenderFromUserOp returns the sender address from the UserOperation
	// The sender field in ERC-4337 UserOperation is the account that authorized the operation
	return "0x" + strings.ToLower(hex.EncodeToString(op.sender))
}
func extractTargetFromCallData(callData []byte) string {
	// extractTargetFromCallData extracts the target address from smart wallet callData
	// Supports: execute(address,uint256,bytes), executeBatch((address,uint256,bytes)[])
	if len(callData) < 4 {
		return ""
	}
	selector := hex.EncodeToString(callData[:4])
	switch selector {
	case "b61d27f6": // execute(address target, uint256 value, bytes data)
		if len(callData) >= 36 {
			return "0x" + strings.ToLower(hex.EncodeToString(callData[16:36]))
		}
	case executeBatchSelector: // executeBatch((address,uint256,bytes)[])
		// For batch calls, extract the first target address
		if len(callData) < 68 {
			return ""
		}
		// Skip selector (4) + offset to array (32) = 36, then array length (32) = 68
		// First element offset is at position 68
		arrayOffset := new(big.Int).SetBytes(callData[4:36]).Uint64()
		if arrayOffset+32 > uint64(len(callData)) {
			return ""
		}
		arrayLen := new(big.Int).SetBytes(callData[4+arrayOffset : 4+arrayOffset+32]).Uint64()
		if arrayLen == 0 {
			return ""
		}
		// Get offset to first struct
		firstStructOffsetPos := 4 + arrayOffset + 32
		if firstStructOffsetPos+32 > uint64(len(callData)) {
			return ""
		}
		firstStructOffset := new(big.Int).SetBytes(callData[firstStructOffsetPos : firstStructOffsetPos+32]).Uint64()
		// First field of struct is the target address
		targetPos := 4 + arrayOffset + 32 + firstStructOffset
		if targetPos+32 > uint64(len(callData)) {
			return ""
		}
		return "0x" + strings.ToLower(hex.EncodeToString(callData[targetPos+12:targetPos+32]))
	case "51945447": // executeCalls((address,uint256,bytes)[]) - alternate batch selector
		if len(callData) < 68 {
			return ""
		}
		arrayOffset := new(big.Int).SetBytes(callData[4:36]).Uint64()
		if arrayOffset+32 > uint64(len(callData)) {
			return ""
		}
		arrayLen := new(big.Int).SetBytes(callData[4+arrayOffset : 4+arrayOffset+32]).Uint64()
		if arrayLen == 0 {
			return ""
		}
		firstStructOffsetPos := 4 + arrayOffset + 32
		if firstStructOffsetPos+32 > uint64(len(callData)) {
			return ""
		}
		firstStructOffset := new(big.Int).SetBytes(callData[firstStructOffsetPos : firstStructOffsetPos+32]).Uint64()
		targetPos := 4 + arrayOffset + 32 + firstStructOffset
		if targetPos+32 > uint64(len(callData)) {
			return ""
		}
		return "0x" + strings.ToLower(hex.EncodeToString(callData[targetPos+12:targetPos+32]))
	}
	return ""
}
func extractSmartWalletPayloads(inputHex string) []smartWalletPayload {
	var results []smartWalletPayload
	if len(inputHex) < 10 {
		return results
	}
	selector := strings.ToLower(inputHex[2:10])
	if selector != handleOpsSelector {
		return results
	}
	dataBytes, err := hex.DecodeString(inputHex[2:])
	if err != nil {
		return results
	}
	// Parse all UserOperations from the handleOps call
	ops, err := parseAllUserOperationsFromHandleOps(dataBytes)
	if err != nil {
		core.LogDebug("Failed to parse UserOperations: " + err.Error())
		return results
	}
	// For each UserOperation, check if it contains a YourPlace payload
	ypPrefixHex := hex.EncodeToString([]byte(services.YpPrefix))
	for _, op := range ops {
		signerAddr := getSenderFromUserOp(op)
		if !security.RegexMatch(`^0x[a-f0-9]{40}$`, signerAddr) || signerAddr == burnAddressETH {
			continue
		}
		targetAddr := extractTargetFromCallData(op.callData)
		if targetAddr == "" {
			targetAddr = burnAddressETH
		}
		// Search for YourPlace payloads within this UserOp's callData
		callDataHex := hex.EncodeToString(op.callData)
		searchStart := 0
		for {
			idx := strings.Index(callDataHex[searchStart:], ypPrefixHex)
			if idx == -1 {
				break
			}
			payloadStartHex := searchStart + idx
			payloadEndHex := payloadStartHex
			for i := payloadStartHex; i < len(callDataHex)-1; i += 2 {
				byteVal, err := strconv.ParseUint(callDataHex[i:i+2], 16, 8)
				if err != nil {
					break
				}
				if byteVal == 0 {
					break
				}
				payloadEndHex = i + 2
			}
			if payloadEndHex > payloadStartHex {
				payloadBytes, err := hex.DecodeString(callDataHex[payloadStartHex:payloadEndHex])
				if err == nil && strings.HasPrefix(string(payloadBytes), services.YpPrefix) {
					payloadStr := string(payloadBytes)
					isValid, _, _, _ := isValidYourPlacePayload(payloadStr)
					if isValid {
						results = append(results, smartWalletPayload{
							fromAddress: signerAddr,
							toAddress:   targetAddr,
							payload:     payloadStr,
						})
					}
				}
			}
			searchStart = payloadEndHex
			if searchStart >= len(callDataHex) {
				break
			}
		}
	}
	return results
}
func createIndexerJob(blockchain string) string {
	uuid := security.UUID()
	_Database.IndexerCreateJob(uuid, blockchain)
	return uuid
}
func indexerPrintProgress(targetEarliestBlock *big.Int, targetLatestBlock *big.Int, blockIndex *big.Int, batchSize *big.Int, direction string, requestTracker *RequestTracker) {
	core.LogDebug("[Base] ------------------------")
	core.LogDebug("[Base] index: " + blockIndex.String() + " - direction: " + direction)
	core.LogDebug("[Base] target latest: " + targetLatestBlock.String() + " - target earliest: " + targetEarliestBlock.String())
	totalRange := new(big.Int).Sub(targetLatestBlock, targetEarliestBlock)
	var progressMade *big.Int
	if direction == "forward" {
		progressMade = new(big.Int).Sub(blockIndex, targetEarliestBlock)
	} else {
		progressMade = new(big.Int).Sub(targetLatestBlock, blockIndex)
	}
	progressPercent := calculatePercentage(totalRange, progressMade)
	core.LogDebug("[Base] blocks processed: " + progressMade.String() + " - progress: " + progressPercent + " %")
	progressRemaining := new(big.Int).Sub(totalRange, progressMade)
	batchesRemaining := new(big.Int).Div(progressRemaining, batchSize)
	batchSizeRemainder := new(big.Int).Mod(progressRemaining, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchesRemaining.Add(batchesRemaining, big.NewInt(1))
	}
	core.LogDebug("[Base] blocks remaining: " + progressRemaining.String() + " - batches remaining: " + batchesRemaining.String())
}
func calculatePercentage(totalRange *big.Int, index *big.Int) string {
	if totalRange.Sign() == 0 {
		return big.NewInt(0).String()
	}
	percentage := new(big.Int)
	hundred := big.NewInt(100)
	percentage.Mul(index, hundred)
	percentage.Div(percentage, totalRange)
	return percentage.String()
}
func calculateOptimalBatchSize(throttleValue int) *big.Int { // Function to calculate the optimal batch size based on throttle and batch size limit
	throttleBasedLimit := throttleValue - throttleOffset
	if throttleBasedLimit <= 0 {
		throttleBasedLimit = 1
	}
	effectiveBatchSize := throttleBasedLimit
	if effectiveBatchSize > batchSizeLimit {
		effectiveBatchSize = batchSizeLimit
	}
	return big.NewInt(int64(effectiveBatchSize))
}
func configureRateLimiter(throttleValue int) *rate.Limiter { // Function to configure rate limiter based on throttle and batch size
	requestsPerSecond := calculateDynamicRate(throttleValue, 1)
	burstCapacity := throttleValue // Allow burst up to the full throttle value
	return rate.NewLimiter(rate.Limit(requestsPerSecond), burstCapacity)
}
func startThrottleController(uuid string, targetThrottleValue int, rateLimiter *rate.Limiter, database *db.Database) {
	ticker := time.NewTicker(30 * time.Second) // Adjust every N-seconds
	defer ticker.Stop()
	const (
		adjustmentStep = 0.1   // 10% adjustment per iteration
		maxMultiplier  = 100.0 // Don't go above 10,000% of the original rate (1.0 == 100%)
		minMultiplier  = 0.1   // Don't go below 10% of the original rate
		tolerance      = 0.1   // 10% tolerance before adjusting
	)
	for {
		select {
		case <-ticker.C:
			if breakPoint(uuid) {
				return
			}
			if globalRequestTracker == nil {
				continue
			}
			actualRPS := globalRequestTracker.GetRequestsPerSecond()
			targetRPS := float64(targetThrottleValue)
			if actualRPS < 0.1 { // Not enough data yet
				continue
			}
			// Update the RPS in the database (rounded to nearest integer)
			database.IndexerUpdateJobSpeed(uuid, uint64(actualRPS+0.5))
			ratio := actualRPS / targetRPS
			// Only adjust if we're significantly outside the tolerance range
			if ratio < (1.0-tolerance) || ratio > (1.0+tolerance) {
				throttleControlMutex.Lock()
				if ratio < (1.0 - tolerance) {
					// We're going too slow, increase multiplier
					dynamicThrottleMultiplier += adjustmentStep
					if dynamicThrottleMultiplier > maxMultiplier {
						dynamicThrottleMultiplier = maxMultiplier
					}
				} else if ratio > (1.0 + tolerance) {
					// We're going too fast, decrease multiplier aggressively to prevent rate limiting
					dynamicThrottleMultiplier -= adjustmentStep * 2 // Decrease faster to avoid hitting rate limits
					if dynamicThrottleMultiplier < minMultiplier {
						dynamicThrottleMultiplier = minMultiplier
					}
				}
				//core.LogDebug("Throttle adjustment:\tactual=" + strconv.FormatFloat(actualRPS, 'f', 2, 64) +
				//	"\ttarget=" + strconv.FormatFloat(targetRPS, 'f', 2, 64) +
				//	"\tmultiplier=" + strconv.FormatFloat(dynamicThrottleMultiplier, 'f', 3, 64))
				throttleControlMutex.Unlock()
				// Update the rate limiter with the new rate
				newRate := calculateDynamicRate(targetThrottleValue, 1)
				rateLimiter.SetLimit(rate.Limit(newRate))
			}
		default:
			if breakPoint(uuid) {
				return
			}
			time.Sleep(100 * time.Millisecond) // Sleep to avoid busy waiting
		}
	}
}
func calculateDynamicRate(throttleValue int, batchSize int) float64 {
	throttleControlMutex.RLock()
	defer throttleControlMutex.RUnlock()
	batchesPerSecond := (float64(throttleValue) / float64(batchSize)) * dynamicThrottleMultiplier
	if batchesPerSecond < 1.0 {
		batchesPerSecond = 1.0
	}
	return batchesPerSecond
}
func rpcBatchGetBlockByNumber(uuid string, base *Base, batchBlockNumbers []big.Int) []map[string]interface{} {
	batchSize := len(batchBlockNumbers)
	// Create the batch RPC call
	var batch []rpc.BatchElem
	for _, blockNumber := range batchBlockNumbers {
		blockNumberHex := "0x" + blockNumber.Text(16)
		request := rpc.BatchElem{
			Method: "eth_getBlockByNumber",
			Args:   []interface{}{blockNumberHex, true},
			Result: &map[string]interface{}{},
		}
		batch = append(batch, request)
	}
	// Make the RPC call with retry only for connection errors
	rpcErrorCount := 0
	rateLimitErrorCount := 0
BATCHRPCCALL:
	var blocks []map[string]interface{}
	if breakPoint(uuid) {
		return nil
	}
	err := base.RpcClient.BatchCallContext(context.Background(), batch)
	if err != nil {
		core.LogDebug("[Base] Could not perform RPC call from rpcBatchGetBlockByNumber, backing off")
		// Check if this is a rate-limiting error
		if strings.Contains(strings.ToLower(err.Error()), "rps limit") || strings.Contains(strings.ToLower(err.Error()), "rate limit") {
			rateLimitErrorCount++
			// Calculate backoff based on batch size and rate limit severity
			baseBackoff := rateLimitErrorCount * rateLimitErrorCount // 1, 4, 9, 16 seconds...
			batchPenalty := batchSize / 5                            // Add penalty based on batch size (1 second per 5 requests)
			backoff := baseBackoff + batchPenalty
			if backoff > 120 { // Cap at 2 minutes
				backoff = 120
			}
			core.LogDebug("[Base] Rate limit detected for batch of " + strconv.Itoa(batchSize) + " requests, backing off for " + strconv.Itoa(backoff) + " seconds")
			time.Sleep(time.Duration(backoff) * time.Second)
			// Reduce throttle multiplier on rate limits (entire batch failed)
			throttleControlMutex.Lock()
			dynamicThrottleMultiplier *= 0.90 // Reduce to 90% on rate limit errors
			if dynamicThrottleMultiplier < 0.1 {
				dynamicThrottleMultiplier = 0.1 // Don't go below 10% of original rate
			}
			core.LogDebug("[Base] Reducing throttle multiplier to " + strconv.FormatFloat(dynamicThrottleMultiplier, 'f', 3, 64))
			throttleControlMutex.Unlock()
			if rateLimitErrorCount >= 20 {
				core.LogDebug("[Base] Too many rate limit errors, failing batch")
				_Database.IndexerUpdateJobStatus(uuid, "failed")
				return nil
			}
		} else {
			// Regular connection errors use standard exponential backoff
			rpcErrorCount++
			backoff := (rpcErrorCount + 1) * 2
			time.Sleep(time.Duration(backoff) * time.Second)
			if rpcErrorCount >= 120 {
				core.LogDebug("[Base] Backfill failed too many times: " + err.Error())
				_Database.IndexerUpdateJobStatus(uuid, "failed")
				return nil
			}
		}
		goto BATCHRPCCALL
	}
	// Process batch results - check for rate limits in individual responses
	hasRateLimitError := false
	rateLimitCount := 0
	i := 0
	for _, elem := range batch { // Loop through each block in the batch response
		if breakPoint(uuid) {
			return nil
		}
		if elem.Error != nil {
			errorMsg := elem.Error.Error()
			//core.LogDebug("Could not get block data from rpcBatchGetBlockByNumber: " + errorMsg)
			//core.LogDebug("\tindex: " + batchBlockNumbers[i].String())
			//core.LogDebug("\tmethod: " + elem.Method)
			// Check for rate limiting in individual responses
			if strings.Contains(strings.ToLower(errorMsg), "rps limit") || strings.Contains(strings.ToLower(errorMsg), "rate limit") {
				hasRateLimitError = true
				rateLimitCount++
			}
			blocks = append(blocks, nil) // Append nil to maintain array alignment
		} else {
			if elem.Result == nil {
				blocks = append(blocks, nil) // Append nil if the result to maintain array alignment
			} else {
				resultPtr, ok := elem.Result.(*map[string]interface{})
				if !ok || resultPtr == nil {
					blocks = append(blocks, nil) // Append nil if the result to maintain array alignment
				} else {
					blocks = append(blocks, *resultPtr) // Append the result to the blocks slice
				}
			}
		}
		i++
	}
	// Only reduce throttle if ALL requests in the batch were rate-limited
	if hasRateLimitError && rateLimitCount == batchSize {
		throttleControlMutex.Lock()
		// All requests were rate limited, reduce throttle slightly
		dynamicThrottleMultiplier *= 0.90 // Reduce to 90% when the entire batch fails
		if dynamicThrottleMultiplier < 0.1 {
			dynamicThrottleMultiplier = 0.1 // Don't go below 10% of the original rate
		}
		core.LogDebug("[Base] All " + strconv.Itoa(batchSize) + " requests in a batch were rate limited, reducing multiplier to " + strconv.FormatFloat(dynamicThrottleMultiplier, 'f', 3, 64))
		throttleControlMutex.Unlock()
	} else if hasRateLimitError {
		// Only some requests were rate-limited, don't adjust throttle, just re-queue failed requests
		core.LogDebug("[Base] Partial rate limiting in batch: " + strconv.Itoa(rateLimitCount) + "/" + strconv.Itoa(batchSize) + " requests throttled, not adjusting global throttle")
	}
	return blocks
}
func workerThread(uuid string, rateLimiter *rate.Limiter, base *Base, batchJobQueue *core.ThreadSafeQueue, sequentialTracker *SequentialBlockTracker, requestTracker *RequestTracker, txnCount *core.ThreadSafeCounter, databaseHistoryDaysInt int, targetEarliestBlock *big.Int, targetLatestBlock *big.Int, batchSize *big.Int, direction string) error {
	// Worker thread to process batches of blocks
	for {
		batch, populated := batchJobQueue.Dequeue()
		if !populated {
			return nil
		}
		batchArray := batch.([]big.Int) // Get the batch of blocks
		_batchSize := len(batchArray)
		// Serialize token acquisition across all workers to prevent simultaneous RPS spikes
		rateLimiterMutex.Lock()
		// Wait for rate limit tokens based on the actual number of ETH requests in batch
		for i := 0; i < _batchSize; i++ {
			err := rateLimiter.Wait(context.Background())
			if err != nil {
				rateLimiterMutex.Unlock()
				return core.LogErrorReturn("[Base] Rate limiter wait failed: " + err.Error())
			}
		}
		rateLimiterMutex.Unlock()
		if breakPoint(sequentialTracker.uuid) { // Check for cancellation before RPC call
			return nil
		}
		// Track active requests with atomic counters for cross-worker coordination
		atomic.AddInt64(&activeRequestsCount, int64(_batchSize))
		atomic.AddInt64(&totalRequestsCount, int64(_batchSize))
		// Make the RPC call
		blocks := rpcBatchGetBlockByNumber(uuid, base, batchArray)
		// Decrement active requests after RPC call completes
		atomic.AddInt64(&activeRequestsCount, -int64(_batchSize))
		// Record the actual number of RPC requests completed (one per block in batch) with completion time
		requestTracker.RecordRequests(_batchSize)
		if globalRequestTracker != nil {
			globalRequestTracker.RecordRequests(_batchSize)
		}
		if blocks == nil {
			continue // Skip processing if blocks are nil
		}
		// Collect failed blocks for re-queuing
		var failedBlocks []big.Int
		// Process each block and update the block index as we go
		for i, block := range blocks {
			if i >= len(batchArray) { // Safety check
				break
			}
			if block == nil {
				// Don't mark failed blocks as processed - add them for retry
				failedBlocks = append(failedBlocks, batchArray[i])
				continue // Skip nil blocks but don't mark as processed
			}
			currentBlockNumber := batchArray[i].Int64()
			transactionsRaw, exists := block["transactions"]
			if !exists || transactionsRaw == nil {
				sequentialTracker.MarkBlockProcessed(currentBlockNumber, direction)
				continue // Skip blocks with no transactions
			}
			transactions, ok := transactionsRaw.([]interface{})
			if !ok {
				sequentialTracker.MarkBlockProcessed(currentBlockNumber, direction)
				continue // Skip blocks with malformed transactions
			}
			// Loop through transactions in the block
			for _, txn := range transactions {
				if txn == nil {
					continue // Skip nil transactions
				}
				transaction, ok2 := txn.(map[string]interface{})
				if !ok2 {
					continue // Skip malformed transactions
				}
				ret := dispatchTransaction(block, transaction, &databaseHistoryDaysInt, big.NewInt(currentBlockNumber), "base", base)
				if ret == 1 || ret == 2 { // Skip transactions that are not valid YP posts
					continue
				}
				txnCount.Increment()
			}
			// Mark this block as processed in the sequential tracker
			sequentialTracker.MarkBlockProcessed(currentBlockNumber, direction)
			// Send status updates (deduplicated to prevent log spam from multiple workers)
			nextExpected := sequentialTracker.GetNextExpectedBlock()
			mod := nextExpected % reportInterval
			if mod == 0 {
				progressLogMutex.Lock()
				// Double-check to prevent duplicate logs if multiple workers reached this simultaneously
				if atomic.LoadInt64(&lastProgressBlock) != nextExpected {
					atomic.StoreInt64(&lastProgressBlock, nextExpected)
					indexerPrintProgress(targetEarliestBlock, targetLatestBlock, big.NewInt(nextExpected), big.NewInt(int64(_batchSize)), direction, requestTracker)
				}
				progressLogMutex.Unlock()
			}
		}
		// Re-queue failed blocks individually with backoff if any failed
		if len(failedBlocks) > 0 {
			core.LogDebug("[Base] Re-queuing " + strconv.Itoa(len(failedBlocks)) + " failed blocks individually")
			// Apply exponential backoff before re-queuing failed blocks
			backoffTime := len(failedBlocks) * 1 // 1 second per failed block
			if backoffTime > 30 {
				backoffTime = 30 // Cap at 30 seconds
			}
			time.Sleep(time.Duration(backoffTime) * time.Second)
			// Re-queue failed blocks individually to avoid batch amplification
			for _, failedBlock := range failedBlocks {
				singleBlockBatch := []big.Int{failedBlock}
				batchJobQueue.Enqueue(singleBlockBatch)
			}
		}
	}
}
func breakPoint(uuid string) bool {
	// This function is a break point for the indexer to allow for graceful cancellation when the signal is received
	select {
	case <-indexerCancel:
		core.LogDebug("[Base] Indexer cancelled in break point")
		_Database.IndexerUpdateJobStatus(uuid, "failed")
		return true
	default:
		return false // continue processing
	}
}

// --- Global Helper Functions --- //
func IndexerClearOldCachedPosts(__database *db.Database) {} // Clear cached transactions that are older than the configured (expired) cached post history limit
func BaseIndexerRestartJobs(__database *db.Database, blockchain string) {
	// set any indexer jobs to "failed" that were left in a "running" state from a crashed server
	jobUUID := __database.IndexerGetJobUUID(blockchain)
	__database.IndexerUpdateJobStatus(jobUUID, "failed")
}
func BaseIndexerStop() {
	IndexerMutex.Lock()
	defer IndexerMutex.Unlock()
	if IsIndexing && indexerCancel != nil {
		select {
		case indexerCancel <- true:
		default: // Channel already has a value, don't block
		}
	}
}
func ToggleIndexer(database *db.Database) {
	indexerRunning := database.SettingsGetValue("baseIndexerRunning")
	if indexerRunning == "true" || indexerRunning == "" {
		database.SettingsUpdateValue("baseIndexerRunning", "false")
		BaseIndexerStop()
	} else {
		database.SettingsUpdateValue("baseIndexerRunning", "true")
	}
}
func BaseIndexerCatchUpAll(database *db.Database) (bool, string) {
	metaKey := "indexerCatchUpLastRun_base"
	lastCatchUpStr := database.MetaGetValue(metaKey)
	if lastCatchUpStr != "" {
		lastCatchUp, err := strconv.ParseUint(lastCatchUpStr, 10, 64)
		if err == nil {
			currentTime := core.GetTimestamp()
			timeSinceLastRun := currentTime - lastCatchUp
			if timeSinceLastRun < 86400 {
				hoursRemaining := (86400 - timeSinceLastRun) / 3600
				minutesRemaining := ((86400 - timeSinceLastRun) % 3600) / 60
				var message string
				if hoursRemaining > 0 {
					message = fmt.Sprintf("Rate limit: Catch-up can only run once every 24 hours. Please try again in %d hours and %d minutes.", hoursRemaining, minutesRemaining)
				} else {
					message = fmt.Sprintf("Rate limit: Catch-up can only run once every 24 hours. Please try again in %d minutes.", minutesRemaining)
				}
				return false, message
			}
		}
	}
	snapshotURL := "https://yourplace-snapshots.s3.us-east-1.amazonaws.com/base-snapshot-complete.db.gz"
	snapshotJsonURL := "https://yourplace-snapshots.s3.us-east-1.amazonaws.com/base-snapshot-complete.json"
	database.MetaUpdateValue(metaKey, strconv.FormatUint(core.GetTimestamp(), 10))
	go func() {
		BaseIndexerStop()
		for i := 0; i < 120; i++ {
			if !IsIndexing {
				snapshotDir := filepath.Join(host.GetDataDir(), "snapshots")
				host.CreateFolder(snapshotDir)
				snapshotFile := filepath.Join(snapshotDir, "base-snapshot-complete.db.gz")
				snapshotMetadataFile := filepath.Join(snapshotDir, "base-snapshot-complete.json")
				if host.DoesExist(snapshotFile) {
					core.LogDebug("[Base] Deleting existing snapshot file: " + snapshotFile)
					host.DeleteIfExists(snapshotFile)
				}
				if host.DoesExist(snapshotMetadataFile) {
					core.LogDebug("[Base] Deleting existing snapshot metadata file: " + snapshotMetadataFile)
					host.DeleteIfExists(snapshotMetadataFile)
				}
				core.LogInfo("[Base] Downloading snapshot from: " + snapshotURL)
				err := network.HttpGetFile(snapshotURL, snapshotFile)
				if err != nil {
					core.LogError("[Base] Could not download snapshot: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				core.LogInfo("[Base] Downloading snapshot metadata from: " + snapshotJsonURL)
				err = network.HttpGetFile(snapshotJsonURL, snapshotMetadataFile)
				if err != nil {
					core.LogError("[Base] Could not download snapshot metadata: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				if !host.DoesExist(snapshotFile) {
					core.LogError("[Base] Snapshot file not found: " + snapshotFile)
					database.MetaUpdateValue(metaKey, "")
					return
				}
				if !host.DoesExist(snapshotMetadataFile) {
					core.LogError("[Base] Snapshot metadata file not found: " + snapshotMetadataFile)
					database.MetaUpdateValue(metaKey, "")
					return
				}
				core.LogInfo("[Base] Importing snapshot from: " + snapshotFile)
				err = database.ImportSnapshotNoMetadata(snapshotFile)
				if err != nil {
					core.LogError("[Base] Could not import snapshot: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				core.LogInfo("[Base] Reading snapshot metadata from: " + snapshotMetadataFile)
				metadataBytes, err := os.ReadFile(snapshotMetadataFile)
				if err != nil {
					core.LogError("[Base] Could not read snapshot metadata: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				var metadata map[string]interface{}
				err = json.Unmarshal(metadataBytes, &metadata)
				if err != nil {
					core.LogError("[Base] Could not parse snapshot metadata: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				headBlock, headOk := metadata["head_block"].(float64)
				tailBlock, tailOk := metadata["tail_block"].(float64)
				if !headOk || !tailOk {
					core.LogError("[Base] Snapshot metadata missing head_block or tail_block")
					database.MetaUpdateValue(metaKey, "")
					return
				}
				core.LogInfo(fmt.Sprintf("[Base] Updating indexer job with head_block: %d, tail_block: %d", uint64(headBlock), uint64(tailBlock)))
				jobUUID := database.IndexerGetJobUUID("base")
				if jobUUID == "" {
					core.LogError("[Base] Could not find indexer job UUID for blockchain: base")
					database.MetaUpdateValue(metaKey, "")
					return
				}
				database.IndexerUpdateHeadBlock(jobUUID, uint64(headBlock))
				database.IndexerUpdateTailBlock(jobUUID, uint64(tailBlock))
				host.DeleteAll(snapshotDir)
				core.LogInfo("[Base] Snapshot import complete")
				return
			}
			time.Sleep(5 * time.Second)
		}
		core.LogError("[Base] Indexer did not stop in time during snapshot import")
		database.MetaUpdateValue(metaKey, "")
		return
	}()
	return true, "Indexer catch-up started successfully."
}
