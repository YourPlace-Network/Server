package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"golang.org/x/time/rate"
)

const (
	algoReportInterval = 10000
	algoSaveInterval   = 200
	algoThrottleOffset = 2
	algoBatchSizeLimit = 10
	algoWorkerCount    = 5
)

var (
	algoIndexerCancel             chan bool
	AlgoIndexerMutex              sync.Mutex
	IsAlgoIndexing                bool
	_AlgoBlockchain               *Blockchain
	_AlgoDatabase                 *db.Database
	algoDynamicThrottleMultiplier = 1.0
	algoThrottleControlMutex      sync.RWMutex
	algoGlobalRequestTracker      *RequestTracker
	algoRateLimiterMutex          sync.Mutex
	algoActiveRequestsCount       int64
	algoProgressLogMutex          sync.Mutex
	algoLastProgressBlock         int64
	algoTotalRequestsCount        int64
	AlgoEarliestBlock             = big.NewInt(56000000) // Algorand YourPlace genesis block (approximate mainnet block)
)

type AlgoSequentialBlockTracker struct {
	mu                sync.RWMutex
	processedBlocks   map[int64]bool
	nextExpectedBlock int64
	uuid              string
	database          *db.Database
	direction         string
}

func NewAlgoSequentialBlockTracker(startBlock int64, uuid string, database *db.Database, direction string) *AlgoSequentialBlockTracker {
	return &AlgoSequentialBlockTracker{
		processedBlocks:   make(map[int64]bool),
		nextExpectedBlock: startBlock,
		uuid:              uuid,
		database:          database,
		direction:         direction,
	}
}
func (sbt *AlgoSequentialBlockTracker) MarkBlockProcessed(blockNumber int64, direction string) {
	sbt.mu.Lock()
	defer sbt.mu.Unlock()
	sbt.processedBlocks[blockNumber] = true
	if direction == "forward" {
		for sbt.processedBlocks[sbt.nextExpectedBlock] {
			delete(sbt.processedBlocks, sbt.nextExpectedBlock)
			sbt.nextExpectedBlock++
			if sbt.nextExpectedBlock%algoSaveInterval == 0 {
				sbt.database.IndexerUpdateHeadBlock(sbt.uuid, uint64(sbt.nextExpectedBlock))
			}
		}
	} else {
		for sbt.processedBlocks[sbt.nextExpectedBlock] {
			delete(sbt.processedBlocks, sbt.nextExpectedBlock)
			sbt.nextExpectedBlock--
			if sbt.nextExpectedBlock%algoSaveInterval == 0 {
				sbt.database.IndexerUpdateTailBlock(sbt.uuid, uint64(sbt.nextExpectedBlock))
			}
		}
	}
}
func (sbt *AlgoSequentialBlockTracker) GetNextExpectedBlock() int64 {
	sbt.mu.RLock()
	defer sbt.mu.RUnlock()
	return sbt.nextExpectedBlock
}
func (sbt *AlgoSequentialBlockTracker) HasPendingBlocks() bool {
	sbt.mu.RLock()
	defer sbt.mu.RUnlock()
	return len(sbt.processedBlocks) > 0
}

// --- Algorand Indexer Main Method --- //
func AlgorandIndexerFetchData(database *db.Database, blockchain *Blockchain) bool {
	chainName := "algorand"
	_AlgoBlockchain = blockchain
	_AlgoDatabase = database
	databaseStatus, uuid, chainLatestBlock, databaseTailBlock, databaseHeadBlock, chainEarliestBlock := algoIndexerPreflight(chainName)
	if uuid == "" || databaseStatus == "" {
		return false
	}
	_ = databaseHeadBlock
	switch databaseStatus {
	case "pending":
		core.LogDebug("[Algo] Starting pending job from the beginning")
		IndexerAlgorandFullFill(blockchain.Algorand, uuid, chainLatestBlock, database)
	case "failed":
		core.LogDebug("[Algo] Restarting failed job from where it left off")
		if databaseTailBlock == 0 {
			AlgoIndexerRestartJobs(_AlgoDatabase, chainName)
			IndexerAlgorandFullFill(blockchain.Algorand, uuid, chainLatestBlock, database)
			return true
		}
		if databaseTailBlock > chainEarliestBlock.Uint64() {
			IndexerAlgorandBackFill(blockchain.Algorand, uuid, chainLatestBlock, database)
		} else {
			IndexerAlgorandFrontFill(blockchain.Algorand, uuid, chainLatestBlock, database)
		}
	case "complete":
		core.LogDebug("[Algo] Last job completed successfully. Getting new blocks")
		IndexerAlgorandFrontFill(blockchain.Algorand, uuid, chainLatestBlock, database)
	}
	return true
}

