package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/time/rate"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 0 --- Earliest --- Tail --- Head --- Latest
// Job States - Running, Complete, Failed, Pending

const (
	reportInterval = 5000 // print progress every # of blocks
	saveInterval   = 100  // save progress every # of blocks
	throttleOffset = 4    // How many blocks to subtract from the throttle limit to allow for the front-end to make RPC calls without getting rate-limited
	batchSizeLimit = 25   // The maximum number of blocks to fetch in a single batch RPC call
	workerCount    = 25   // Number of worker threads to use for processing batches
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
)

type SequentialBlockTracker struct {
	mu                sync.Mutex
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
	sbt.mu.Lock()
	defer sbt.mu.Unlock()
	return sbt.nextExpectedBlock
}
func (sbt *SequentialBlockTracker) HasPendingBlocks() bool {
	sbt.mu.Lock()
	defer sbt.mu.Unlock()
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
func IndexerFetchData(database *db.Database, blockchain *Blockchain, chainName string) {
	_Blockchain = blockchain
	_Database = database
	databaseStatus, uuid, chainLatestBlock, databaseTailBlock, databaseHeadBlock, chainEarliestBlock := indexerPreflight(chainName)
	if uuid == "" || databaseStatus == "" {
		return // bail out if the preflight bails out
	}
	_ = databaseHeadBlock
	switch databaseStatus { // Post fill job dispatch, based on last job status
	case "pending":
		core.LogDebug("Starting pending job from the beginning")
		IndexerBaseFullFill(blockchain.Base, uuid, chainLatestBlock)
	case "failed":
		core.LogDebug("Restarting failed job from where it left off")
		if databaseTailBlock == 0 { // If a full fill job started, but failed before the tail block was written, start all over
			IndexerRestartJobs(_Database, chainName)
			IndexerBaseFullFill(blockchain.Base, uuid, chainLatestBlock)
			return
		}
		if databaseTailBlock > chainEarliestBlock.Uint64() { // if a backfill job failed, restart it
			IndexerBaseBackFill(blockchain.Base, uuid, chainLatestBlock)
		} else { // if a front fill job failed, restart it
			IndexerBaseFrontFill(blockchain.Base, uuid, chainLatestBlock)
		}
	case "complete": // If everything is backfilled, then just process the newest blocks
		core.LogDebug("Last job completed successfully. Getting new blocks")
		IndexerBaseFrontFill(blockchain.Base, uuid, chainLatestBlock)
	}
}

// --- Base Indexer Functions --- //
func IndexerBaseFrontFill(base *Base, uuid string, baseLatestBlock *big.Int) {
	// (old) head block <----- latest block (starting traversal @ latest block)
	// head block -----> latest block (starting traversal @ head block)
	core.LogDebug("--- IndexerBaseFrontFill()")
	direction := "forward"
	_Database.IndexerUpdateJobStatus(uuid, "running")
	headBlock := _Database.IndexerGetHeadBlock(uuid)
	if headBlock <= 0 {
		core.LogWarn("IndexerBaseFrontFill(): Head block is <= 0 - aborting")
		return
	}
	targetLatestBlock := baseLatestBlock
	targetEarliestBlock := headBlock
	core.LogDebug("Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("Target Earliest Block: " + strconv.Itoa(int(targetEarliestBlock)))
	targetEarliestBlockBigInt := big.NewInt(int64(targetEarliestBlock))
	databaseHistoryDaysInt, _ := strconv.Atoi(_Database.SettingsGetValue("historyDays"))
	baseThrottle, _ := strconv.Atoi(_Database.SettingsGetValue("baseThrottle"))
	core.LogDebug("Base Throttle: " + strconv.Itoa(baseThrottle))
	batchSize := calculateOptimalBatchSize(baseThrottle)
	core.LogDebug("Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, targetEarliestBlockBigInt) // figure out how many blocks we need to fetch
	if blockCount.Int64() <= 0 {
		core.LogError("Block count is negative or zero")
		return
	}
	core.LogDebug("Number of Blocks: " + blockCount.String())
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("Batch Count: " + batchCount.String())
	rateLimiter := configureRateLimiter(baseThrottle)
	batchStartBlock := new(big.Int).Set(targetEarliestBlockBigInt) // Start at the earliest block
	txnCount := core.NewThreadSafeCounter()
	batchJobQueue := core.NewThreadSafeQueue()
	sequentialTracker := NewSequentialBlockTracker(targetEarliestBlockBigInt.Int64(), uuid, _Database, direction)
	globalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, workerCount)

	go startThrottleController(uuid, baseThrottle, rateLimiter) // Start the throttle controller in a separate goroutine

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
	// Start worker threads to process the batch jobs
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := workerThread(uuid, rateLimiter, base, batchJobQueue, sequentialTracker, globalRequestTracker, txnCount, databaseHistoryDaysInt, targetEarliestBlockBigInt, targetLatestBlock, batchSize, "forward", errorChan)
			if err != nil {
				core.LogError("Worker thread failed: " + err.Error())
				errorChan <- err // Send the error to the error channel
			}
		}()
	}
	// Wait for completion or error
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done) // Close the done channel when all workers are done
	}()
	select {
	case err := <-errorChan:
		core.LogError("Worker thread failed: " + err.Error())
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
func IndexerBaseBackFill(base *Base, uuid string, baseLatestBlock *big.Int) {
	// earliest block <----- tail block (starting traversal @ tail block)—
	core.LogDebug("--- IndexerBaseBackFill() ---")
	direction := "backward"
	_Database.IndexerUpdateJobStatus(uuid, "running")
	databaseTailBlock := big.NewInt(int64(_Database.IndexerGetTailBlock(uuid)))
	if databaseTailBlock.Cmp(big.NewInt(0)) == 0 { // if databaseTailBlock = 0, then the job is new
		core.LogDebug("Database Tail Block is 0 - setting to Head Block")
		headBlockInt := _Database.IndexerGetHeadBlock(uuid)
		databaseTailBlock = big.NewInt(int64(headBlockInt))
		core.LogDebug("Database Tail Block: " + databaseTailBlock.String())
	}
	targetLatestBlock := databaseTailBlock
	targetEarliestBlock := &base.EarliestBlock
	if targetLatestBlock.Cmp(targetEarliestBlock) == 0 {
		core.LogDebug("Target latest block is equal to target earliest block - completing")
		_Database.IndexerUpdateJobStatus(uuid, "complete")
		return
	}
	if targetLatestBlock.Int64() == 0 { // if targetLatestBlock = 0, then the job is new
		targetLatestBlock = baseLatestBlock // set the widest possible target for a new backfill
		targetEarliestBlock = &base.EarliestBlock
	}
	core.LogDebug("Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("Target Earliest Block: " + targetEarliestBlock.String())
	targetEarliestBlockBigInt := targetEarliestBlock
	databaseHistoryDaysInt, _ := strconv.Atoi(_Database.SettingsGetValue("historyDays"))
	baseThrottle, _ := strconv.Atoi(_Database.SettingsGetValue("baseThrottle"))
	core.LogDebug("Base Throttle: " + strconv.Itoa(baseThrottle))
	batchSize := calculateOptimalBatchSize(baseThrottle)
	core.LogDebug("Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, targetEarliestBlockBigInt) // figure out how many blocks we need to fetch
	core.LogDebug("Block Count: " + blockCount.String())
	if blockCount.Int64() <= 0 {
		core.LogError("Block count is negative or zero")
		return
	}
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("Batch Count: " + batchCount.String())
	rateLimiter := configureRateLimiter(baseThrottle)                                                     // Configure rate limiter based on throttle and batch size
	batchStartBlock := targetLatestBlock                                                                  // Start at the latest block
	txnCount := core.NewThreadSafeCounter()                                                               // Count the number of transactions processed
	batchJobQueue := core.NewThreadSafeQueue()                                                            // Queue to hold batch jobs
	sequentialTracker := NewSequentialBlockTracker(targetLatestBlock.Int64(), uuid, _Database, direction) // Sequential block tracker starting from the highest block
	globalRequestTracker = NewRequestTracker()                                                            // Request tracker to monitor request rates
	errorChan := make(chan error, workerCount)                                                            // Channel to handle errors from workers

	// Start the throttle controller in a separate goroutine
	go startThrottleController(uuid, baseThrottle, rateLimiter)

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
	// Start worker threads to process the batch jobs
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1) // Add a worker to the wait group
		go func() {
			defer wg.Done()
			err := workerThread(uuid, rateLimiter, base, batchJobQueue, sequentialTracker, globalRequestTracker, txnCount, databaseHistoryDaysInt, targetEarliestBlock, targetLatestBlock, batchSize, "backward", errorChan)
			if err != nil {
				core.LogError("Worker encountered an error: " + err.Error())
				errorChan <- err // Send the error to the error channel
			}
		}()
	}
	// Wait for completion or error
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done) // Close the done channel when all workers are done
	}()
	select {
	case err := <-errorChan:
		core.LogError("Worker thread failed: " + err.Error())
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
func IndexerBaseFullFill(base *Base, uuid string, baseLatestBlock *big.Int) {
	// earliest block <----- latest block (starting traversal @ latest block)
	core.LogDebug("--- IndexerBaseFullFill()")
	direction := "backward"
	_Database.IndexerUpdateJobStatus(uuid, "running")
	targetLatestBlock := baseLatestBlock
	core.LogDebug("Target Latest Block: " + targetLatestBlock.String())
	targetEarliestBlock := BaseGetEarliestBlock()
	core.LogDebug("Target Earliest Block: " + targetEarliestBlock.String())
	targetEarliestBlockBigInt := targetEarliestBlock
	_Database.IndexerUpdateHeadBlock(uuid, targetLatestBlock.Uint64())
	databaseHistoryDaysInt, _ := strconv.Atoi(_Database.SettingsGetValue("historyDays"))
	baseThrottle, _ := strconv.Atoi(_Database.SettingsGetValue("baseThrottle"))
	core.LogDebug("Base Throttle: " + strconv.Itoa(baseThrottle))
	batchSize := calculateOptimalBatchSize(baseThrottle)
	core.LogDebug("Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, &targetEarliestBlockBigInt) // figure out how many blocks we need to fetch
	if blockCount.Int64() <= 0 {
		core.LogError("Block count is negative or zero")
		return
	}
	core.LogDebug("Number of Blocks: " + blockCount.String())
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("Batch Count: " + batchCount.String())
	rateLimiter := configureRateLimiter(baseThrottle)                                                     // Configure a rate limiter based on throttle and batch size
	batchStartBlock := targetLatestBlock                                                                  // Start at the latest block
	txnCount := core.NewThreadSafeCounter()                                                               // Count the number of transactions processed
	batchJobQueue := core.NewThreadSafeQueue()                                                            // Queue to hold batch jobs
	sequentialTracker := NewSequentialBlockTracker(targetLatestBlock.Int64(), uuid, _Database, direction) // Sequential block tracker starting from the highest block
	globalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, workerCount) // Channel to handle errors from workers

	go startThrottleController(uuid, baseThrottle, rateLimiter) // Start the throttle controller in a separate goroutine

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
	// Start worker threads to process the batch jobs
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := workerThread(uuid, rateLimiter, base, batchJobQueue, sequentialTracker, globalRequestTracker, txnCount, databaseHistoryDaysInt, &targetEarliestBlockBigInt, targetLatestBlock, batchSize, "backward", errorChan)
			if err != nil {
				core.LogError("Worker encountered an error: " + err.Error())
				errorChan <- err
			}
		}()
	}
	// Wait for completion or error
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case err := <-errorChan:
		core.LogError("Worker thread failed: " + err.Error())
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

