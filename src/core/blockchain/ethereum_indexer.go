package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"context"
	"encoding/hex"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/time/rate"
)

var (
	ethereumIndexerCancel             chan bool
	EthereumIndexerMutex              sync.Mutex
	ethereumIsIndexing                bool
	ethereumDynamicThrottleMultiplier = 1.0
	ethereumThrottleControlMutex      sync.RWMutex
	ethereumGlobalRequestTracker      *RequestTracker
	ethereumRateLimiterMutex          sync.Mutex
	ethereumActiveRequestsCount       int64
	ethereumProgressLogMutex          sync.Mutex
	ethereumLastProgressBlock         int64
	ethereumTotalRequestsCount        int64
)

func EthereumIndexerFetchData(database *db.Database, blockchain *Blockchain) bool {
	chainName := "ethereum"
	ethereumBlockchain := blockchain
	ethereumDatabase := database
	databaseStatus, uuid, chainLatestBlock, databaseTailBlock, databaseHeadBlock, chainEarliestBlock := ethereumIndexerPreflight(chainName, ethereumBlockchain, ethereumDatabase)
	if uuid == "" || databaseStatus == "" {
		return false
	}
	_ = databaseHeadBlock
	switch databaseStatus {
	case "pending":
		core.LogDebug("[Ethereum] Starting pending job from the beginning")
		IndexerEthereumFullFill(blockchain.Ethereum, uuid, chainLatestBlock, database)
	case "failed":
		core.LogDebug("[Ethereum] Restarting failed job from where it left off")
		if databaseTailBlock == 0 {
			EthereumIndexerRestartJobs(database, chainName)
			IndexerEthereumFullFill(blockchain.Ethereum, uuid, chainLatestBlock, database)
			return true
		}
		if databaseTailBlock > chainEarliestBlock.Uint64() {
			IndexerEthereumBackFill(blockchain.Ethereum, uuid, chainLatestBlock, database)
		} else {
			IndexerEthereumFrontFill(blockchain.Ethereum, uuid, chainLatestBlock, database)
		}
	case "complete":
		core.LogDebug("[Ethereum] Last job completed successfully. Getting new blocks")
		IndexerEthereumFrontFill(blockchain.Ethereum, uuid, chainLatestBlock, database)
	}
	return true
}

