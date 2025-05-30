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
	batchSizeLimit = 10   // The maximum number of blocks to fetch in a single batch RPC call
	workerCount    = 10   // Number of worker threads to use for processing batches
)

var (
	indexerCancel chan bool
	IndexerMutex  sync.Mutex
	IsIndexing    bool
	_Blockchain   *Blockchain
	_Database     *db.Database
)

type SequentialBlockTracker struct {
	mu                sync.Mutex
	processedBlocks   map[int64]bool
	nextExpectedBlock int64
	uuid              string
	database          *db.Database
}

func NewSequentialBlockTracker(startBlock int64, uuid string, database *db.Database) *SequentialBlockTracker {
	return &SequentialBlockTracker{
		processedBlocks:   make(map[int64]bool),
		nextExpectedBlock: startBlock,
		uuid:              uuid,
		database:          database,
	}
}
func (sbt *SequentialBlockTracker) MarkBlockProcessed(blockNumber int64) {
	sbt.mu.Lock()
	defer sbt.mu.Unlock()
	sbt.processedBlocks[blockNumber] = true
	// Update nextExpectedBlock to the next unprocessed sequential block
	for sbt.processedBlocks[sbt.nextExpectedBlock] {
		delete(sbt.processedBlocks, sbt.nextExpectedBlock) // Remove processed blocks from the map
		sbt.nextExpectedBlock--                            // Move to the next expected block
		if sbt.nextExpectedBlock%saveInterval == 0 {       // Check if we should save progress
			sbt.database.IndexerUpdateTailBlock(sbt.uuid, uint64(sbt.nextExpectedBlock))
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

// --- Indexer Main Method --- //
func IndexerFetchData(database *db.Database, blockchain *Blockchain, chainName string) {
	_Blockchain = blockchain
	_Database = database
	databaseStatus, uuid, chainLatestBlock, databaseTailBlock, databaseHeadBlock, chainEarliestBlock := indexerPreflight(chainName)
	if uuid == "" || databaseStatus == "" {
		return // bail out if the preflight bails out
	}
	_ = databaseHeadBlock   // todo - placeholder for head block use later on
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
	rateLimiter := configureRateLimiter(baseThrottle, batchSize)   // Configure a ratelimiter based on throttle and batch size
	batchStartBlock := new(big.Int).Set(targetEarliestBlockBigInt) // Start at the earliest block
	core.LogDebug("Batch Start Block: " + batchStartBlock.String())
	txnCount := 0
	blockIndex := core.NewThreadSafeMaxTracker(batchStartBlock.Int64()) // Running tally of the current block
	blockIndexTempBigInt := big.NewInt(blockIndex.Get())
	core.LogDebug("Block Index: " + blockIndexTempBigInt.String())

	for i := 1; i <= int(batchCount.Int64()); i++ { // Loop over batches of blocks
		if breakPoint(uuid) {
			return
		}
		_ = rateLimiter.Wait(context.Background())
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
		// Make the RPC call
		blocks := rpcBatchGetBlockByNumber(base, batchBlockNumbers)
		// Loop through blocks
		for _, block := range blocks {
			transactions := block["transactions"].([]interface{})
			// Loop through transactions in the block
			for _, txn := range transactions {
				transaction := txn.(map[string]interface{})                                                           // Get a handle on the transaction
				ret := dispatchTransaction(block, transaction, &databaseHistoryDaysInt, blockIndexTempBigInt, "base") // Dispatch the transaction to the database
				if ret == 1 || ret == 2 {                                                                             // Skip transactions that are not valid YP posts
					continue
				}
				txnCount++
			}
			// Send a status update
			mod := big.NewInt(0)
			mod.Mod(blockIndexTempBigInt, big.NewInt(reportInterval))
			if mod.Sign() == 0 {
				indexerPrintProgress(targetEarliestBlockBigInt, targetLatestBlock, blockIndexTempBigInt, batchSize, "forward")
			}
			mod.Mod(blockIndexTempBigInt, big.NewInt(saveInterval))
			if mod.Sign() == 0 {
				_Database.IndexerUpdateHeadBlock(uuid, blockIndexTempBigInt.Uint64())
			}
		}
		batchStartBlock = new(big.Int).Add(batchStartBlock, batchSize)
	}
	core.LogDebug("Completed Front Fill - Updating Head Block: " + blockIndexTempBigInt.String())
	_Database.IndexerUpdateHeadBlock(uuid, blockIndexTempBigInt.Uint64())
	_Database.IndexerUpdateJobStatus(uuid, "complete")
}
func IndexerBaseBackFill(base *Base, uuid string, baseLatestBlock *big.Int) {
	// earliest block <----- tail block (starting traversal @ tail block)—
	core.LogDebug("--- IndexerBaseBackFill() ---")
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
	rateLimiter := configureRateLimiter(baseThrottle, batchSize)                               // Configure rate limiter based on throttle and batch size
	batchStartBlock := targetLatestBlock                                                       // Start at the latest block
	txnCount := core.NewThreadSafeCounter()                                                    // Count the number of transactions processed
	batchJobQueue := core.NewThreadSafeQueue()                                                 // Queue to hold batch jobs
	sequentialTracker := NewSequentialBlockTracker(targetLatestBlock.Int64(), uuid, _Database) // Sequential block tracker starting from the highest block
	errorChan := make(chan error, workerCount)                                                 // Channel to handle errors from workers

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
			err := workerThread(rateLimiter, base, batchJobQueue, sequentialTracker, txnCount, databaseHistoryDaysInt, targetEarliestBlock, targetLatestBlock, batchSize, "backward", errorChan)
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
	rateLimiter := configureRateLimiter(baseThrottle, batchSize)        // Configure a rate limiter based on throttle and batch size
	batchStartBlock := targetLatestBlock                                // Start at the latest block
	txnCount := 0                                                       // Count the number of transactions processed
	blockIndex := core.NewThreadSafeMinTracker(batchStartBlock.Int64()) // Running tally of the current block
	blockIndexTempBigInt := big.NewInt(blockIndex.Get())

	for i := 1; i <= int(batchCount.Int64()); i++ { // Loop over batches of blocks
		if breakPoint(uuid) {
			return
		}
		_ = rateLimiter.Wait(context.Background())
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
		// Make the RPC call
		blocks := rpcBatchGetBlockByNumber(base, batchBlockNumbers) // Returns an array of blocks
		// Loop through blocks
		for _, block := range blocks {
			transactions := block["transactions"].([]interface{}) // Get the transactions from the block
			// Loop through transactions in the block
			for _, txn := range transactions {
				transaction := txn.(map[string]interface{})                                                           // Get a handle on the transaction
				ret := dispatchTransaction(block, transaction, &databaseHistoryDaysInt, blockIndexTempBigInt, "base") // Dispatch the transaction to the database
				if ret == 1 || ret == 2 {                                                                             // Skip transactions that are not valid YP posts
					continue
				}
				txnCount++
			}
			_blockIndexTempBigInt := big.NewInt(int64(blockIndexTempBigInt.Uint64() - 1))
			mod := big.NewInt(0) // Send a status update
			mod.Mod(_blockIndexTempBigInt, big.NewInt(reportInterval))
			if mod.Sign() == 0 {
				indexerPrintProgress(&targetEarliestBlock, targetLatestBlock, blockIndexTempBigInt, batchSize, "backward")
			}
			mod.Mod(blockIndexTempBigInt, big.NewInt(saveInterval))
			if mod.Sign() == 0 {
				_Database.IndexerUpdateTailBlock(uuid, blockIndexTempBigInt.Uint64())
			}
		}
		batchStartBlock = new(big.Int).Sub(batchStartBlock, batchSize)
	}
	//core.LogDebug("Completed Full Fill - Updating Tail Block: " + blockIndexTempBigInt.String()) debug
	//_Database.IndexerUpdateTailBlock(uuid, blockIndex.Uint64()) debug
	//_Database.IndexerUpdateJobStatus(uuid, "complete") debug
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
func indexerPrintProgress(targetEarliestBlock *big.Int, targetLatestBlock *big.Int, blockIndex *big.Int, batchSize *big.Int, traversalDirection string) {
	core.LogDebug("------------------------")
	core.LogDebug("index: " + blockIndex.String())
	core.LogDebug("target latest: " + targetLatestBlock.String())
	core.LogDebug("target earliest: " + targetEarliestBlock.String())
	totalRange := new(big.Int).Sub(targetLatestBlock, targetEarliestBlock)
	core.LogDebug("total range: " + totalRange.String())
	var progressMade *big.Int
	if traversalDirection == "forward" {
		progressMade = new(big.Int).Sub(blockIndex, targetEarliestBlock)
	} else {
		progressMade = new(big.Int).Sub(targetLatestBlock, blockIndex)
	}
	core.LogDebug("progress made: " + progressMade.String())
	progressPercent := calculatePercentage(totalRange, progressMade)
	core.LogDebug("progress: " + progressPercent + " %")
	progressRemaining := new(big.Int).Sub(totalRange, progressMade)
	core.LogDebug("progress remaining: " + progressRemaining.String())
	batchesRemaining := new(big.Int).Div(progressRemaining, batchSize)
	batchSizeRemainder := new(big.Int).Mod(progressRemaining, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchesRemaining.Add(batchesRemaining, big.NewInt(1))
	}
	core.LogDebug("batches remaining: " + batchesRemaining.String())
	core.LogDebug("batch size: " + batchSize.String())
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
func configureRateLimiter(throttleValue int, batchSize *big.Int) *rate.Limiter { // Function to configure rate limiter based on throttle and batch size
	requestsPerSecond := float64(throttleValue) / float64(batchSize.Int64())
	if requestsPerSecond < 1.0 {
		requestsPerSecond = 1.0 // Minimum of 1 request per second
	}
	return rate.NewLimiter(rate.Limit(requestsPerSecond), int(requestsPerSecond)+1)
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
	core.LogDebug("Action: " + action)
	if len(action) < 1 {
		core.LogError("Invalid YourPlace transaction action: " + action)
		return
	}
	actionPrefix := action[0] // parse out the action prefix
	core.LogDebug("Action Prefix: " + strconv.FormatUint(uint64(actionPrefix), 10))
	actionPostfix := action[1:] // parse out the action postfix
	core.LogDebug("Action Postfix: " + actionPostfix)
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
			break
		case 'b': // Blocking Actions
		case 's': // Settings Actions
		default:
			core.LogError("Unknown YourPlace transaction action: " + action)
		}
	}
}
func rpcBatchGetBlockByNumber(base *Base, batchBlockNumbers []big.Int) []map[string]interface{} {
	// Get the necessary variables
	uuid := _Database.IndexerGetJobUUID("base")
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
	select {
	case <-indexerCancel:
		core.LogDebug("Indexer cancelled during batch RPC call")
		_Database.IndexerUpdateJobStatus(uuid, "failed")
		return nil
	default:
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
	}
	i := 0
	for _, elem := range batch { // Loop through each block in the batch response
		select {
		case <-indexerCancel:
			core.LogDebug("Indexer cancelled in backfill during batch processing")
			_Database.IndexerUpdateJobStatus(uuid, "failed")
			return nil
		default:
			if elem.Error != nil {
				core.LogDebug("Could not get block data from rpcBatchGetBlockByNumber: " + elem.Error.Error())
				core.LogDebug("index: " + batchBlockNumbers[i].String())
				core.LogDebug("method: " + elem.Method)
				println("Args: ", elem.Args)
				println("result: ", elem.Result)
			}
			block := *elem.Result.(*map[string]interface{})
			blocks = append(blocks, block)
		}
		i++
	}
	return blocks
}
func workerThread(rateLimiter *rate.Limiter, base *Base, batchJobQueue *core.ThreadSafeQueue, sequentialTracker *SequentialBlockTracker, txnCount *core.ThreadSafeCounter, databaseHistoryDaysInt int, targetEarliestBlock *big.Int, targetLatestBlock *big.Int, batchSize *big.Int, direction string, errorChan chan<- error) error {
	// Worker thread to process batches of blocks
	core.LogDebug("Worker Thread Spawn")
	for {
		batch, populated := batchJobQueue.Dequeue()
		if !populated {
			core.LogDebug("Completed Worker Thread - No more batches to process")
			return nil
		}
		batchArray := batch.([]big.Int) // Get the batch of blocks
		// Reserve rate limit tokens for all requests in this batch
		reservation := rateLimiter.ReserveN(time.Now(), len(batchArray))
		if !reservation.OK() {
			return core.LogErrorReturn("Rate limiter reservation failed")
		}
		time.Sleep(reservation.Delay())
		//core.LogDebug("\tProcessing batch of blocks: " + batchArray[0].String() + " to " + batchArray[len(batchArray)-1].String())
		// Make the RPC call
		blocks := rpcBatchGetBlockByNumber(base, batchArray)
		// Process each block and update the block index as we go
		for i, block := range blocks {
			currentBlockNumber := batchArray[i].Int64()
			transactions := block["transactions"].([]interface{}) // Get the transactions from the block
			// Loop through transactions in the block
			for _, txn := range transactions {
				transaction := txn.(map[string]interface{})
				ret := dispatchTransaction(block, transaction, &databaseHistoryDaysInt, big.NewInt(currentBlockNumber), "base")
				if ret == 1 || ret == 2 { // Skip transactions that are not valid YP posts
					continue
				}
				txnCount.Increment()
			}
			// Mark this block as processed in the sequential tracker
			sequentialTracker.MarkBlockProcessed(currentBlockNumber)
			// Send status updates
			nextExpected := sequentialTracker.GetNextExpectedBlock()
			mod := nextExpected % reportInterval
			if mod == 0 {
				indexerPrintProgress(targetEarliestBlock, targetLatestBlock, big.NewInt(nextExpected), batchSize, direction)
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
		if !okURL || !okMimeType || !okSize {
			core.LogDebug("Post attach array values are not properly typed")
			return false
		}
		sizeUint := uint64(sizeFloat)
		parsedAttachment := db.Attachment{
			FileURL:  parsedURL,
			MimeType: parsedMimeType,
			FileSize: sizeUint,
		}
		parsedAttachments = append(parsedAttachments, parsedAttachment)
	}
	_Database.OnchainPA(txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, postTextStr, parsedAttachments)
	return true
}