// --- Algorand Indexer Functions --- //
func IndexerAlgorandFrontFill(algo *Algorand, uuid string, algoLatestBlock *big.Int) {}
func IndexerAlgorandBackFill(algo *Algorand, uuid string, algoLatestBlock *big.Int)  {}
func IndexerAlgorandFullFill(algo *Algorand, uuid string, algoLatestBlock *big.Int)  {}

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
	indexerRunning := _Database.SettingsGetValue("indexerRunning") // Check if the indexer is globally enabled
	if indexerRunning != "true" {
		return "", "", nil, 0, 0, nil // bail out
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
	if err != nil || chainLatestBlock.Cmp(big.NewInt(0)) == 0 {    // Error checking the latest block number we got from the RPC node
		core.LogError("Could not get Base latest block number - Will try again on next indexer run")
		return "", "", nil, 0, 0, nil // bail out
	}
	chainEarliestBlock := _Blockchain.GetEarliestBlock(chainName)                  // Get the earliest block number that a YourPlace post existed (YourPlace genesis block)
	databaseHeadBlock := _Database.IndexerGetHeadBlock(uuid)                       // Get the head block from the database (latest block processed)
	databaseTailBlock := _Database.IndexerGetTailBlock(uuid)                       // Get the tail block from the database (earliest block processed)
	if databaseTailBlock < chainEarliestBlock.Uint64() && databaseTailBlock != 0 { // Check that the tail block is ahead of the earliest block
		core.LogDebug("Database tail block is too far back - resetting to EarliestBlock")
		_Database.IndexerUpdateTailBlock(uuid, chainEarliestBlock.Uint64()) // If not, reset the tail block to the earliest block
		databaseTailBlock = chainEarliestBlock.Uint64()
	}
	core.LogDebug("--- IndexerFetchData(): Fetching posts for " + chainName + " ---")
	core.LogDebug("Chain Latest Block: " + chainLatestBlock.String())
	core.LogDebug("Database Head Block: " + strconv.Itoa(int(databaseHeadBlock)))
	core.LogDebug("Database Tail Block: " + strconv.Itoa(int(databaseTailBlock)))
	core.LogDebug("Chain Earliest Block: " + chainEarliestBlock.String())
	return databaseStatus, uuid, chainLatestBlock, databaseTailBlock, databaseHeadBlock, chainEarliestBlock
}
func dispatchTransaction(block map[string]interface{}, transaction map[string]interface{}, databaseHistoryDaysInt *int, blockIndex *big.Int, blockchain string) int {
	// ret 0 == success == transaction was a YP txn and was processed
	// ret 1 == skipped == transaction was not a YP txn
	// ret 2 == expired == transaction is older than the cached history limit
	txHash := strings.ToLower(transaction["hash"].(string))
	//fromAddr := strings.ToLower(transaction["from"].(string))
	if transaction["to"] == nil { // Skip transactions with no recipient
		return 1
	}
	//toAddr := strings.ToLower(transaction["to"].(string))
	if transaction["input"] == nil { // Skip transactions with no data payload
		return 1
	}
	data := transaction["input"].(string)[2:]
	decodedDataBytes, _ := hex.DecodeString(data)
	decodedDataStr := string(decodedDataBytes)
	//amountHexStr := transaction["value"].(string)[2:]
	//amountInt, _ := strconv.ParseUint(amountHexStr, 16, 64)
	//parentTxHash := "" // todo - figure out comment logic hierarchy
	timestampHexStr := block["timestamp"].(string)[2:]
	timestamp, _ := strconv.ParseUint(timestampHexStr, 16, 64)
	if isTimestampExpired(int64(*databaseHistoryDaysInt), int64(timestamp)) { // skip transactions older than the cached history limit
		return 2
	}
	if strings.HasPrefix(decodedDataStr, services.YpPrefix) { // Is the txn a YourPlace post
		core.LogDebug("YourPlace transaction found on " + blockchain + ": " + txHash)
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
func createIndexerJob(blockchain string) string {
	uuid := security.UUID()
	_Database.IndexerCreateJob(uuid, blockchain)
	return uuid
}
func indexerPrintProgress(targetEarliestBlock *big.Int, targetLatestBlock *big.Int, blockIndex *big.Int, batchSize *big.Int, direction string, requestTracker *RequestTracker) {
	core.LogDebug("------------------------")
	core.LogDebug("index: " + blockIndex.String() + " - direction: " + direction)
	core.LogDebug("target latest: " + targetLatestBlock.String() + " - target earliest: " + targetEarliestBlock.String())
	totalRange := new(big.Int).Sub(targetLatestBlock, targetEarliestBlock)
	var progressMade *big.Int
	if direction == "forward" {
		progressMade = new(big.Int).Sub(blockIndex, targetEarliestBlock)
	} else {
		progressMade = new(big.Int).Sub(targetLatestBlock, blockIndex)
	}
	progressPercent := calculatePercentage(totalRange, progressMade)
	core.LogDebug("blocks processed: " + progressMade.String() + " - progress: " + progressPercent + " %")
	progressRemaining := new(big.Int).Sub(totalRange, progressMade)
	batchesRemaining := new(big.Int).Div(progressRemaining, batchSize)
	batchSizeRemainder := new(big.Int).Mod(progressRemaining, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchesRemaining.Add(batchesRemaining, big.NewInt(1))
	}
	core.LogDebug("blocks remaining: " + progressRemaining.String() + " - batches remaining: " + batchesRemaining.String())
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
	burstCapacity := workerCount // Allow each worker to have one token
	return rate.NewLimiter(rate.Limit(requestsPerSecond), burstCapacity)
}
func startThrottleController(uuid string, targetThrottleValue int, rateLimiter *rate.Limiter) {
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
			if actualRPS < 1.0 { // Not enough data yet
				continue
			}
			ratio := actualRPS / targetRPS
			// Only adjust if we're outside the tolerance range
			if ratio < (1.0-tolerance) || ratio > (1.0+tolerance) {
				throttleControlMutex.Lock()
				if ratio < (1.0 - tolerance) {
					// We're going too slow, increase multiplier
					dynamicThrottleMultiplier += adjustmentStep
					if dynamicThrottleMultiplier > maxMultiplier {
						dynamicThrottleMultiplier = maxMultiplier
					}
				} else if ratio > (1.0 + tolerance) {
					// We're going too fast, decrease multiplier
					dynamicThrottleMultiplier -= adjustmentStep
					if dynamicThrottleMultiplier < minMultiplier {
						dynamicThrottleMultiplier = minMultiplier
					}
				}
				core.LogDebug("Throttle adjustment:\tactual=" + strconv.FormatFloat(actualRPS, 'f', 2, 64) +
					"\ttarget=" + strconv.FormatFloat(targetRPS, 'f', 2, 64) +
					"\tmultiplier=" + strconv.FormatFloat(dynamicThrottleMultiplier, 'f', 3, 64))
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
func tokenizeYourPlaceTransaction(blockchain string, transaction map[string]interface{}, timestamp uint64, blockNumber uint64) {
	// Pattern-based tokenization and database storage of YourPlace transactions
	data := transaction["input"].(string)[2:]       // get data from the transaction & drop the '0x' prefix
	decodedDataBytes, err := hex.DecodeString(data) // hex decode data
	if err != nil {
		core.LogDebug("Could not decode YourPlace transaction: " + err.Error())
		return
	}
	decodedDataStr := string(decodedDataBytes) // convert bytes to string
	var protocolRegex = regexp.MustCompile(`^yp/([\d.]+)/([a-z]+):(.+)$`)
	matches := protocolRegex.FindStringSubmatch(decodedDataStr) // match the string to the protocol regex
	if matches == nil {
		core.LogDebug("Could not tokenize YourPlace transaction: " + decodedDataStr)
		return
	}

	txHash := strings.ToLower(transaction["hash"].(string))
	fromAddress := strings.ToLower(transaction["from"].(string))
	toAddress := strings.ToLower(transaction["to"].(string))
	parentTxHash := ""
	amountHexStr := transaction["value"].(string)[2:]
	amountInt, _ := strconv.ParseUint(amountHexStr, 16, 64)

	version, err := strconv.Atoi(matches[1]) // get the version number
	if err != nil {
		core.LogError("Could not convert YourPlace transaction version: " + err.Error())
		return
	}
	action := matches[2] // get the action code
	if len(action) < 1 {
		core.LogError("Invalid YourPlace transaction action: " + action)
		return
	}
	actionPrefix := action[0]                                // parse out the action prefix
	actionPostfix := action[1:]                              // parse out the action postfix
	var payloadObject map[string]interface{}                 // create a map for the YourPlace payload object
	err = json.Unmarshal([]byte(matches[3]), &payloadObject) // unmarshal the payload object
	if err != nil {
		core.LogError("Could not unmarshal YourPlace transaction payload: " + err.Error())
		return
	}

	// Execute the YourPlace transaction based on the action code
	if version == 1 {
		switch actionPrefix {
		case 'p': // Post Actions
			core.LogDebug("Post Action: " + action)
			switch actionPostfix {
			case "":
				if !handlePostTransaction(payloadObject, txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, blockNumber) {
					break
				}
			case "a":
				if !handlePostTransactionAttachment(payloadObject, txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, blockNumber) {
					break
				}
			}
			break
		case 'r': // Reply Actions
		case 'f': // Follow Actions
			core.LogDebug("Follow Action: " + action)
			switch actionPostfix {
			case "":
				blockchainPayload, ok1 := payloadObject["b"]
				addressPayload, ok2 := payloadObject["a"]
				if !ok1 || !ok2 {
					core.LogDebug("Follow action missing required fields")
					break
				}
				blockchainStr, ok1 := blockchainPayload.(string)
				addressStr, ok2 := addressPayload.(string)
				if !ok1 || !ok2 {
					core.LogDebug("Follow action fields are not strings")
					break
				}
				if !security.IsValidBlockchain(blockchainStr) {
					core.LogDebug("Invalid blockchain in follow action")
					break
				}
				if !security.IsValidAddress(addressStr, blockchainStr) {
					core.LogDebug("Invalid address in follow action")
					break
				}
				if fromAddress == addressStr && blockchain == blockchainStr { // Ignore self-follow attempts (follower count fraud)
					break
				}
				_Database.OnchainF(txHash, blockchain, fromAddress, blockchain, addressStr, blockchainStr, timestamp)
				break
			}
			break
		case 'm': // Metadata Actions
			core.LogDebug("Metadata Action: " + action)
			switch actionPostfix {
			case "n":
				name, ok1 := payloadObject["n"]
				if !ok1 {
					core.LogDebug("Metadata action missing required name field")
					break
				}
				nameStr, ok2 := name.(string)
				if !ok2 {
					core.LogDebug("Metadata action name field is not a string")
					break
				}
				nameStr = security.SanitizeNonPrintable(payloadObject["n"].(string))
				_Database.OnchainMN(blockchain, fromAddress, nameStr, timestamp)
				break
			case "a":
				avatar, ok1 := payloadObject["a"]
				if !ok1 {
					core.LogDebug("Metadata action missing required avatar field")
					break
				}
				avatarStr, ok2 := avatar.(string)
				if !ok2 {
					core.LogDebug("Metadata action avatar field is not a string")
					break
				}
				avatarStr = security.SanitizeNonPrintable(avatarStr)
				if security.IsValidURL(avatarStr) || security.IsValidCID(avatarStr) {
					_Database.OnchainMA(blockchain, fromAddress, avatarStr, timestamp)
				}
				break
			case "b":
				banner, ok1 := payloadObject["b"]
				if !ok1 {
					core.LogDebug("Metadata action missing required banner field")
					break
				}
				bannerStr, ok2 := banner.(string)
				if !ok2 {
					core.LogDebug("Metadata action banner field is not a string")
					break
				}
				bannerStr = security.SanitizeNonPrintable(bannerStr)
				if security.IsValidURL(bannerStr) || security.IsValidCID(bannerStr) {
					_Database.OnchainMB(blockchain, fromAddress, bannerStr, timestamp)
				}
				break
			case "bd":
				birthdate, ok1 := payloadObject["bd"]
				if !ok1 {
					core.LogDebug("Metadata action missing required birthdate field")
					break
				}
				birthdateStr, ok2 := birthdate.(string)
				if !ok2 {
					core.LogDebug("Metadata action birthdate field is not a string")
					break
				}
				birthdateInt, _err := strconv.ParseInt(birthdateStr, 10, 64)
				if _err != nil {
					core.LogDebug("Could not convert YourPlace transaction birthdate: " + _err.Error())
					break
				}
				if security.IsValidBirthDate(birthdateInt) {
					_Database.OnchainMBD(blockchain, fromAddress, uint64(birthdateInt), timestamp)
				}
				break
			case "l":
				location, ok1 := payloadObject["l"]
				if !ok1 {
					core.LogDebug("Metadata action missing required location field")
					break
				}
				locationStr, ok2 := location.(string)
				if !ok2 {
					core.LogDebug("Metadata action location field is not a string")
					break
				}
				locationStr = security.SanitizeNonPrintable(locationStr)
				_Database.OnchainML(blockchain, fromAddress, locationStr, timestamp)
				break
			case "w":
				website, ok1 := payloadObject["w"]
				if !ok1 {
					core.LogDebug("Metadata action missing required website field")
					break
				}
				websiteStr, ok2 := website.(string)
				if !ok2 {
					core.LogDebug("Metadata action website field is not a string")
					break
				}
				websiteStr = security.SanitizeNonPrintable(websiteStr)
				if security.IsValidURL(websiteStr) && len(websiteStr) > 0 {
					_Database.OnchainMW(blockchain, fromAddress, websiteStr, timestamp)
				}
				break
			case "d":
				description, ok1 := payloadObject["d"]
				if !ok1 {
					core.LogDebug("Metadata action missing required description field")
					break
				}
				descriptionStr, ok2 := description.(string)
				if !ok2 {
					core.LogDebug("Metadata action description field is not a string")
					break
				}
				descriptionStr = security.SanitizeNonPrintable(descriptionStr)
				if len(descriptionStr) > 0 {
					_Database.OnchainMD(blockchain, fromAddress, descriptionStr, timestamp)
				}
				break
			}
		case '$':
			switch actionPostfix {
			case "l":
				core.LogDebug("Marketplace Action: " + action)
			case "o": // Marketplace Offer
				core.LogDebug("Marketplace Action: " + action)
				listingTxHash, ok1 := payloadObject["l"]
				offerPrice, ok2 := payloadObject["p"]
				offerPriceSmallUnit, ok3 := payloadObject["ps"]
				if !ok1 || !ok2 || !ok3 {
					core.LogDebug("Marketplace offer missing required fields")
					break
				}
				listingTxHashStr, ok := listingTxHash.(string)
				if !ok || len(listingTxHashStr) == 0 {
					core.LogDebug("Invalid listing transaction hash in marketplace offer")
					break
				}
				offerPriceStr, ok := offerPrice.(string)
				if !ok || len(offerPriceStr) == 0 {
					core.LogDebug("Invalid offer price in marketplace offer")
					break
				}
				offerPriceSmallUnitStr, ok := offerPriceSmallUnit.(string)
				if !ok || len(offerPriceSmallUnitStr) == 0 {
					core.LogDebug("Invalid offer price small unit in marketplace offer")
					break
				}
				message := ""
				if msg, ok := payloadObject["m"]; ok {
					if msgStr, ok := msg.(string); ok && len(msgStr) <= 1000 {
						message = msgStr
					}
				}
				_Database.MarketplaceOffer(txHash, blockchain, fromAddress, toAddress, listingTxHashStr, offerPriceStr, offerPriceSmallUnitStr, message, timestamp)
				break
			case "oa": // Marketplace Offer Accept
				core.LogDebug("Marketplace Action: " + action)
				offerTxHash, ok1 := payloadObject["o"]
				if !ok1 {
					core.LogDebug("Marketplace offer accept missing required fields")
					break
				}
				offerTxHashStr, ok := offerTxHash.(string)
				if !ok || len(offerTxHashStr) == 0 {
					core.LogDebug("Invalid offer transaction hash in marketplace offer accept")
					break
				}
				_Database.MarketplaceOfferAccept(txHash, blockchain, fromAddress, toAddress, offerTxHashStr, timestamp)
				break
			case "p": // Marketplace Payment
				core.LogDebug("Marketplace Action: " + action)
				offerAcceptTxHash, ok1 := payloadObject["oa"]
				totalPrice, ok2 := payloadObject["tp"]
				totalPriceSmallUnit, ok3 := payloadObject["tps"]
				if !ok1 || !ok2 || !ok3 {
					core.LogDebug("Marketplace payment missing required fields")
					break
				}
				offerAcceptTxHashStr, ok := offerAcceptTxHash.(string)
				if !ok || len(offerAcceptTxHashStr) == 0 {
					core.LogDebug("Invalid offer accept transaction hash in marketplace payment")
					break
				}
				totalPriceStr, ok := totalPrice.(string)
				if !ok || len(totalPriceStr) == 0 {
					core.LogDebug("Invalid total price in marketplace payment")
					break
				}
				totalPriceSmallUnitStr, ok := totalPriceSmallUnit.(string)
				if !ok || len(totalPriceSmallUnitStr) == 0 {
					core.LogDebug("Invalid total price small unit in marketplace payment")
					break
				}
				_Database.MarketplacePayment(txHash, blockchain, fromAddress, toAddress, offerAcceptTxHashStr, totalPriceStr, totalPriceSmallUnitStr, timestamp)
				break
			case "r": // Marketplace Receipt
				core.LogDebug("Marketplace Action: " + action)
				paymentTxHash, ok1 := payloadObject["p"]
				if !ok1 {
					core.LogDebug("Marketplace receipt missing required fields")
					break
				}
				paymentTxHashStr, ok := paymentTxHash.(string)
				if !ok || len(paymentTxHashStr) == 0 {
					core.LogDebug("Invalid payment transaction hash in marketplace receipt")
					break
				}
				_Database.MarketplaceReceipt(txHash, blockchain, fromAddress, toAddress, paymentTxHashStr, timestamp)
				break
			case "al": // Marketplace Auction Listing (use regular listing with auction type)
				core.LogDebug("Marketplace Action: " + action)
				title, ok1 := payloadObject["t"]
				description, ok2 := payloadObject["d"]
				startPrice, ok3 := payloadObject["sp"]
				startPriceSmallUnit, ok4 := payloadObject["sps"]
				currencySymbol, ok5 := payloadObject["c"]
				if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
					core.LogDebug("Marketplace auction listing missing required fields")
					break
				}
				titleStr, ok := title.(string)
				if !ok || len(titleStr) == 0 || len(titleStr) > 200 {
					core.LogDebug("Invalid title in marketplace auction listing")
					break
				}
				descriptionStr, ok := description.(string)
				if !ok || len(descriptionStr) > 2000 {
					core.LogDebug("Invalid description in marketplace auction listing")
					break
				}
				startPriceStr, ok := startPrice.(string)
				if !ok || len(startPriceStr) == 0 {
					core.LogDebug("Invalid start price in marketplace auction listing")
					break
				}
				startPriceSmallUnitStr, ok := startPriceSmallUnit.(string)
				if !ok || len(startPriceSmallUnitStr) == 0 {
					core.LogDebug("Invalid start price small unit in marketplace auction listing")
					break
				}
				currencySymbolStr, ok := currencySymbol.(string)
				if !ok || len(currencySymbolStr) == 0 || len(currencySymbolStr) > 10 {
					core.LogDebug("Invalid currency symbol in marketplace auction listing")
					break
				}
				// Store auction as regular listing with auction type indicator
				imageUrls := []string{}
				_Database.MarketplaceListing(txHash, blockchain, fromAddress, toAddress, titleStr, descriptionStr, startPriceStr, startPriceSmallUnitStr, currencySymbolStr, imageUrls, timestamp)
				break
			case "lc": // Marketplace Listing Cancel
				core.LogDebug("Marketplace Action: " + action)
				listingTxHash, ok1 := payloadObject["l"]
				if !ok1 {
					core.LogDebug("Marketplace listing cancel missing required fields")
					break
				}
				listingTxHashStr, ok := listingTxHash.(string)
				if !ok || len(listingTxHashStr) == 0 {
					core.LogDebug("Invalid listing transaction hash in marketplace listing cancel")
					break
				}
				reason := ""
				if r, ok := payloadObject["r"]; ok {
					if reasonStr, ok := r.(string); ok && len(reasonStr) <= 500 {
						reason = reasonStr
					}
				}
				_Database.MarketplaceListingCancel(txHash, blockchain, fromAddress, toAddress, listingTxHashStr, reason, timestamp)
				break
			case "oc": // Marketplace Offer Cancel
				core.LogDebug("Marketplace Action: " + action)
				offerTxHash, ok1 := payloadObject["o"]
				if !ok1 {
					core.LogDebug("Marketplace offer cancel missing required fields")
					break
				}
				offerTxHashStr, ok := offerTxHash.(string)
				if !ok || len(offerTxHashStr) == 0 {
					core.LogDebug("Invalid offer transaction hash in marketplace offer cancel")
					break
				}
				reason := ""
				if r, ok := payloadObject["r"]; ok {
					if reasonStr, ok := r.(string); ok && len(reasonStr) <= 500 {
						reason = reasonStr
					}
				}
				_Database.MarketplaceOfferCancel(txHash, blockchain, fromAddress, toAddress, offerTxHashStr, reason, timestamp)
				break
			}
		default:
			core.LogError("Unknown YourPlace transaction action: " + action)
		}
	}
}
func rpcBatchGetBlockByNumber(uuid string, base *Base, batchBlockNumbers []big.Int) []map[string]interface{} {
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
	// Make the RPC call
	rpcErrorCount := 0
	var blocks []map[string]interface{}
BATCHRPCCALL:
	if breakPoint(uuid) {
		return nil
	}
	err := base.RpcClient.BatchCallContext(context.Background(), batch)
	if err != nil {
		core.LogDebug("Could not perform RPC call from rpcBatchGetBlockByNumber, backing off: " + err.Error())
		rpcErrorCount++
		backoff := (rpcErrorCount + 1) * 2
		time.Sleep(time.Duration(backoff) * time.Second) // exponential backoff
		if rpcErrorCount >= 120 {
			core.LogDebug("Backfill failed too many times: " + err.Error())
			_Database.IndexerUpdateJobStatus(uuid, "failed")
			return nil
		}
		goto BATCHRPCCALL
	}
	i := 0
	for _, elem := range batch { // Loop through each block in the batch response
		if breakPoint(uuid) {
			return nil
		}
		if elem.Error != nil {
			core.LogDebug("Could not get block data from rpcBatchGetBlockByNumber: " + elem.Error.Error())
			core.LogDebug("index: " + batchBlockNumbers[i].String())
			core.LogDebug("method: " + elem.Method)
			println("Args: ", elem.Args)
			println("result: ", elem.Result)
			blocks = append(blocks, nil) // Append nil to the blocks to maintain array alignment
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
	return blocks
}
func workerThread(uuid string, rateLimiter *rate.Limiter, base *Base, batchJobQueue *core.ThreadSafeQueue, sequentialTracker *SequentialBlockTracker,
	requestTracker *RequestTracker, txnCount *core.ThreadSafeCounter, databaseHistoryDaysInt int, targetEarliestBlock *big.Int,
	targetLatestBlock *big.Int, batchSize *big.Int, direction string, errorChan chan<- error) error {
	// Worker thread to process batches of blocks
	for {
		batch, populated := batchJobQueue.Dequeue()
		if !populated {
			core.LogDebug("Completed Worker Thread - No more batches to process")
			return nil
		}
		batchArray := batch.([]big.Int) // Get the batch of blocks
		// Wait for rate limit token immediately when we get work
		err := rateLimiter.Wait(context.Background())
		if err != nil {
			return core.LogErrorReturn("Rate limiter wait failed: " + err.Error())
		}
		// Record the requests being made
		requestTracker.RecordRequests(1)
		if globalRequestTracker != nil {
			globalRequestTracker.RecordRequests(1)
		}
		if breakPoint(sequentialTracker.uuid) { // Check for cancellation before RPC call
			return nil
		}
		// Make the RPC call
		blocks := rpcBatchGetBlockByNumber(uuid, base, batchArray)
		if blocks == nil {
			continue // Skip processing if blocks are nil
		}
		// Process each block and update the block index as we go
		for i, block := range blocks {
			if i >= len(batchArray) { // Safety check
				break
			}
			if block == nil {
				sequentialTracker.MarkBlockProcessed(batchArray[i].Int64(), direction)
				continue // Skip nil blocks
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
				ret := dispatchTransaction(block, transaction, &databaseHistoryDaysInt, big.NewInt(currentBlockNumber), "base")
				if ret == 1 || ret == 2 { // Skip transactions that are not valid YP posts
					continue
				}
				txnCount.Increment()
			}
			// Mark this block as processed in the sequential tracker
			sequentialTracker.MarkBlockProcessed(currentBlockNumber, direction)
			// Send status updates
			nextExpected := sequentialTracker.GetNextExpectedBlock()
			mod := nextExpected % reportInterval
			if mod == 0 {
				indexerPrintProgress(targetEarliestBlock, targetLatestBlock, big.NewInt(nextExpected), batchSize, direction, requestTracker)
			}
		}
	}
}
func breakPoint(uuid string) bool {
	// This function is a break point for the indexer to allow for graceful cancellation when the signal is received
	select {
	case <-indexerCancel:
		core.LogDebug("Indexer cancelled in break point")
		_Database.IndexerUpdateJobStatus(uuid, "failed")
		return true
	default:
		return false // continue processing
	}
}

// --- Global Helper Functions --- //
func IndexerClearOldCachedPosts(__database *db.Database) {} // Clear cached transactions that are older than the configured (expired) cached post history limit
func IndexerRestartJobs(__database *db.Database, blockchain string) {
	// set any indexer jobs to "failed" that were left in a "running" state from a crashed server
	jobUUID := __database.IndexerGetJobUUID(blockchain)
	__database.IndexerUpdateJobStatus(jobUUID, "failed")
}
func IndexerStop() {
	IndexerMutex.Lock()
	defer IndexerMutex.Unlock()
	core.LogDebug("Received Indexer Stop Request")
	if IsIndexing && indexerCancel != nil {
		indexerCancel = make(chan bool, 1)
		indexerCancel <- true
	}
}

// --- Transaction Parsing Functions --- //
func handlePostTransaction(payloadObject map[string]interface{}, txHash, blockchain, fromAddress, toAddress, parentTxHash string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
	postText, ok := payloadObject["p"]
	if !ok {
		core.LogDebug("Post Action: no p in payload")
		return false
	}
	postTextStr, ok := postText.(string)
	if !ok {
		core.LogDebug("Failed to convert post text to string")
		return false
	}
	postTextStr = security.SanitizeNonPrintable(postTextStr)
	_Database.OnchainP(txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, postTextStr)
	return true
}
func handlePostTransactionAttachment(payloadObject map[string]interface{}, txHash, blockchain, fromAddress, toAddress, parentTxHash string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
	postText, ok1 := payloadObject["p"]
	attachmentsRaw, ok2 := payloadObject["a"]
	if !ok1 || !ok2 {
		core.LogDebug("Post attach action missing required fields")
		return false
	}
	postTextStr, ok1 := postText.(string)
	attachmentsArray, ok2 := attachmentsRaw.([]interface{}) // ensures array json format for the array containing all attachments
	if !ok1 || !ok2 {
		core.LogDebug("Post attach action fields are not properly typed")
		return false
	}
	parsedAttachments := []db.Attachment{}
	for _, attachment := range attachmentsArray {
		attachmentArray, ok := attachment.([]interface{}) //ensures array json format for each individual attachment
		if !ok {
			core.LogDebug("Post attach action fields are not array")
			return false
		}
		parsedURL, okURL := attachmentArray[0].(string)
		parsedMimeType, okMimeType := attachmentArray[1].(string)
		sizeFloat, okSize := attachmentArray[2].(float64)
		fileName, okFileName := attachmentArray[3].(string)
		if !okURL || !okMimeType || !okSize || !okFileName {
			core.LogDebug("Post attach array values are not properly typed")
			return false
		}
		if !security.IsValidIndexedFilename(fileName) {
			core.LogDebug("Post attach action does not contain a valid filename")
			return false
		}
		if !security.IsValidURL(parsedURL) && !security.IsValidCID(parsedURL) {
			core.LogDebug("Post attach action does not contain a valid URL or CID")
			return false
		}
		if sizeFloat < 0 {
			core.LogDebug("Post attach action contains negative file size")
			return false
		}
		sizeUint := uint64(sizeFloat)
		parsedAttachment := db.Attachment{
			FileURL:  parsedURL,
			MimeType: parsedMimeType,
			FileSize: sizeUint,
			FileName: fileName,
		}
		parsedAttachments = append(parsedAttachments, parsedAttachment)
	}
	postTextStr = security.SanitizeNonPrintable(postTextStr)
	_Database.OnchainPA(txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, postTextStr, parsedAttachments)
	return true
}