func IndexerEthereumFrontFill(ethereum *Ethereum, uuid string, ethereumLatestBlock *big.Int, database *db.Database) {
	core.LogDebug("[Ethereum] --- IndexerEthereumFrontFill()")
	direction := "forward"
	database.IndexerUpdateJobStatus(uuid, "running")
	headBlock := database.IndexerGetHeadBlock(uuid)
	if headBlock <= 0 {
		core.LogWarn("[Ethereum] IndexerEthereumFrontFill(): Head block is <= 0 - aborting")
		database.IndexerUpdateJobStatus(uuid, "failed")
		return
	}
	targetLatestBlock := ethereumLatestBlock
	targetEarliestBlock := headBlock
	core.LogDebug("[Ethereum] Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("[Ethereum] Target Earliest Block: " + strconv.Itoa(int(targetEarliestBlock)))
	targetEarliestBlockBigInt := big.NewInt(int64(targetEarliestBlock))
	databaseHistoryDaysInt, _ := strconv.Atoi(database.SettingsGetValue("historyDays"))
	ethereumThrottle, _ := strconv.Atoi(database.SettingsGetValue("ethereumThrottle"))
	core.LogDebug("[Ethereum] Throttle: " + strconv.Itoa(ethereumThrottle))
	batchSize := calculateOptimalBatchSize(ethereumThrottle)
	core.LogDebug("[Ethereum] Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, targetEarliestBlockBigInt)
	if blockCount.Int64() <= 0 {
		core.LogDebug("[Ethereum] Block count is negative or zero - likely up to date")
		database.IndexerUpdateJobStatus(uuid, "complete")
		return
	}
	core.LogDebug("[Ethereum] Number of Blocks: " + blockCount.String())
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("[Ethereum] Batch Count: " + batchCount.String())
	rateLimiter := configureRateLimiter(ethereumThrottle)
	batchStartBlock := new(big.Int).Set(targetEarliestBlockBigInt)
	txnCount := core.NewThreadSafeCounter()
	batchJobQueue := core.NewThreadSafeQueue()
	sequentialTracker := NewSequentialBlockTracker(targetEarliestBlockBigInt.Int64(), uuid, database, direction)
	ethereumGlobalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, workerCount)
	atomic.StoreInt64(&ethereumActiveRequestsCount, 0)
	atomic.StoreInt64(&ethereumTotalRequestsCount, 0)
	atomic.StoreInt64(&ethereumLastProgressBlock, 0)

	go ethereumStartThrottleController(uuid, ethereumThrottle, rateLimiter, database)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			baseDelay := time.Duration(workerID) * 500 * time.Millisecond
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			time.Sleep(baseDelay + jitter)
			defer wg.Done()
			err := ethereumWorkerThread(uuid, rateLimiter, ethereum, batchJobQueue, sequentialTracker, ethereumGlobalRequestTracker, txnCount, databaseHistoryDaysInt, targetEarliestBlockBigInt, targetLatestBlock, batchSize, "forward", database)
			if err != nil {
				core.LogError("[Ethereum] Worker thread failed: " + err.Error())
				errorChan <- err
			}
		}(i)
	}
	go func() {
		for i := 1; i <= int(batchCount.Int64()); i++ {
			if ethereumBreakPoint(uuid, database) {
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
		core.LogError("[Ethereum] Worker thread failed: " + err.Error())
		database.IndexerUpdateJobStatus(uuid, "failed")
		return
	case <-done:
		break
	}
	finalBlockIndex := sequentialTracker.GetNextExpectedBlock()
	database.IndexerUpdateHeadBlock(uuid, uint64(finalBlockIndex))
	database.IndexerUpdateJobStatus(uuid, "complete")
}
func IndexerEthereumBackFill(ethereum *Ethereum, uuid string, ethereumLatestBlock *big.Int, database *db.Database) {
	core.LogDebug("[Ethereum] --- IndexerEthereumBackFill() ---")
	direction := "backward"
	database.IndexerUpdateJobStatus(uuid, "running")
	databaseTailBlock := big.NewInt(int64(database.IndexerGetTailBlock(uuid)))
	if databaseTailBlock.Cmp(big.NewInt(0)) == 0 {
		core.LogDebug("[Ethereum] Database Tail Block is 0 - setting to Head Block")
		headBlockInt := database.IndexerGetHeadBlock(uuid)
		databaseTailBlock = big.NewInt(int64(headBlockInt))
		core.LogDebug("[Ethereum] Database Tail Block: " + databaseTailBlock.String())
	}
	targetLatestBlock := databaseTailBlock
	targetEarliestBlock := &ethereum.EarliestBlock
	if targetLatestBlock.Cmp(targetEarliestBlock) == 0 {
		core.LogDebug("[Ethereum] Target latest block is equal to target earliest block - completing")
		database.IndexerUpdateJobStatus(uuid, "complete")
		return
	}
	if targetLatestBlock.Int64() == 0 {
		targetLatestBlock = ethereumLatestBlock
		targetEarliestBlock = &ethereum.EarliestBlock
	}
	core.LogDebug("[Ethereum] Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("[Ethereum] Target Earliest Block: " + targetEarliestBlock.String())
	targetEarliestBlockBigInt := targetEarliestBlock
	databaseHistoryDaysInt, _ := strconv.Atoi(database.SettingsGetValue("historyDays"))
	ethereumThrottle, _ := strconv.Atoi(database.SettingsGetValue("ethereumThrottle"))
	core.LogDebug("[Ethereum] Throttle: " + strconv.Itoa(ethereumThrottle))
	batchSize := calculateOptimalBatchSize(ethereumThrottle)
	core.LogDebug("[Ethereum] Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, targetEarliestBlockBigInt)
	core.LogDebug("[Ethereum] Block Count: " + blockCount.String())
	if blockCount.Int64() <= 0 {
		core.LogWarn("[Ethereum] Backfill block count is negative or zero - marking complete")
		database.IndexerUpdateJobStatus(uuid, "complete")
		return
	}
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("[Ethereum] Batch Count: " + batchCount.String())
	rateLimiter := configureRateLimiter(ethereumThrottle)
	batchStartBlock := targetLatestBlock
	txnCount := core.NewThreadSafeCounter()
	batchJobQueue := core.NewThreadSafeQueue()
	sequentialTracker := NewSequentialBlockTracker(targetLatestBlock.Int64(), uuid, database, direction)
	ethereumGlobalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, workerCount)
	atomic.StoreInt64(&ethereumActiveRequestsCount, 0)
	atomic.StoreInt64(&ethereumTotalRequestsCount, 0)

	go ethereumStartThrottleController(uuid, ethereumThrottle, rateLimiter, database)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			baseDelay := time.Duration(workerID) * 500 * time.Millisecond
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			time.Sleep(baseDelay + jitter)
			defer wg.Done()
			err := ethereumWorkerThread(uuid, rateLimiter, ethereum, batchJobQueue, sequentialTracker, ethereumGlobalRequestTracker, txnCount, databaseHistoryDaysInt, targetEarliestBlock, targetLatestBlock, batchSize, "backward", database)
			if err != nil {
				core.LogError("[Ethereum] Worker encountered an error: " + err.Error())
				errorChan <- err
			}
		}(i)
	}
	go func() {
		for i := 1; i <= int(batchCount.Int64()); i++ {
			if ethereumBreakPoint(uuid, database) {
				return
			}
			batchEndBlock := new(big.Int).Sub(batchStartBlock, batchSize)
			if batchEndBlock.Cmp(targetEarliestBlockBigInt) == -1 {
				batchEndBlock = targetEarliestBlockBigInt
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
		core.LogError("[Ethereum] Worker thread failed: " + err.Error())
		database.IndexerUpdateJobStatus(uuid, "failed")
		return
	case <-done:
		break
	}
	finalBlockIndex := sequentialTracker.GetNextExpectedBlock()
	database.IndexerUpdateTailBlock(uuid, uint64(finalBlockIndex))
	database.IndexerUpdateJobStatus(uuid, "complete")
}
func IndexerEthereumFullFill(ethereum *Ethereum, uuid string, ethereumLatestBlock *big.Int, database *db.Database) {
	core.LogDebug("[Ethereum] --- IndexerEthereumFullFill()")
	direction := "backward"
	database.IndexerUpdateJobStatus(uuid, "running")
	targetLatestBlock := ethereumLatestBlock
	core.LogDebug("[Ethereum] Target Latest Block: " + targetLatestBlock.String())
	targetEarliestBlock := EthereumGetEarliestBlock()
	core.LogDebug("[Ethereum] Target Earliest Block: " + targetEarliestBlock.String())
	targetEarliestBlockBigInt := targetEarliestBlock
	database.IndexerUpdateHeadBlock(uuid, targetLatestBlock.Uint64())
	databaseHistoryDaysInt, _ := strconv.Atoi(database.SettingsGetValue("historyDays"))
	ethereumThrottle, _ := strconv.Atoi(database.SettingsGetValue("ethereumThrottle"))
	core.LogDebug("[Ethereum] Throttle: " + strconv.Itoa(ethereumThrottle))
	batchSize := calculateOptimalBatchSize(ethereumThrottle)
	core.LogDebug("[Ethereum] Batch Size: " + batchSize.String())
	blockCount := new(big.Int).Sub(targetLatestBlock, &targetEarliestBlockBigInt)
	if blockCount.Int64() <= 0 {
		core.LogWarn("[Ethereum] Full fill block count is negative or zero - marking complete")
		database.IndexerUpdateJobStatus(uuid, "complete")
		return
	}
	core.LogDebug("[Ethereum] Number of Blocks: " + blockCount.String())
	batchCount := new(big.Int).Div(blockCount, batchSize)
	batchSizeRemainder := new(big.Int).Mod(blockCount, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchCount.Add(batchCount, big.NewInt(1))
	}
	core.LogDebug("[Ethereum] Batch Count: " + batchCount.String())
	rateLimiter := configureRateLimiter(ethereumThrottle)
	batchStartBlock := targetLatestBlock
	txnCount := core.NewThreadSafeCounter()
	batchJobQueue := core.NewThreadSafeQueue()
	sequentialTracker := NewSequentialBlockTracker(targetLatestBlock.Int64(), uuid, database, direction)
	ethereumGlobalRequestTracker = NewRequestTracker()
	errorChan := make(chan error, workerCount)
	atomic.StoreInt64(&ethereumActiveRequestsCount, 0)
	atomic.StoreInt64(&ethereumTotalRequestsCount, 0)
	atomic.StoreInt64(&ethereumLastProgressBlock, 0)

	go ethereumStartThrottleController(uuid, ethereumThrottle, rateLimiter, database)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			baseDelay := time.Duration(workerID) * 500 * time.Millisecond
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			time.Sleep(baseDelay + jitter)
			defer wg.Done()
			err := ethereumWorkerThread(uuid, rateLimiter, ethereum, batchJobQueue, sequentialTracker, ethereumGlobalRequestTracker, txnCount, databaseHistoryDaysInt, &targetEarliestBlockBigInt, targetLatestBlock, batchSize, "backward", database)
			if err != nil {
				core.LogError("[Ethereum] Worker encountered an error: " + err.Error())
				errorChan <- err
			}
		}(i)
	}
	go func() {
		for i := 1; i <= int(batchCount.Int64()); i++ {
			if ethereumBreakPoint(uuid, database) {
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
		core.LogError("[Ethereum] Worker thread failed: " + err.Error())
		database.IndexerUpdateJobStatus(uuid, "failed")
		return
	case <-done:
		break
	}
	finalBlockIndex := sequentialTracker.GetNextExpectedBlock()
	database.IndexerUpdateTailBlock(uuid, uint64(finalBlockIndex))
	database.IndexerUpdateJobStatus(uuid, "complete")
}

func ethereumIndexerPreflight(chainName string, _blockchain *Blockchain, _database *db.Database) (string, string, *big.Int, uint64, uint64, *big.Int) {
	ethereumIndexerCancel = make(chan bool, 1)
	EthereumIndexerMutex.Lock()
	if ethereumIsIndexing {
		EthereumIndexerMutex.Unlock()
		return "", "", nil, 0, 0, nil
	}
	ethereumIsIndexing = true
	EthereumIndexerMutex.Unlock()
	defer func() {
		EthereumIndexerMutex.Lock()
		ethereumIsIndexing = false
		EthereumIndexerMutex.Unlock()
	}()
	indexerRunning := _database.SettingsGetValue("indexerRunning")
	ethereumIndexerRunning := _database.SettingsGetValue("ethereumIndexerRunning")
	if indexerRunning != "true" || ethereumIndexerRunning == "false" {
		return "", "", nil, 0, 0, nil
	}
	uuid := _database.IndexerGetJobUUID(chainName)
	if uuid == "" {
		uuid = security.UUID()
		_database.IndexerCreateJob(uuid, chainName)
	}
	databaseStatus := _database.IndexerGetJobStatus(uuid)
	if databaseStatus == "running" {
		core.LogWarn("[Ethereum] Indexer job already marked running; skipping cron pass. uuid: " + uuid + " head: " + strconv.Itoa(int(_database.IndexerGetHeadBlock(uuid))) + " tail: " + strconv.Itoa(int(_database.IndexerGetTailBlock(uuid))))
		return "", "", nil, 0, 0, nil
	}
	switch _blockchain.Ethereum.RpcUrl {
	case DefaultBlockchainNodes["ethereum"][0]:
		_database.SettingsUpdateValue("ethereumThrottle", DefaultBlockchainNodes["ethereum"][1])
	}
	chainLatestBlock, err := _blockchain.GetLatestBlock(chainName)
	if err != nil {
		core.LogDebug("[Ethereum] Could not get latest block number: GetLatestBlock returned error: " + err.Error())
		return "", "", nil, 0, 0, nil
	}
	if chainLatestBlock.Cmp(big.NewInt(0)) == 0 {
		core.LogDebug("[Ethereum] Could not get latest block number: " + chainLatestBlock.String())
		return "", "", nil, 0, 0, nil
	}
	chainEarliestBlock := _blockchain.GetEarliestBlock(chainName)
	databaseHeadBlock := _database.IndexerGetHeadBlock(uuid)
	databaseTailBlock := _database.IndexerGetTailBlock(uuid)
	if databaseTailBlock < chainEarliestBlock.Uint64() && databaseTailBlock != 0 {
		core.LogDebug("[Ethereum] Database tail block is too far back - resetting to EarliestBlock")
		cutoffTimestamp := _blockchain.Ethereum.GetBlockTimestamp(chainEarliestBlock)
		if cutoffTimestamp > 0 {
			_database.OnchainDeleteExpired(chainName, cutoffTimestamp)
		}
		_database.IndexerUpdateTailBlock(uuid, chainEarliestBlock.Uint64())
		databaseTailBlock = chainEarliestBlock.Uint64()
	}
	core.LogDebug("[Ethereum] --- EthereumIndexerFetchData(): Fetching posts for " + chainName + " ---")
	core.LogDebug("[Ethereum] Chain Latest Block: " + chainLatestBlock.String())
	core.LogDebug("[Ethereum] Database Head Block: " + strconv.Itoa(int(databaseHeadBlock)))
	core.LogDebug("[Ethereum] Database Tail Block: " + strconv.Itoa(int(databaseTailBlock)))
	core.LogDebug("[Ethereum] Chain Earliest Block: " + chainEarliestBlock.String())
	return databaseStatus, uuid, chainLatestBlock, databaseTailBlock, databaseHeadBlock, chainEarliestBlock
}
func ethereumDispatchTransaction(block map[string]interface{}, transaction map[string]interface{}, databaseHistoryDaysInt *int, blockIndex *big.Int, blockchain string, ethereum *Ethereum) int {
	txHash := transaction["hash"].(string)
	if transaction["to"] == nil {
		return 1
	}
	toAddr := transaction["to"].(string)
	if transaction["input"] == nil {
		return 1
	}
	inputHex := transaction["input"].(string)
	timestampHexStr := block["timestamp"].(string)[2:]
	timestamp, _ := strconv.ParseUint(timestampHexStr, 16, 64)
	if toAddr == entryPointV06Address {
		payloads := extractSmartWalletPayloads(inputHex)
		if len(payloads) > 0 {
			for _, p := range payloads {
				core.LogDebug("[Ethereum] Smart wallet YourPlace transaction found: " + txHash + " from " + p.fromAddress + " to " + p.toAddress)
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
	if strings.HasPrefix(decodedDataStr, services.YpPrefix) {
		core.LogDebug("[Ethereum] YourPlace transaction found: " + txHash)
		tokenizeYourPlaceTransaction(blockchain, transaction, timestamp, blockIndex.Uint64())
		return 0
	} else {
		return 1
	}
}
func ethereumIndexerPrintProgress(targetEarliestBlock *big.Int, targetLatestBlock *big.Int, blockIndex *big.Int, batchSize *big.Int, direction string, requestTracker *RequestTracker) {
	core.LogDebug("[Ethereum] ------------------------")
	core.LogDebug("[Ethereum] index: " + blockIndex.String() + " - direction: " + direction)
	core.LogDebug("[Ethereum] target latest: " + targetLatestBlock.String() + " - target earliest: " + targetEarliestBlock.String())
	totalRange := new(big.Int).Sub(targetLatestBlock, targetEarliestBlock)
	var progressMade *big.Int
	if direction == "forward" {
		progressMade = new(big.Int).Sub(blockIndex, targetEarliestBlock)
	} else {
		progressMade = new(big.Int).Sub(targetLatestBlock, blockIndex)
	}
	progressPercent := calculatePercentage(totalRange, progressMade)
	core.LogDebug("[Ethereum] blocks processed: " + progressMade.String() + " - progress: " + progressPercent + " %")
	progressRemaining := new(big.Int).Sub(totalRange, progressMade)
	batchesRemaining := new(big.Int).Div(progressRemaining, batchSize)
	batchSizeRemainder := new(big.Int).Mod(progressRemaining, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchesRemaining.Add(batchesRemaining, big.NewInt(1))
	}
	core.LogDebug("[Ethereum] blocks remaining: " + progressRemaining.String() + " - batches remaining: " + batchesRemaining.String())
}
func ethereumStartThrottleController(uuid string, targetThrottleValue int, rateLimiter *rate.Limiter, database *db.Database) {
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
			if ethereumBreakPoint(uuid, database) {
				return
			}
			if ethereumGlobalRequestTracker == nil {
				continue
			}
			actualRPS := ethereumGlobalRequestTracker.GetRequestsPerSecond()
			targetRPS := float64(targetThrottleValue)
			if actualRPS < 0.1 {
				continue
			}
			database.IndexerUpdateJobSpeed(uuid, uint64(actualRPS+0.5))
			ratio := actualRPS / targetRPS
			if ratio < (1.0-tolerance) || ratio > (1.0+tolerance) {
				ethereumThrottleControlMutex.Lock()
				if ratio < (1.0 - tolerance) {
					ethereumDynamicThrottleMultiplier += adjustmentStep
					if ethereumDynamicThrottleMultiplier > maxMultiplier {
						ethereumDynamicThrottleMultiplier = maxMultiplier
					}
				} else if ratio > (1.0 + tolerance) {
					ethereumDynamicThrottleMultiplier -= adjustmentStep * 2
					if ethereumDynamicThrottleMultiplier < minMultiplier {
						ethereumDynamicThrottleMultiplier = minMultiplier
					}
				}
				ethereumThrottleControlMutex.Unlock()
				newRate := ethereumCalculateDynamicRate(targetThrottleValue, 1)
				rateLimiter.SetLimit(rate.Limit(newRate))
			}
		default:
			if ethereumBreakPoint(uuid, database) {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
func ethereumCalculateDynamicRate(throttleValue int, batchSize int) float64 {
	ethereumThrottleControlMutex.RLock()
	defer ethereumThrottleControlMutex.RUnlock()
	batchesPerSecond := (float64(throttleValue) / float64(batchSize)) * ethereumDynamicThrottleMultiplier
	if batchesPerSecond < 1.0 {
		batchesPerSecond = 1.0
	}
	return batchesPerSecond
}
func ethereumRpcBatchGetBlockByNumber(uuid string, ethereum *Ethereum, batchBlockNumbers []big.Int, database *db.Database) []map[string]interface{} {
	batchSize := len(batchBlockNumbers)
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
	rpcErrorCount := 0
	rateLimitErrorCount := 0
BATCHRPCCALL:
	var blocks []map[string]interface{}
	if ethereumBreakPoint(uuid, database) {
		return nil
	}
	err := ethereum.RpcClient.BatchCallContext(context.Background(), batch)
	if err != nil {
		core.LogDebug("[Ethereum] Could not perform RPC call from ethereumRpcBatchGetBlockByNumber, backing off")
		if strings.Contains(strings.ToLower(err.Error()), "rps limit") || strings.Contains(strings.ToLower(err.Error()), "rate limit") {
			rateLimitErrorCount++
			baseBackoff := rateLimitErrorCount * rateLimitErrorCount
			batchPenalty := batchSize / 5
			backoff := baseBackoff + batchPenalty
			if backoff > 120 {
				backoff = 120
			}
			core.LogDebug("[Ethereum] Rate limit detected for batch of " + strconv.Itoa(batchSize) + " requests, backing off for " + strconv.Itoa(backoff) + " seconds")
			time.Sleep(time.Duration(backoff) * time.Second)
			ethereumThrottleControlMutex.Lock()
			ethereumDynamicThrottleMultiplier *= 0.90
			if ethereumDynamicThrottleMultiplier < 0.1 {
				ethereumDynamicThrottleMultiplier = 0.1
			}
			core.LogDebug("[Ethereum] Reducing throttle multiplier to " + strconv.FormatFloat(ethereumDynamicThrottleMultiplier, 'f', 3, 64))
			ethereumThrottleControlMutex.Unlock()
			if rateLimitErrorCount >= 20 {
				core.LogDebug("[Ethereum] Too many rate limit errors, failing batch")
				database.IndexerUpdateJobStatus(uuid, "failed")
				return nil
			}
		} else {
			rpcErrorCount++
			backoff := (rpcErrorCount + 1) * 2
			time.Sleep(time.Duration(backoff) * time.Second)
			if rpcErrorCount >= 120 {
				core.LogDebug("[Ethereum] Backfill failed too many times: " + err.Error())
				database.IndexerUpdateJobStatus(uuid, "failed")
				return nil
			}
		}
		goto BATCHRPCCALL
	}
	hasRateLimitError := false
	rateLimitCount := 0
	i := 0
	for _, elem := range batch {
		if ethereumBreakPoint(uuid, database) {
			return nil
		}
		if elem.Error != nil {
			errorMsg := elem.Error.Error()
			if strings.Contains(strings.ToLower(errorMsg), "rps limit") || strings.Contains(strings.ToLower(errorMsg), "rate limit") {
				hasRateLimitError = true
				rateLimitCount++
			}
			blocks = append(blocks, nil)
		} else {
			if elem.Result == nil {
				blocks = append(blocks, nil)
			} else {
				resultPtr, ok := elem.Result.(*map[string]interface{})
				if !ok || resultPtr == nil {
					blocks = append(blocks, nil)
				} else {
					blocks = append(blocks, *resultPtr)
				}
			}
		}
		i++
	}
	if hasRateLimitError && rateLimitCount == batchSize {
		ethereumThrottleControlMutex.Lock()
		ethereumDynamicThrottleMultiplier *= 0.90
		if ethereumDynamicThrottleMultiplier < 0.1 {
			ethereumDynamicThrottleMultiplier = 0.1
		}
		core.LogDebug("[Ethereum] All " + strconv.Itoa(batchSize) + " requests in a batch were rate limited, reducing multiplier to " + strconv.FormatFloat(ethereumDynamicThrottleMultiplier, 'f', 3, 64))
		ethereumThrottleControlMutex.Unlock()
	} else if hasRateLimitError {
		core.LogDebug("[Ethereum] Partial rate limiting in batch: " + strconv.Itoa(rateLimitCount) + "/" + strconv.Itoa(batchSize) + " requests throttled, not adjusting global throttle")
	}
	return blocks
}
func ethereumWorkerThread(uuid string, rateLimiter *rate.Limiter, ethereum *Ethereum, batchJobQueue *core.ThreadSafeQueue, sequentialTracker *SequentialBlockTracker, requestTracker *RequestTracker, txnCount *core.ThreadSafeCounter, databaseHistoryDaysInt int, targetEarliestBlock *big.Int, targetLatestBlock *big.Int, batchSize *big.Int, direction string, database *db.Database) error {
	for {
		batch, populated := batchJobQueue.Dequeue()
		if !populated {
			return nil
		}
		batchArray := batch.([]big.Int)
		_batchSize := len(batchArray)
		ethereumRateLimiterMutex.Lock()
		for i := 0; i < _batchSize; i++ {
			err := rateLimiter.Wait(context.Background())
			if err != nil {
				ethereumRateLimiterMutex.Unlock()
				return core.LogErrorReturn("[Ethereum] Rate limiter wait failed: " + err.Error())
			}
		}
		ethereumRateLimiterMutex.Unlock()
		if ethereumBreakPoint(sequentialTracker.uuid, database) {
			return nil
		}
		atomic.AddInt64(&ethereumActiveRequestsCount, int64(_batchSize))
		atomic.AddInt64(&ethereumTotalRequestsCount, int64(_batchSize))
		blocks := ethereumRpcBatchGetBlockByNumber(uuid, ethereum, batchArray, database)
		atomic.AddInt64(&ethereumActiveRequestsCount, -int64(_batchSize))
		requestTracker.RecordRequests(_batchSize)
		if ethereumGlobalRequestTracker != nil {
			ethereumGlobalRequestTracker.RecordRequests(_batchSize)
		}
		if blocks == nil {
			continue
		}
		var failedBlocks []big.Int
		for i, block := range blocks {
			if i >= len(batchArray) {
				break
			}
			if block == nil {
				failedBlocks = append(failedBlocks, batchArray[i])
				continue
			}
			currentBlockNumber := batchArray[i].Int64()
			transactionsRaw, exists := block["transactions"]
			if !exists || transactionsRaw == nil {
				sequentialTracker.MarkBlockProcessed(currentBlockNumber, direction)
				continue
			}
			transactions, ok := transactionsRaw.([]interface{})
			if !ok {
				sequentialTracker.MarkBlockProcessed(currentBlockNumber, direction)
				continue
			}
			for _, txn := range transactions {
				if txn == nil {
					continue
				}
				transaction, ok2 := txn.(map[string]interface{})
				if !ok2 {
					continue
				}
				ret := ethereumDispatchTransaction(block, transaction, &databaseHistoryDaysInt, big.NewInt(currentBlockNumber), "ethereum", ethereum)
				if ret == 1 || ret == 2 {
					continue
				}
				txnCount.Increment()
			}
			sequentialTracker.MarkBlockProcessed(currentBlockNumber, direction)
			nextExpected := sequentialTracker.GetNextExpectedBlock()
			mod := nextExpected % reportInterval
			if mod == 0 {
				ethereumProgressLogMutex.Lock()
				if atomic.LoadInt64(&ethereumLastProgressBlock) != nextExpected {
					atomic.StoreInt64(&ethereumLastProgressBlock, nextExpected)
					ethereumIndexerPrintProgress(targetEarliestBlock, targetLatestBlock, big.NewInt(nextExpected), big.NewInt(int64(_batchSize)), direction, requestTracker)
				}
				ethereumProgressLogMutex.Unlock()
			}
		}
		if len(failedBlocks) > 0 {
			core.LogDebug("[Ethereum] Re-queuing " + strconv.Itoa(len(failedBlocks)) + " failed blocks individually")
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
func ethereumBreakPoint(uuid string, database *db.Database) bool {
	select {
	case <-ethereumIndexerCancel:
		core.LogDebug("[Ethereum] Indexer cancelled in break point")
		database.IndexerUpdateJobStatus(uuid, "failed")
		return true
	default:
		return false
	}
}

func EthereumIndexerRestartJobs(__database *db.Database, blockchain string) {
	jobUUID := __database.IndexerGetJobUUID(blockchain)
	__database.IndexerUpdateJobStatus(jobUUID, "failed")
}
func EthereumIndexerStop() {
	EthereumIndexerMutex.Lock()
	defer EthereumIndexerMutex.Unlock()
	if ethereumIsIndexing && ethereumIndexerCancel != nil {
		select {
		case ethereumIndexerCancel <- true:
		default:
		}
	}
}
func ToggleEthereumIndexer(database *db.Database) {
	indexerRunning := database.SettingsGetValue("ethereumIndexerRunning")
	if indexerRunning == "true" || indexerRunning == "" {
		database.SettingsUpdateValue("ethereumIndexerRunning", "false")
		EthereumIndexerStop()
	} else {
		database.SettingsUpdateValue("ethereumIndexerRunning", "true")
	}
}
func EthereumIndexerCatchUpAll(database *db.Database) (bool, string) {
	core.LogDebug("[Ethereum] Ethereum snapshot not available yet")
	return false, "Ethereum snapshot not available yet."
}