// --- Algorand Indexer Functions --- //
func IndexerAlgorandFrontFill(algo *Algorand, uuid string, algoLatestBlock *big.Int, database *db.Database) {
	core.LogDebug("[Algo] --- IndexerAlgorandFrontFill()")
	direction := "forward"
	_AlgoDatabase.IndexerUpdateJobStatus(uuid, "running")
	headBlock := _AlgoDatabase.IndexerGetHeadBlock(uuid)
	if headBlock <= 0 {
		core.LogWarn("[Algo] IndexerAlgorandFrontFill(): Head block is <= 0 - aborting")
		return
	}
	targetLatestBlock := algoLatestBlock
	targetEarliestBlock := headBlock
	core.LogDebug("[Algo] Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("[Algo] Target Earliest Block: " + strconv.Itoa(int(targetEarliestBlock)))
	targetEarliestBlockBigInt := big.NewInt(int64(targetEarliestBlock))
	algoThrottle, _ := strconv.Atoi(_AlgoDatabase.SettingsGetValue("algoThrottle"))
	if algoThrottle == 0 {
		algoThrottle, _ = strconv.Atoi(DefaultBlockchainNodes["algorand"][1])
	}
	algoThrottle = int(float64(algoThrottle) * 0.95)
	core.LogDebug("[Algo] Throttle: " + strconv.Itoa(algoThrottle))
	batchSize := algoCalculateOptimalBatchSize(algoThrottle)
	core.LogDebug("[Algo] Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, targetEarliestBlockBigInt)
	if blockCount.Int64() <= 0 {
		core.LogDebug("[Algo] Block count is negative or zero - likely up to date")
		_AlgoDatabase.IndexerUpdateJobStatus(uuid, "complete")
		return
	}
	core.LogDebug("[Algo] Number of Blocks: " + blockCount.String())
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("[Algo] Batch Count: " + batchCount.String())
	rateLimiter := algoConfigureRateLimiter(algoThrottle)
	batchStartBlock := new(big.Int).Set(targetEarliestBlockBigInt)
	txnCount := core.NewThreadSafeCounter()
	batchJobQueue := core.NewThreadSafeQueue()
	sequentialTracker := NewAlgoSequentialBlockTracker(targetEarliestBlockBigInt.Int64(), uuid, _AlgoDatabase, direction)
	algoGlobalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, algoWorkerCount)
	atomic.StoreInt64(&algoActiveRequestsCount, 0)
	atomic.StoreInt64(&algoTotalRequestsCount, 0)
	atomic.StoreInt64(&algoLastProgressBlock, 0)
	go algoStartThrottleController(uuid, algoThrottle, rateLimiter, database)
	var wg sync.WaitGroup
	for i := 0; i < algoWorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			baseDelay := time.Duration(workerID) * 500 * time.Millisecond
			time.Sleep(baseDelay)
			defer wg.Done()
			err := algoWorkerThread(uuid, rateLimiter, algo, batchJobQueue, sequentialTracker, algoGlobalRequestTracker, txnCount, targetEarliestBlockBigInt, targetLatestBlock, batchSize, "forward")
			if err != nil {
				core.LogError("[Algo] Worker thread failed: " + err.Error())
				errorChan <- err
			}
		}(i)
	}
	go func() {
		for i := 1; i <= int(batchCount.Int64()); i++ {
			if algoBreakPoint(uuid) {
				return
			}
			batchEndBlock := new(big.Int).Add(batchStartBlock, batchSize)
			if batchEndBlock.Cmp(targetLatestBlock) == 1 {
				batchEndBlock = targetLatestBlock
			}
			if batchStartBlock.Cmp(batchEndBlock) >= 0 {
				break
			}
			var batchBlockNumbers []big.Int
			for j := new(big.Int).Set(batchStartBlock); j.Cmp(batchEndBlock) == -1; j = new(big.Int).Add(j, big.NewInt(1)) {
				batchBlockNumbers = append(batchBlockNumbers, *j)
			}
			batchJobQueue.Enqueue(batchBlockNumbers)
			batchStartBlock = new(big.Int).Add(batchStartBlock, batchSize)
		}
	}()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case err := <-errorChan:
		core.LogError("[Algo] Worker thread failed: " + err.Error())
		_AlgoDatabase.IndexerUpdateJobStatus(uuid, "failed")
		return
	case <-done:
		break
	}
	finalBlockIndex := sequentialTracker.GetNextExpectedBlock()
	_AlgoDatabase.IndexerUpdateHeadBlock(uuid, uint64(finalBlockIndex))
	_AlgoDatabase.IndexerUpdateJobStatus(uuid, "complete")
}
func IndexerAlgorandBackFill(algo *Algorand, uuid string, algoLatestBlock *big.Int, database *db.Database) {
	core.LogDebug("[Algo] --- IndexerAlgorandBackFill() ---")
	direction := "backward"
	_AlgoDatabase.IndexerUpdateJobStatus(uuid, "running")
	databaseTailBlock := big.NewInt(int64(_AlgoDatabase.IndexerGetTailBlock(uuid)))
	if databaseTailBlock.Cmp(big.NewInt(0)) == 0 {
		core.LogDebug("[Algo] Database Tail Block is 0 - setting to Head Block")
		headBlockInt := _AlgoDatabase.IndexerGetHeadBlock(uuid)
		databaseTailBlock = big.NewInt(int64(headBlockInt))
		core.LogDebug("[Algo] Database Tail Block: " + databaseTailBlock.String())
	}
	targetLatestBlock := databaseTailBlock
	targetEarliestBlock := AlgoEarliestBlock
	if targetLatestBlock.Cmp(targetEarliestBlock) == 0 {
		core.LogDebug("[Algo] Target latest block is equal to target earliest block - completing")
		_AlgoDatabase.IndexerUpdateJobStatus(uuid, "complete")
		return
	}
	if targetLatestBlock.Int64() == 0 {
		targetLatestBlock = algoLatestBlock
		targetEarliestBlock = AlgoEarliestBlock
	}
	core.LogDebug("[Algo] Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("[Algo] Target Earliest Block: " + targetEarliestBlock.String())
	algoThrottle, _ := strconv.Atoi(_AlgoDatabase.SettingsGetValue("algoThrottle"))
	if algoThrottle == 0 {
		algoThrottle, _ = strconv.Atoi(DefaultBlockchainNodes["algorand"][1])
	}
	algoThrottle = int(float64(algoThrottle) * 0.95)
	core.LogDebug("[Algo] Throttle: " + strconv.Itoa(algoThrottle))
	batchSize := algoCalculateOptimalBatchSize(algoThrottle)
	core.LogDebug("[Algo] Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, targetEarliestBlock)
	core.LogDebug("[Algo] Block Count: " + blockCount.String())
	if blockCount.Int64() <= 0 {
		core.LogError("[Algo] Block count is negative or zero")
		return
	}
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("[Algo] Batch Count: " + batchCount.String())
	rateLimiter := algoConfigureRateLimiter(algoThrottle)
	batchStartBlock := targetLatestBlock
	txnCount := core.NewThreadSafeCounter()
	batchJobQueue := core.NewThreadSafeQueue()
	sequentialTracker := NewAlgoSequentialBlockTracker(targetLatestBlock.Int64(), uuid, _AlgoDatabase, direction)
	algoGlobalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, algoWorkerCount)
	atomic.StoreInt64(&algoActiveRequestsCount, 0)
	atomic.StoreInt64(&algoTotalRequestsCount, 0)
	go algoStartThrottleController(uuid, algoThrottle, rateLimiter, database)
	var wg sync.WaitGroup
	for i := 0; i < algoWorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			baseDelay := time.Duration(workerID) * 500 * time.Millisecond
			time.Sleep(baseDelay)
			defer wg.Done()
			err := algoWorkerThread(uuid, rateLimiter, algo, batchJobQueue, sequentialTracker, algoGlobalRequestTracker, txnCount, targetEarliestBlock, targetLatestBlock, batchSize, "backward")
			if err != nil {
				core.LogError("[Algo] Worker encountered an error: " + err.Error())
				errorChan <- err
			}
		}(i)
	}
	go func() {
		for i := 1; i <= int(batchCount.Int64()); i++ {
			if algoBreakPoint(uuid) {
				return
			}
			batchEndBlock := new(big.Int).Sub(batchStartBlock, batchSize)
			if batchEndBlock.Cmp(targetEarliestBlock) == -1 {
				batchEndBlock = targetEarliestBlock
			}
			if batchStartBlock.Cmp(batchEndBlock) <= 0 {
				break
			}
			var batchBlockNumbers []big.Int
			for j := batchStartBlock; j.Cmp(batchEndBlock) == 1; j = new(big.Int).Sub(j, big.NewInt(1)) {
				batchBlockNumbers = append(batchBlockNumbers, *j)
			}
			batchJobQueue.Enqueue(batchBlockNumbers)
			batchStartBlock = new(big.Int).Sub(batchStartBlock, batchSize)
		}
	}()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case err := <-errorChan:
		core.LogError("[Algo] Worker thread failed: " + err.Error())
		_AlgoDatabase.IndexerUpdateJobStatus(uuid, "failed")
		return
	case <-done:
		break
	}
	finalBlockIndex := sequentialTracker.GetNextExpectedBlock()
	_AlgoDatabase.IndexerUpdateTailBlock(uuid, uint64(finalBlockIndex))
	_AlgoDatabase.IndexerUpdateJobStatus(uuid, "complete")
}
func IndexerAlgorandFullFill(algo *Algorand, uuid string, algoLatestBlock *big.Int, database *db.Database) {
	core.LogDebug("[Algo] --- IndexerAlgorandFullFill()")
	direction := "backward"
	_AlgoDatabase.IndexerUpdateJobStatus(uuid, "running")
	targetLatestBlock := algoLatestBlock
	core.LogDebug("[Algo] Target Latest Block: " + targetLatestBlock.String())
	targetEarliestBlock := AlgoGetEarliestBlock()
	core.LogDebug("[Algo] Target Earliest Block: " + targetEarliestBlock.String())
	targetEarliestBlockBigInt := targetEarliestBlock
	_AlgoDatabase.IndexerUpdateHeadBlock(uuid, targetLatestBlock.Uint64())
	algoThrottle, _ := strconv.Atoi(_AlgoDatabase.SettingsGetValue("algoThrottle"))
	if algoThrottle == 0 {
		algoThrottle, _ = strconv.Atoi(DefaultBlockchainNodes["algorand"][1])
	}
	algoThrottle = int(float64(algoThrottle) * 0.95)
	core.LogDebug("[Algo] Throttle: " + strconv.Itoa(algoThrottle))
	batchSize := algoCalculateOptimalBatchSize(algoThrottle)
	core.LogDebug("[Algo] Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, &targetEarliestBlockBigInt)
	if blockCount.Int64() <= 0 {
		core.LogError("[Algo] Block count is negative or zero")
		return
	}
	core.LogDebug("[Algo] Number of Blocks: " + blockCount.String())
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("[Algo] Batch Count: " + batchCount.String())
	rateLimiter := algoConfigureRateLimiter(algoThrottle)
	batchStartBlock := targetLatestBlock
	txnCount := core.NewThreadSafeCounter()
	batchJobQueue := core.NewThreadSafeQueue()
	sequentialTracker := NewAlgoSequentialBlockTracker(targetLatestBlock.Int64(), uuid, _AlgoDatabase, direction)
	algoGlobalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, algoWorkerCount)
	atomic.StoreInt64(&algoActiveRequestsCount, 0)
	atomic.StoreInt64(&algoTotalRequestsCount, 0)
	atomic.StoreInt64(&algoLastProgressBlock, 0)
	go algoStartThrottleController(uuid, algoThrottle, rateLimiter, database)
	var wg sync.WaitGroup
	for i := 0; i < algoWorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			baseDelay := time.Duration(workerID) * 500 * time.Millisecond
			time.Sleep(baseDelay)
			defer wg.Done()
			err := algoWorkerThread(uuid, rateLimiter, algo, batchJobQueue, sequentialTracker, algoGlobalRequestTracker, txnCount, &targetEarliestBlockBigInt, targetLatestBlock, batchSize, "backward")
			if err != nil {
				core.LogError("[Algo] Worker encountered an error: " + err.Error())
				errorChan <- err
			}
		}(i)
	}
	go func() {
		for i := 1; i <= int(batchCount.Int64()); i++ {
			if algoBreakPoint(uuid) {
				return
			}
			batchEndBlock := new(big.Int).Sub(batchStartBlock, batchSize)
			if batchEndBlock.Cmp(&targetEarliestBlockBigInt) == -1 {
				batchEndBlock = &targetEarliestBlockBigInt
			}
			if batchStartBlock.Cmp(batchEndBlock) < 0 {
				break
			}
			var batchBlockNumbers []big.Int
			for j := batchStartBlock; j.Cmp(batchEndBlock) == 1; j = new(big.Int).Sub(j, big.NewInt(1)) {
				batchBlockNumbers = append(batchBlockNumbers, *j)
			}
			batchJobQueue.Enqueue(batchBlockNumbers)
			batchStartBlock = new(big.Int).Sub(batchStartBlock, batchSize)
		}
	}()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case err := <-errorChan:
		core.LogError("[Algo] Worker thread failed: " + err.Error())
		_AlgoDatabase.IndexerUpdateJobStatus(uuid, "failed")
		return
	case <-done:
		break
	}
	finalBlockIndex := sequentialTracker.GetNextExpectedBlock()
	_AlgoDatabase.IndexerUpdateTailBlock(uuid, uint64(finalBlockIndex))
	_AlgoDatabase.IndexerUpdateJobStatus(uuid, "complete")
}

// --- Algorand Helper Functions --- //
func algoIndexerPreflight(chainName string) (string, string, *big.Int, uint64, uint64, *big.Int) {
	algoIndexerCancel = make(chan bool, 1)
	AlgoIndexerMutex.Lock()
	if IsAlgoIndexing {
		AlgoIndexerMutex.Unlock()
		return "", "", nil, 0, 0, nil
	}
	IsAlgoIndexing = true
	AlgoIndexerMutex.Unlock()
	defer func() {
		AlgoIndexerMutex.Lock()
		IsAlgoIndexing = false
		AlgoIndexerMutex.Unlock()
	}()
	globalIndexerRunning := _AlgoDatabase.SettingsGetValue("indexerRunning")
	algoIndexerRunning := _AlgoDatabase.SettingsGetValue("algoIndexerRunning")
	if globalIndexerRunning != "true" {
		return "", "", nil, 0, 0, nil
	}
	if algoIndexerRunning == "false" {
		return "", "", nil, 0, 0, nil
	}
	uuid := _AlgoDatabase.IndexerGetJobUUID(chainName)
	if uuid == "" {
		uuid = algoCreateIndexerJob(chainName)
	}
	databaseStatus := _AlgoDatabase.IndexerGetJobStatus(uuid)
	if databaseStatus == "running" {
		return "", "", nil, 0, 0, nil
	}
	chainLatestBlock, err := _AlgoBlockchain.GetLatestBlock(chainName)
	if err != nil {
		core.LogDebug("[Algo] Could not get latest block number: GetLatestBlock returned error: " + err.Error())
		return "", "", nil, 0, 0, nil
	}
	if chainLatestBlock.Cmp(big.NewInt(0)) == 0 {
		core.LogDebug("[Algo] Could not get latest block number: " + chainLatestBlock.String())
		return "", "", nil, 0, 0, nil
	}
	chainEarliestBlock := AlgoGetEarliestBlock()
	databaseHeadBlock := _AlgoDatabase.IndexerGetHeadBlock(uuid)
	databaseTailBlock := _AlgoDatabase.IndexerGetTailBlock(uuid)
	if databaseTailBlock < chainEarliestBlock.Uint64() && databaseTailBlock != 0 {
		core.LogDebug("[Algo] Database tail block is too far back - resetting to EarliestBlock")
		_AlgoDatabase.IndexerUpdateTailBlock(uuid, chainEarliestBlock.Uint64())
		databaseTailBlock = chainEarliestBlock.Uint64()
	}
	core.LogDebug("[Algo] --- algoIndexerPreflight(): Fetching posts for " + chainName + " ---")
	core.LogDebug("[Algo] Chain Latest Block: " + chainLatestBlock.String())
	core.LogDebug("[Algo] Database Head Block: " + strconv.Itoa(int(databaseHeadBlock)))
	core.LogDebug("[Algo] Database Tail Block: " + strconv.Itoa(int(databaseTailBlock)))
	core.LogDebug("[Algo] Chain Earliest Block: " + chainEarliestBlock.String())
	return databaseStatus, uuid, chainLatestBlock, databaseTailBlock, databaseHeadBlock, &chainEarliestBlock
}
func algoDispatchTransaction(transaction models.Transaction, blockIndex *big.Int) int {
	// ret 0 == success == transaction was a YP txn and was processed
	// ret 1 == skipped == transaction was not a YP txn
	txID := transaction.Id
	fromAddr := transaction.Sender
	if transaction.PaymentTransaction.Receiver == "" {
		return 1
	}
	toAddr := transaction.PaymentTransaction.Receiver
	noteBytes := transaction.Note
	if len(noteBytes) == 0 {
		return 1
	}
	// Decode base64 note if needed
	noteStr := string(noteBytes)
	timestamp := uint64(transaction.RoundTime)
	amount := transaction.PaymentTransaction.Amount
	if !strings.HasPrefix(noteStr, services.YpPrefix) {
		return 1
	}
	core.LogDebug("[Algo] YourPlace transaction found: " + txID)
	algoTokenizeYourPlaceTransaction("algorand", txID, fromAddr, toAddr, noteStr, amount, timestamp, blockIndex.Uint64())
	return 0
}
func algoTokenizeYourPlaceTransaction(blockchain string, txID string, fromAddress string, toAddress string, noteStr string, amount uint64, timestamp uint64, blockNumber uint64) {
	isValid, versionNumber, actionCode, payloadObject := isValidYourPlacePayload(noteStr)
	if !isValid {
		core.LogDebug("[Algo] Could not decode YourPlace transaction: " + txID)
		return
	}
	txHash := strings.ToLower(txID)
	fromAddr := strings.ToLower(fromAddress)
	_ = strings.ToLower(toAddress)
	parentTxHash := ""
	actionPrefix := actionCode[0]
	actionPostfix := actionCode[1:]
	if versionNumber == 1 {
		switch actionPrefix {
		case 'p': // Post Actions
			switch actionPostfix {
			case "":
				postText, ok := payloadObject["p"]
				if !ok {
					core.LogDebug("[Algo] Post Action: no p in payload")
					return
				}
				postTextStr, ok := postText.(string)
				if !ok {
					return
				}
				postTextStr = security.SanitizeNonPrintable(postTextStr)
				_AlgoDatabase.OnchainP(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, postTextStr)
			case "a":
				postText, ok1 := payloadObject["p"]
				attachmentsRaw, ok2 := payloadObject["a"]
				if !ok1 || !ok2 {
					return
				}
				postTextStr, ok3 := postText.(string)
				attachmentsArray, ok4 := attachmentsRaw.([]interface{})
				if !ok3 || !ok4 {
					return
				}
				parsedAttachments := []db.Attachment{}
				for _, attachment := range attachmentsArray {
					attachmentArray, ok := attachment.([]interface{})
					if !ok || len(attachmentArray) != 4 {
						return
					}
					parsedURL, okURL := attachmentArray[0].(string)
					parsedMimeType, okMimeType := attachmentArray[1].(string)
					sizeFloat, okSize := attachmentArray[2].(float64)
					fileName, okFileName := attachmentArray[3].(string)
					if !okURL || !okMimeType || !okSize || !okFileName {
						return
					}
					if !security.IsValidIndexedFilename(fileName) {
						return
					}
					if !security.IsValidURL(parsedURL) && !security.IsValidCID(parsedURL) {
						return
					}
					if sizeFloat < 0 {
						return
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
				_AlgoDatabase.OnchainPA(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, postTextStr, parsedAttachments)
			}
		case 'f': // Follow Actions
			switch actionPostfix {
			case "":
				blockchainPayload, ok1 := payloadObject["b"]
				addressPayload, ok2 := payloadObject["a"]
				if !ok1 || !ok2 {
					return
				}
				blockchainStr, ok1 := blockchainPayload.(string)
				addressStr, ok2 := addressPayload.(string)
				if !ok1 || !ok2 {
					return
				}
				if !security.IsValidBlockchain(blockchainStr) {
					return
				}
				if !security.IsValidAddress(addressStr, blockchainStr) {
					return
				}
				if fromAddr == addressStr && blockchain == blockchainStr {
					return
				}
				_AlgoDatabase.OnchainF(txHash, blockchain, fromAddr, blockchain, addressStr, blockchainStr, timestamp)
			case "u":
				blockchainPayload, ok1 := payloadObject["b"]
				addressPayload, ok2 := payloadObject["a"]
				if !ok1 || !ok2 {
					return
				}
				blockchainStr, ok1 := blockchainPayload.(string)
				addressStr, ok2 := addressPayload.(string)
				if !ok1 || !ok2 {
					return
				}
				if !security.IsValidBlockchain(blockchainStr) {
					return
				}
				if !security.IsValidAddress(addressStr, blockchainStr) {
					return
				}
				if fromAddr == addressStr && blockchain == blockchainStr {
					return
				}
				_AlgoDatabase.OnchainFU(txHash, blockchain, fromAddr, blockchain, addressStr, blockchainStr, timestamp)
			}
		case 'm': // Metadata Actions
			switch actionPostfix {
			case "n":
				name, ok1 := payloadObject["n"]
				if !ok1 {
					return
				}
				nameStr, ok2 := name.(string)
				if !ok2 {
					return
				}
				nameStr = security.SanitizeNonPrintable(nameStr)
				_AlgoDatabase.OnchainMN(blockchain, fromAddr, nameStr, timestamp)
			case "a":
				avatar, ok1 := payloadObject["a"]
				if !ok1 {
					return
				}
				avatarStr, ok2 := avatar.(string)
				if !ok2 {
					return
				}
				avatarStr = security.SanitizeNonPrintable(avatarStr)
				if security.IsValidURL(avatarStr) || security.IsValidCID(avatarStr) {
					_AlgoDatabase.OnchainMA(blockchain, fromAddr, avatarStr, timestamp)
				}
			case "b":
				banner, ok1 := payloadObject["b"]
				if !ok1 {
					return
				}
				bannerStr, ok2 := banner.(string)
				if !ok2 {
					return
				}
				bannerStr = security.SanitizeNonPrintable(bannerStr)
				if security.IsValidURL(bannerStr) || security.IsValidCID(bannerStr) {
					_AlgoDatabase.OnchainMB(blockchain, fromAddr, bannerStr, timestamp)
				}
			case "v":
				vertical, ok1 := payloadObject["v"]
				if !ok1 {
					return
				}
				verticalStr, ok2 := vertical.(string)
				if !ok2 {
					return
				}
				if security.IsValidVertical(verticalStr) {
					_AlgoDatabase.OnchainMV(blockchain, fromAddr, verticalStr, timestamp)
				}
			case "l":
				location, ok1 := payloadObject["l"]
				if !ok1 {
					return
				}
				locationStr, ok2 := location.(string)
				if !ok2 {
					return
				}
				locationStr = security.SanitizeNonPrintable(locationStr)
				_AlgoDatabase.OnchainML(blockchain, fromAddr, locationStr, timestamp)
			case "w":
				website, ok1 := payloadObject["w"]
				if !ok1 {
					return
				}
				websiteStr, ok2 := website.(string)
				if !ok2 {
					return
				}
				websiteStr = security.SanitizeNonPrintable(websiteStr)
				if security.IsValidURL(websiteStr) && len(websiteStr) > 0 {
					_AlgoDatabase.OnchainMW(blockchain, fromAddr, websiteStr, timestamp)
				}
			case "d":
				description, ok1 := payloadObject["d"]
				if !ok1 {
					return
				}
				descriptionStr, ok2 := description.(string)
				if !ok2 {
					return
				}
				descriptionStr = security.SanitizeNonPrintable(descriptionStr)
				if len(descriptionStr) > 0 {
					_AlgoDatabase.OnchainMD(blockchain, fromAddr, descriptionStr, timestamp)
				}
			}
		case 'c': // Comment Actions
			switch actionPostfix {
			case "":
				parentTxHashRaw, ok1 := payloadObject["t"]
				commentText, ok2 := payloadObject["p"]
				if !ok1 || !ok2 {
					return
				}
				parentTxHashStr, ok3 := parentTxHashRaw.(string)
				commentTextStr, ok4 := commentText.(string)
				if !ok3 || !ok4 {
					return
				}
				if !security.IsValidAlgoTransaction(parentTxHashStr) {
					return
				}
				commentTextStr = security.SanitizeNonPrintable(commentTextStr)
				_AlgoDatabase.OnchainC(txHash, blockchain, fromAddr, parentTxHashStr, amount, timestamp, commentTextStr)
			case "a":
				parentTxHashRaw, ok1 := payloadObject["t"]
				commentText, ok2 := payloadObject["p"]
				attachmentsRaw, ok3 := payloadObject["a"]
				if !ok1 || !ok2 || !ok3 {
					return
				}
				parentTxHashStr, ok4 := parentTxHashRaw.(string)
				commentTextStr, ok5 := commentText.(string)
				attachmentsArray, ok6 := attachmentsRaw.([]interface{})
				if !ok4 || !ok5 || !ok6 {
					return
				}
				if !security.IsValidAlgoTransaction(parentTxHashStr) {
					return
				}
				parsedAttachments := []db.Attachment{}
				for _, attachment := range attachmentsArray {
					attachmentArray, ok := attachment.([]interface{})
					if !ok || len(attachmentArray) != 4 {
						return
					}
					parsedURL, okURL := attachmentArray[0].(string)
					parsedMimeType, okMimeType := attachmentArray[1].(string)
					sizeFloat, okSize := attachmentArray[2].(float64)
					fileName, okFileName := attachmentArray[3].(string)
					if !okURL || !okMimeType || !okSize || !okFileName {
						return
					}
					if !security.IsValidIndexedFilename(fileName) {
						return
					}
					if !security.IsValidURL(parsedURL) && !security.IsValidCID(parsedURL) {
						return
					}
					if sizeFloat < 0 {
						return
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
				commentTextStr = security.SanitizeNonPrintable(commentTextStr)
				_AlgoDatabase.OnchainCA(txHash, blockchain, fromAddr, parentTxHashStr, amount, timestamp, commentTextStr, parsedAttachments)
			}
		case 'r': // Reaction Actions
			switch actionPostfix {
			case "l": // Like
				targetTxHashRaw, ok1 := payloadObject["t"]
				targetTypeRaw, ok2 := payloadObject["t"]
				if !ok1 {
					return
				}
				targetTxHashStr, ok3 := targetTxHashRaw.(string)
				if !ok3 {
					return
				}
				if !security.IsValidAlgoTransaction(targetTxHashStr) {
					return
				}
				targetType := "post"
				if ok2 {
					if tt, ok := targetTypeRaw.(string); ok && (tt == "post" || tt == "comment") {
						targetType = tt
					}
				}
				_AlgoDatabase.OnchainR(txHash, blockchain, fromAddr, targetTxHashStr, targetType, "like", timestamp)
			case "dl": // Dislike
				targetTxHashRaw, ok1 := payloadObject["t"]
				targetTypeRaw, ok2 := payloadObject["t"]
				if !ok1 {
					return
				}
				targetTxHashStr, ok3 := targetTxHashRaw.(string)
				if !ok3 {
					return
				}
				if !security.IsValidAlgoTransaction(targetTxHashStr) {
					return
				}
				targetType := "post"
				if ok2 {
					if tt, ok := targetTypeRaw.(string); ok && (tt == "post" || tt == "comment") {
						targetType = tt
					}
				}
				_AlgoDatabase.OnchainR(txHash, blockchain, fromAddr, targetTxHashStr, targetType, "dislike", timestamp)
			case "e": // Emoji reaction
				targetTxHashRaw, ok1 := payloadObject["t"]
				targetTypeRaw, ok2 := payloadObject["t"]
				emojiRaw, ok3 := payloadObject["e"]
				if !ok1 || !ok3 {
					return
				}
				targetTxHashStr, ok4 := targetTxHashRaw.(string)
				emojiStr, ok5 := emojiRaw.(string)
				if !ok4 || !ok5 {
					return
				}
				if !security.IsValidAlgoTransaction(targetTxHashStr) {
					return
				}
				if len(emojiStr) == 0 || len(emojiStr) > 32 {
					return
				}
				targetType := "post"
				if ok2 {
					if tt, ok := targetTypeRaw.(string); ok && (tt == "post" || tt == "comment") {
						targetType = tt
					}
				}
				_AlgoDatabase.OnchainR(txHash, blockchain, fromAddr, targetTxHashStr, targetType, emojiStr, timestamp)
			}
		}
	}
}
func algoCreateIndexerJob(blockchain string) string {
	uuid := security.UUID()
	_AlgoDatabase.IndexerCreateJob(uuid, blockchain)
	return uuid
}
func algoIndexerPrintProgress(targetEarliestBlock *big.Int, targetLatestBlock *big.Int, blockIndex *big.Int, batchSize *big.Int, direction string, requestTracker *RequestTracker) {
	core.LogDebug("[Algo] ------------------------")
	core.LogDebug("[Algo] index: " + blockIndex.String() + " - direction: " + direction)
	core.LogDebug("[Algo] target latest: " + targetLatestBlock.String() + " - target earliest: " + targetEarliestBlock.String())
	totalRange := new(big.Int).Sub(targetLatestBlock, targetEarliestBlock)
	var progressMade *big.Int
	if direction == "forward" {
		progressMade = new(big.Int).Sub(blockIndex, targetEarliestBlock)
	} else {
		progressMade = new(big.Int).Sub(targetLatestBlock, blockIndex)
	}
	progressPercent := calculatePercentage(totalRange, progressMade)
	core.LogDebug("[Algo] blocks processed: " + progressMade.String() + " - progress: " + progressPercent + " %")
	progressRemaining := new(big.Int).Sub(totalRange, progressMade)
	batchesRemaining := new(big.Int).Div(progressRemaining, batchSize)
	batchSizeRemainder := new(big.Int).Mod(progressRemaining, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchesRemaining.Add(batchesRemaining, big.NewInt(1))
	}
	core.LogDebug("[Algo] blocks remaining: " + progressRemaining.String() + " - batches remaining: " + batchesRemaining.String())
}
func algoCalculateOptimalBatchSize(throttleValue int) *big.Int {
	throttleBasedLimit := throttleValue - algoThrottleOffset
	if throttleBasedLimit <= 0 {
		throttleBasedLimit = 1
	}
	effectiveBatchSize := throttleBasedLimit
	if effectiveBatchSize > algoBatchSizeLimit {
		effectiveBatchSize = algoBatchSizeLimit
	}
	return big.NewInt(int64(effectiveBatchSize))
}
func algoConfigureRateLimiter(throttleValue int) *rate.Limiter {
	requestsPerSecond := algoCalculateDynamicRate(throttleValue, 1)
	burstCapacity := throttleValue
	return rate.NewLimiter(rate.Limit(requestsPerSecond), burstCapacity)
}
func algoStartThrottleController(uuid string, targetThrottleValue int, rateLimiter *rate.Limiter, database *db.Database) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	const (
		adjustmentStep = 0.1
		maxMultiplier  = 100.0
		minMultiplier  = 0.1
		tolerance      = 0.1
	)
	for {
		select {
		case <-ticker.C:
			if algoBreakPoint(uuid) {
				return
			}
			if algoGlobalRequestTracker == nil {
				continue
			}
			actualRPS := algoGlobalRequestTracker.GetRequestsPerSecond()
			targetRPS := float64(targetThrottleValue)
			if actualRPS < 0.1 {
				continue
			}
			database.IndexerUpdateJobSpeed(uuid, uint64(actualRPS+0.5))
			ratio := actualRPS / targetRPS
			if ratio < (1.0-tolerance) || ratio > (1.0+tolerance) {
				algoThrottleControlMutex.Lock()
				if ratio < (1.0 - tolerance) {
					algoDynamicThrottleMultiplier += adjustmentStep
					if algoDynamicThrottleMultiplier > maxMultiplier {
						algoDynamicThrottleMultiplier = maxMultiplier
					}
				} else if ratio > (1.0 + tolerance) {
					algoDynamicThrottleMultiplier -= adjustmentStep * 2
					if algoDynamicThrottleMultiplier < minMultiplier {
						algoDynamicThrottleMultiplier = minMultiplier
					}
				}
				//core.LogDebug("[Algo] Throttle adjustment:\tactual=" + strconv.FormatFloat(actualRPS, 'f', 2, 64) +
				//	"\ttarget=" + strconv.FormatFloat(targetRPS, 'f', 2, 64) +
				//	"\tmultiplier=" + strconv.FormatFloat(algoDynamicThrottleMultiplier, 'f', 3, 64))
				algoThrottleControlMutex.Unlock()
				newRate := algoCalculateDynamicRate(targetThrottleValue, 1)
				rateLimiter.SetLimit(rate.Limit(newRate))
			}
		default:
			if algoBreakPoint(uuid) {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
func algoCalculateDynamicRate(throttleValue int, batchSize int) float64 {
	algoThrottleControlMutex.RLock()
	defer algoThrottleControlMutex.RUnlock()
	batchesPerSecond := (float64(throttleValue) / float64(batchSize)) * algoDynamicThrottleMultiplier
	if batchesPerSecond < 1.0 {
		batchesPerSecond = 1.0
	}
	return batchesPerSecond
}
func algoGetBlockTransactions(uuid string, algo *Algorand, blockNumber uint64) ([]models.Transaction, error) {
	rpcErrorCount := 0
RETRYBLOCK:
	if algoBreakPoint(uuid) {
		return nil, nil
	}
	block, err := algo.algodClient.Block(blockNumber).Do(context.Background())
	if err != nil {
		rpcErrorCount++
		backoff := (rpcErrorCount + 1) * 2
		time.Sleep(time.Duration(backoff) * time.Second)
		if rpcErrorCount >= 60 {
			return nil, core.LogErrorReturn("[Algo] Block fetch failed too many times: " + err.Error())
		}
		goto RETRYBLOCK
	}
	var transactions []models.Transaction
	for _, stx := range block.Payset {
		txn := models.Transaction{
			Id:     crypto.TransactionIDString(stx.Txn),
			Sender: stx.Txn.Sender.String(),
			PaymentTransaction: models.TransactionPayment{
				Receiver: stx.Txn.Receiver.String(),
				Amount:   uint64(stx.Txn.Amount),
			},
			Note:      stx.Txn.Note,
			RoundTime: uint64(block.TimeStamp),
		}
		transactions = append(transactions, txn)
	}
	return transactions, nil
}
func algoWorkerThread(uuid string, rateLimiter *rate.Limiter, algo *Algorand, batchJobQueue *core.ThreadSafeQueue, sequentialTracker *AlgoSequentialBlockTracker, requestTracker *RequestTracker, txnCount *core.ThreadSafeCounter, targetEarliestBlock *big.Int, targetLatestBlock *big.Int, batchSize *big.Int, direction string) error {
	for {
		batch, populated := batchJobQueue.Dequeue()
		if !populated {
			return nil
		}
		batchArray := batch.([]big.Int)
		_batchSize := len(batchArray)
		algoRateLimiterMutex.Lock()
		for i := 0; i < _batchSize; i++ {
			err := rateLimiter.Wait(context.Background())
			if err != nil {
				algoRateLimiterMutex.Unlock()
				return core.LogErrorReturn("[Algo] Rate limiter wait failed: " + err.Error())
			}
		}
		algoRateLimiterMutex.Unlock()
		if algoBreakPoint(sequentialTracker.uuid) {
			return nil
		}
		atomic.AddInt64(&algoActiveRequestsCount, int64(_batchSize))
		atomic.AddInt64(&algoTotalRequestsCount, int64(_batchSize))
		var failedBlocks []big.Int
		for _, blockNumber := range batchArray {
			currentBlockNumber := blockNumber.Int64()
			transactions, err := algoGetBlockTransactions(uuid, algo, uint64(currentBlockNumber))
			if err != nil {
				failedBlocks = append(failedBlocks, blockNumber)
				continue
			}
			if transactions == nil {
				sequentialTracker.MarkBlockProcessed(currentBlockNumber, direction)
				continue
			}
			for _, txn := range transactions {
				ret := algoDispatchTransaction(txn, &blockNumber)
				if ret == 0 {
					txnCount.Increment()
				}
			}
			sequentialTracker.MarkBlockProcessed(currentBlockNumber, direction)
			nextExpected := sequentialTracker.GetNextExpectedBlock()
			mod := nextExpected % algoReportInterval
			if mod == 0 {
				algoProgressLogMutex.Lock()
				if atomic.LoadInt64(&algoLastProgressBlock) != nextExpected {
					atomic.StoreInt64(&algoLastProgressBlock, nextExpected)
					algoIndexerPrintProgress(targetEarliestBlock, targetLatestBlock, big.NewInt(nextExpected), big.NewInt(int64(_batchSize)), direction, requestTracker)
				}
				algoProgressLogMutex.Unlock()
			}
		}
		atomic.AddInt64(&algoActiveRequestsCount, -int64(_batchSize))
		requestTracker.RecordRequests(_batchSize)
		if algoGlobalRequestTracker != nil {
			algoGlobalRequestTracker.RecordRequests(_batchSize)
		}
		if len(failedBlocks) > 0 {
			core.LogDebug("[Algo] Re-queuing " + strconv.Itoa(len(failedBlocks)) + " failed blocks individually")
			backoffTime := len(failedBlocks) * 1
			if backoffTime > 30 {
				backoffTime = 30
			}
			time.Sleep(time.Duration(backoffTime) * time.Second)
			for _, failedBlock := range failedBlocks {
				singleBlockBatch := []big.Int{failedBlock}
				batchJobQueue.Enqueue(singleBlockBatch)
			}
		}
	}
}
func algoBreakPoint(uuid string) bool {
	select {
	case <-algoIndexerCancel:
		core.LogDebug("[Algo] Indexer cancelled in break point")
		_AlgoDatabase.IndexerUpdateJobStatus(uuid, "failed")
		return true
	default:
		return false
	}
}

// --- Algorand Global Helper Functions --- //
func AlgoGetEarliestBlock() big.Int {
	return *AlgoEarliestBlock
}
func AlgoIndexerRestartJobs(__database *db.Database, blockchain string) {
	jobUUID := __database.IndexerGetJobUUID(blockchain)
	__database.IndexerUpdateJobStatus(jobUUID, "failed")
}
func AlgoIndexerStop() {
	AlgoIndexerMutex.Lock()
	defer AlgoIndexerMutex.Unlock()
	if IsAlgoIndexing && algoIndexerCancel != nil {
		select {
		case algoIndexerCancel <- true:
		default:
		}
	}
}
func AlgoToggleIndexer(database *db.Database) {
	indexerRunning := database.SettingsGetValue("algoIndexerRunning")
	if indexerRunning == "true" {
		database.SettingsUpdateValue("algoIndexerRunning", "false")
		AlgoIndexerStop()
	} else {
		database.SettingsUpdateValue("algoIndexerRunning", "true")
	}
}

func AlgoIndexerCatchUpAll(database *db.Database) (bool, string) {
	metaKey := "indexerCatchUpLastRun_algorand"
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
	snapshotURL := "https://yourplace-snapshots.s3.us-east-1.amazonaws.com/algorand-snapshot-complete.db.gz"
	snapshotJsonURL := "https://yourplace-snapshots.s3.us-east-1.amazonaws.com/algorand-snapshot-complete.json"
	database.MetaUpdateValue(metaKey, strconv.FormatUint(core.GetTimestamp(), 10))
	go func() {
		AlgoIndexerStop()
		for i := 0; i < 120; i++ {
			if !IsAlgoIndexing {
				snapshotDir := filepath.Join(host.GetDataDir(), "snapshots")
				host.CreateFolder(snapshotDir)
				snapshotFile := filepath.Join(snapshotDir, "algorand-snapshot-complete.db.gz")
				snapshotMetadataFile := filepath.Join(snapshotDir, "algorand-snapshot-complete.json")
				if host.DoesExist(snapshotFile) {
					core.LogDebug("[Algo] Deleting existing snapshot file: " + snapshotFile)
					host.DeleteIfExists(snapshotFile)
				}
				if host.DoesExist(snapshotMetadataFile) {
					core.LogDebug("[Algo] Deleting existing snapshot metadata file: " + snapshotMetadataFile)
					host.DeleteIfExists(snapshotMetadataFile)
				}
				core.LogInfo("[Algo] Downloading snapshot from: " + snapshotURL)
				err := network.HttpGetFile(snapshotURL, snapshotFile)
				if err != nil {
					core.LogError("[Algo] Could not download snapshot: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				core.LogInfo("[Algo] Downloading snapshot metadata from: " + snapshotJsonURL)
				err = network.HttpGetFile(snapshotJsonURL, snapshotMetadataFile)
				if err != nil {
					core.LogError("[Algo] Could not download snapshot metadata: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				if !host.DoesExist(snapshotFile) {
					core.LogError("[Algo] Snapshot file not found: " + snapshotFile)
					database.MetaUpdateValue(metaKey, "")
					return
				}
				if !host.DoesExist(snapshotMetadataFile) {
					core.LogError("[Algo] Snapshot metadata file not found: " + snapshotMetadataFile)
					database.MetaUpdateValue(metaKey, "")
					return
				}
				core.LogInfo("[Algo] Importing snapshot from: " + snapshotFile)
				err = database.ImportSnapshotNoMetadata(snapshotFile)
				if err != nil {
					core.LogError("[Algo] Could not import snapshot: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				core.LogInfo("[Algo] Reading snapshot metadata from: " + snapshotMetadataFile)
				metadataBytes, err := os.ReadFile(snapshotMetadataFile)
				if err != nil {
					core.LogError("[Algo] Could not read snapshot metadata: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				var metadata map[string]interface{}
				err = json.Unmarshal(metadataBytes, &metadata)
				if err != nil {
					core.LogError("[Algo] Could not parse snapshot metadata: " + err.Error())
					database.MetaUpdateValue(metaKey, "")
					return
				}
				headBlock, headOk := metadata["head_block"].(float64)
				tailBlock, tailOk := metadata["tail_block"].(float64)
				if !headOk || !tailOk {
					core.LogError("[Algo] Snapshot metadata missing head_block or tail_block")
					database.MetaUpdateValue(metaKey, "")
					return
				}
				core.LogInfo(fmt.Sprintf("[Algo] Updating indexer job with head_block: %d, tail_block: %d", uint64(headBlock), uint64(tailBlock)))
				jobUUID := database.IndexerGetJobUUID("algorand")
				if jobUUID == "" {
					core.LogError("[Algo] Could not find indexer job UUID for blockchain: algorand")
					database.MetaUpdateValue(metaKey, "")
					return
				}
				database.IndexerUpdateHeadBlock(jobUUID, uint64(headBlock))
				database.IndexerUpdateTailBlock(jobUUID, uint64(tailBlock))
				host.DeleteAll(snapshotDir)
				core.LogInfo("[Algo] Snapshot import complete")
				return
			}
			time.Sleep(5 * time.Second)
		}
		core.LogError("[Algo] Indexer did not stop in time during snapshot import")
		database.MetaUpdateValue(metaKey, "")
		return
	}()
	return true, "Indexer catch-up started successfully."
}

// Helper to decode base64 note field
func decodeAlgoNote(noteB64 string) string {
	decoded, err := base64.StdEncoding.DecodeString(noteB64)
	if err != nil {
		return ""
	}
	return string(decoded)
}
