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

const reportInterval = 10000 // print progress every # of blocks
const saveInterval = 1000    // save progress every # of blocks
const throttleOffset = 5     // How many blocks to subtract from the throttle limit to allow for the front-end to make calls without getting rate-limited

var (
	indexerCancel chan bool
	IndexerMutex  sync.Mutex
	IsIndexing    bool
)

// --- Indexer Main Method --- //
func IndexerFetchData(database *db.Database, blockchain *Blockchain, chainName string) {
	// Handle indexer mutex and exit channel
	indexerCancel = make(chan bool, 1)
	IndexerMutex.Lock()
	if IsIndexing {
		IndexerMutex.Unlock()
		return // Already running
	}
	IsIndexing = true
	IndexerMutex.Unlock()
	defer func() { // Cleanup mutex when we're done
		IndexerMutex.Lock()
		IsIndexing = false
		IndexerMutex.Unlock()
	}()

	uuid := database.IndexerGetJobUUID(chainName) // Lookup the UUID of the blockchain job
	if uuid == "" {                               // If no job exists, create one
		uuid = CreateIndexerJob(database, chainName)
	}
	databaseStatus := database.IndexerGetJobStatus(uuid) // Get the status of the job
	// ---- Job Status Dispatch ---- //
	if databaseStatus == "running" { // Only 1 post caching job running at a time
		return // bail out
	}

	core.LogDebug("--- IndexerFetchData(): Fetching posts for " + chainName + " ---")
	switch blockchain.Base.RpcUrl { // Set throttle defaults for known public nodes
	case DefaultBlockchainNodes["base"][0]:
		database.SettingsUpdateValue("baseThrottle", DefaultBlockchainNodes["base"][1]) // default rate limit for default nodes
	}
	baseLatestBlock, err := blockchain.GetLatestBlock("base")  // Get the latest block number from the blockchain RPC node
	if err != nil || baseLatestBlock.Cmp(big.NewInt(0)) == 0 { // Error checking the latest block number we got from the RPC node
		core.LogError("Could not get Base latest block number - Will try again on next indexer run")
		return // bail out
	}
	core.LogDebug("Base Latest Block: " + baseLatestBlock.String())
	baseEarliestBlock := blockchain.GetEarliestBlock("base") // Get the earliest block number that a YourPlace post existed (YourPlace genesis block)
	databaseHeadBlock := database.IndexerGetHeadBlock(uuid)  // Get the head block from the database (latest block processed)
	core.LogDebug("Database Head Block: " + strconv.Itoa(int(databaseHeadBlock)))
	databaseTailBlock := database.IndexerGetTailBlock(uuid)                       // Get the tail block from the database (earliest block processed)
	if databaseTailBlock < baseEarliestBlock.Uint64() && databaseTailBlock != 0 { // Check that the tail block is ahead of the earliest block
		core.LogDebug("Database tail block is too far back - resetting to EarliestBlock")
		database.IndexerUpdateTailBlock(uuid, baseEarliestBlock.Uint64()) // If not, reset the tail block to the earliest block
		databaseTailBlock = baseEarliestBlock.Uint64()
	}
	core.LogDebug("Database Tail Block: " + strconv.Itoa(int(databaseTailBlock)))
	core.LogDebug("Base Earliest Block: " + baseEarliestBlock.String())

	switch databaseStatus { // Post fill job dispatch, based on last job status
	case "pending":
		core.LogDebug("Starting pending job from the beginning")
		IndexerBaseFullFill(database, blockchain.Base, uuid, baseLatestBlock)
	case "failed":
		core.LogDebug("Restarting failed job from where it left off")
		if databaseTailBlock == 0 { // If a full fill job started, but failed before the tail block was written, start all over
			IndexerRestartJobs(database, chainName)
			IndexerBaseFullFill(database, blockchain.Base, uuid, baseLatestBlock)
			return
		}
		if databaseTailBlock > baseEarliestBlock.Uint64() { // if a backfill job failed, restart it
			IndexerBaseBackFill(database, blockchain.Base, uuid, baseLatestBlock)
		} else { // if a front fill job failed, restart it
			IndexerBaseFrontFill(database, blockchain.Base, uuid, baseLatestBlock)
		}
	case "complete": // If everything is backfilled, then just process the newest blocks
		core.LogDebug("Last job completed successfully. Getting new blocks")
		IndexerBaseFrontFill(database, blockchain.Base, uuid, baseLatestBlock)
	}
}

// --- Base Indexer Functions --- //
func IndexerBaseFrontFill(database *db.Database, base *Base, uuid string, baseLatestBlock *big.Int) {
	// head block <----- latest block (starting traversal @ latest block)
	core.LogDebug("--- IndexerBaseFrontFill()")
	database.IndexerUpdateJobStatus(uuid, "running")
	headBlock := database.IndexerGetHeadBlock(uuid)
	if headBlock <= 0 {
		core.LogWarn("IndexerBaseFrontFill(): Head block is <= 0 - aborting")
		return
	}
	targetLatestBlock := baseLatestBlock
	targetEarliestBlock := headBlock
	core.LogDebug("Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("Target Earliest Block: " + strconv.Itoa(int(targetEarliestBlock)))
	targetEarliestBlockBigInt := big.NewInt(int64(targetEarliestBlock))
	databaseHistoryDaysInt, _ := strconv.Atoi(database.SettingsGetValue("historyDays"))
	baseThrottle, _ := strconv.Atoi(database.SettingsGetValue("baseThrottle"))
	batchSize := big.NewInt(int64(baseThrottle - throttleOffset))
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
	rateLimiter := rate.NewLimiter(rate.Limit(1), 1) // 1 request per second (1 request = 1 batch of blocks)
	batchStartBlock := targetLatestBlock             // Start at the latest block
	txnCount := 0                                    // Count the number of transactions processed
	blockIndex := batchStartBlock                    // Running tally of the current block
	for i := 1; i <= int(batchCount.Int64()); i++ {  // Loop over batches of blocks
		select {
		case <-indexerCancel:
			core.LogDebug("Indexer job cancelled in frontfill during batch loop")
			database.IndexerUpdateJobStatus(uuid, "failed")
			return
		default:
			_ = rateLimiter.Wait(context.Background())
			batchEndBlock := new(big.Int).Sub(batchStartBlock, batchSize)
			if batchEndBlock.Cmp(targetEarliestBlockBigInt) == -1 { // stop at the earliest block allowed
				batchEndBlock = targetEarliestBlockBigInt
			}
			if batchStartBlock.Cmp(batchEndBlock) < 0 { // break if the start block is behind the end block
				break
			}
			// Batch up blocks into one RPC call
			var batchBlockNumbers []big.Int
			for j := batchStartBlock; j.Cmp(batchEndBlock) == 1; j = new(big.Int).Sub(j, big.NewInt(1)) {
				batchBlockNumbers = append(batchBlockNumbers, *j)
			}
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
		BATCHRPCCALL:
			select {
			case <-indexerCancel:
				core.LogDebug("Indexer cancelled in frontfill during RPC call")
				database.IndexerUpdateJobStatus(uuid, "failed")
				return
			default:
				err := base.RpcClient.BatchCallContext(context.Background(), batch)
				if err != nil {
					//core.LogError("Could not get block data 1: " + err.Error())
					rpcErrorCount++
					backoff := rpcErrorCount + 1
					time.Sleep(time.Duration(backoff) * time.Second) // exponential backoff
					if rpcErrorCount >= 120 {
						database.IndexerUpdateJobStatus(uuid, "failed")
						return
					}
					goto BATCHRPCCALL
				}
			}
			for _, elem := range batch { // Loop through each block in the batch response
				select {
				case <-indexerCancel:
					core.LogDebug("Indexer cancelled in frontfill during batch processing")
					database.IndexerUpdateJobStatus(uuid, "stopped")
					return
				default:
					if elem.Error != nil {
						//core.LogError("Could not get block data 2: " + elem.Error.Error())
						rpcErrorCount++
						backoff := rpcErrorCount + 1
						time.Sleep(time.Duration(backoff) * time.Second) // exponential backoff
						if rpcErrorCount >= 120 {
							database.IndexerUpdateJobStatus(uuid, "failed")
							return
						}
						goto BATCHRPCCALL
					}
					block := *elem.Result.(*map[string]interface{})
					transactions := block["transactions"].([]interface{})
					for _, txn := range transactions { // Loop through transactions in the block
						transaction := txn.(map[string]interface{})
						ret := DispatchTransaction(database, block, transaction, &databaseHistoryDaysInt, blockIndex)
						if ret == 1 || ret == 2 { // skip transactions that are not valid YP posts
							continue
						}
						txnCount++
					}
					blockIndex = new(big.Int).Sub(blockIndex, big.NewInt(1)) // decrement the block index
					mod := big.NewInt(0)                                     // Send a status update
					mod.Mod(blockIndex, big.NewInt(reportInterval))
					if mod.Sign() == 0 {
						IndexerPrintProgress(big.NewInt(int64(targetEarliestBlock)), targetLatestBlock, blockIndex, batchSize)
					}
					mod.Mod(blockIndex, big.NewInt(saveInterval))
					if mod.Sign() == 0 {
						//database.IndexerUpdateTailBlock(uuid, blockIndex.Uint64()) <-- This was messing up the database logic
						// todo - maybe do nothing?
						//core.LogDebug("Save interval reached - index block: " + blockIndex.String())
					}
				}
			}
			batchStartBlock = new(big.Int).Sub(batchStartBlock, batchSize)
		}
	}
	database.IndexerUpdateHeadBlock(uuid, targetLatestBlock.Uint64())
	database.IndexerUpdateJobStatus(uuid, "complete")
	latestBlockTemp := database.IndexerGetHeadBlock(uuid)
	core.LogDebug("Completed Front Fill - Updating Head Block: " + strconv.Itoa(int(latestBlockTemp)))
}
func IndexerBaseBackFill(database *db.Database, base *Base, uuid string, baseLatestBlock *big.Int) {
	// earliest block <----- tail block (starting traversal @ tail block)—
	core.LogDebug("--- IndexerBaseBackFill()")
	database.IndexerUpdateJobStatus(uuid, "running")
	databaseTailBlock := big.NewInt(int64(database.IndexerGetTailBlock(uuid)))
	if databaseTailBlock.Cmp(big.NewInt(0)) == 0 { // if databaseTailBlock = 0, then the job is new
		core.LogDebug("Database Tail Block is 0 - setting to Head Block")
		headBlockInt := database.IndexerGetHeadBlock(uuid)
		databaseTailBlock = big.NewInt(int64(headBlockInt))
		core.LogDebug("Database Tail Block: " + databaseTailBlock.String())
	}
	targetLatestBlock := databaseTailBlock
	targetEarliestBlock := &base.EarliestBlock
	if targetLatestBlock.Cmp(targetEarliestBlock) == 0 {
		core.LogDebug("Target latest block is equal to target earliest block - completing")
		database.IndexerUpdateJobStatus(uuid, "complete")
		return
	}
	if targetLatestBlock.Int64() == 0 { // if targetLatestBlock = 0, then the job is new
		targetLatestBlock = baseLatestBlock // set the widest possible target for a new backfill
		targetEarliestBlock = &base.EarliestBlock
	}
	core.LogDebug("Target Latest Block: " + targetLatestBlock.String())
	core.LogDebug("Target Earliest Block: " + targetEarliestBlock.String())
	targetEarliestBlockBigInt := targetEarliestBlock
	databaseHistoryDaysInt, _ := strconv.Atoi(database.SettingsGetValue("historyDays"))
	baseThrottle, _ := strconv.Atoi(database.SettingsGetValue("baseThrottle"))
	batchSize := big.NewInt(int64(baseThrottle - throttleOffset))
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
	rateLimiter := rate.NewLimiter(rate.Limit(1), 1) // 1 request per second (1 request = 1 batch of blocks)
	batchStartBlock := targetLatestBlock             // Start at the latest block
	txnCount := 0                                    // Count the number of transactions processed
	blockIndex := batchStartBlock                    // Running tally of the current block

	for i := 1; i <= int(batchCount.Int64()); i++ { // Loop over batches of blocks
		select {
		case <-indexerCancel:
			core.LogDebug("Indexer cancelled in backfill during batch loop")
			database.IndexerUpdateJobStatus(uuid, "failed")
			return
		default:
			_ = rateLimiter.Wait(context.Background())
			batchEndBlock := new(big.Int).Sub(batchStartBlock, batchSize)
			if batchEndBlock.Cmp(targetEarliestBlockBigInt) == -1 { // stop at the earliest block allowed
				batchEndBlock = targetEarliestBlockBigInt
			}
			if batchStartBlock.Cmp(batchEndBlock) < 0 { // break if the start block is behind the end block
				break
			}
			// Batch up blocks into one RPC call
			var batchBlockNumbers []big.Int
			for j := batchStartBlock; j.Cmp(batchEndBlock) == 1; j = new(big.Int).Sub(j, big.NewInt(1)) {
				batchBlockNumbers = append(batchBlockNumbers, *j)
			}
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
		BATCHRPCCALL:
			select {
			case <-indexerCancel:
				core.LogDebug("Indexer cancelled in backfill during RPC call")
				database.IndexerUpdateJobStatus(uuid, "failed")
				return
			default:
				err := base.RpcClient.BatchCallContext(context.Background(), batch)
				if err != nil {
					//core.LogDebug("Could not get block data 1, backing off: " + err.Error())
					rpcErrorCount++
					backoff := rpcErrorCount + 1
					time.Sleep(time.Duration(backoff) * time.Second) // exponential backoff
					if rpcErrorCount >= 120 {
						//core.LogDebug("Base Backfill failed too many times: " + err.Error())
						database.IndexerUpdateJobStatus(uuid, "failed")
						return
					}
					goto BATCHRPCCALL
				}
			}

			for _, elem := range batch { // Loop through each block in the batch response
				select {
				case <-indexerCancel:
					core.LogDebug("Indexer cancelled in backfill during batch processing")
					database.IndexerUpdateJobStatus(uuid, "failed")
					return
				default:
					if elem.Error != nil {
						//core.LogDebug("Could not get block data 2, backing off: " + elem.Error.Error())
						rpcErrorCount++
						backoff := rpcErrorCount + 1
						time.Sleep(time.Duration(backoff) * time.Second) // exponential backoff
						if rpcErrorCount >= 120 {
							database.IndexerUpdateJobStatus(uuid, "failed")
							return
						}
						goto BATCHRPCCALL
					}
					block := *elem.Result.(*map[string]interface{})
					transactions := block["transactions"].([]interface{})
					for _, txn := range transactions { // Loop through transactions in the block
						transaction := txn.(map[string]interface{})
						ret := DispatchTransaction(database, block, transaction, &databaseHistoryDaysInt, blockIndex)
						if ret == 1 || ret == 2 { // skip transactions that are not valid YP posts
							continue
						}
						txnCount++
					}
					blockIndex = new(big.Int).Sub(blockIndex, big.NewInt(1)) // decrement the block index
					mod := big.NewInt(0)                                     // Send a status update
					mod.Mod(blockIndex, big.NewInt(reportInterval))
					if mod.Sign() == 0 {
						IndexerPrintProgress(targetEarliestBlock, targetLatestBlock, blockIndex, batchSize)
					}
					mod.Mod(blockIndex, big.NewInt(saveInterval))
					if mod.Sign() == 0 {
						database.IndexerUpdateTailBlock(uuid, blockIndex.Uint64())
					}
				}
			}
			batchStartBlock = new(big.Int).Sub(batchStartBlock, batchSize)
		}
	}
	core.LogDebug("Completed Back Fill - Updating Tail Block: " + blockIndex.String())
	database.IndexerUpdateTailBlock(uuid, blockIndex.Uint64())
	database.IndexerUpdateJobStatus(uuid, "complete")
}
func IndexerBaseFullFill(database *db.Database, base *Base, uuid string, baseLatestBlock *big.Int) {
	// earliest block <----- latest block (starting traversal @ latest block)
	core.LogDebug("--- IndexerBaseFullFill()")
	database.IndexerUpdateJobStatus(uuid, "running")
	targetLatestBlock := baseLatestBlock
	core.LogDebug("Target Latest Block: " + targetLatestBlock.String())
	targetEarliestBlock := BaseGetEarliestBlock()
	core.LogDebug("Target Earliest Block: " + targetEarliestBlock.String())
	targetEarliestBlockBigInt := targetEarliestBlock
	database.IndexerUpdateHeadBlock(uuid, targetLatestBlock.Uint64())
	databaseHistoryDaysInt, _ := strconv.Atoi(database.SettingsGetValue("historyDays"))
	baseThrottle, _ := strconv.Atoi(database.SettingsGetValue("baseThrottle"))
	batchSize := big.NewInt(int64(baseThrottle - throttleOffset))
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
	rateLimiter := rate.NewLimiter(rate.Limit(1), 1) // 1 request per second (1 request = 1 batch of blocks)
	batchStartBlock := targetLatestBlock             // Start at the latest block
	txnCount := 0                                    // Count the number of transactions processed
	blockIndex := batchStartBlock                    // Running tally of the current block

	for i := 1; i <= int(batchCount.Int64()); i++ { // Loop over batches of blocks
		select {
		case <-indexerCancel:
			core.LogDebug("Indexer cancelled in fullfill during batch loop")
			database.IndexerUpdateJobStatus(uuid, "failed")
			return
		default:
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
		BATCHRPCCALL:
			select {
			case <-indexerCancel:
				core.LogDebug("Indexer cancelled in fullfill during RPC call")
				database.IndexerUpdateJobStatus(uuid, "failed")
				return
			default:
				err := base.RpcClient.BatchCallContext(context.Background(), batch)
				if err != nil {
					//core.LogDebug("Could not get block data 1, backing off: " + err.Error())
					rpcErrorCount++
					backoff := rpcErrorCount + 1
					time.Sleep(time.Duration(backoff) * time.Second) // exponential backoff
					if rpcErrorCount >= 120 {
						database.IndexerUpdateJobStatus(uuid, "failed")
						return
					}
					goto BATCHRPCCALL
				}
			}

			for _, elem := range batch { // Loop through each block in the batch response
				select {
				case <-indexerCancel:
					core.LogDebug("Indexer cancelled in fullfill during batch processing")
					database.IndexerUpdateJobStatus(uuid, "failed")
					return
				default:
					if elem.Error != nil {
						//core.LogDebug("Could not get block data 2, backing off: " + elem.Error.Error())
						rpcErrorCount++
						backoff := rpcErrorCount + 1
						time.Sleep(time.Duration(backoff) * time.Second) // exponential backoff
						if rpcErrorCount >= 120 {
							database.IndexerUpdateJobStatus(uuid, "failed")
							return
						}
						goto BATCHRPCCALL
					}
					block := *elem.Result.(*map[string]interface{})
					transactions := block["transactions"].([]interface{})
					for _, txn := range transactions { // Loop through transactions in the block
						transaction := txn.(map[string]interface{})
						ret := DispatchTransaction(database, block, transaction, &databaseHistoryDaysInt, blockIndex)
						if ret == 1 || ret == 2 { // skip transactions that are not valid YP posts
							continue
						}
						txnCount++
					}
					blockIndex = new(big.Int).Sub(blockIndex, big.NewInt(1)) // decrement the block index
					mod := big.NewInt(0)                                     // Send a status update
					mod.Mod(blockIndex, big.NewInt(reportInterval))
					if mod.Sign() == 0 {
						IndexerPrintProgress(&targetEarliestBlock, targetLatestBlock, blockIndex, batchSize)
					}
					mod.Mod(blockIndex, big.NewInt(saveInterval))
					if mod.Sign() == 0 {
						database.IndexerUpdateTailBlock(uuid, blockIndex.Uint64())
					}
				}
			}
			batchStartBlock = new(big.Int).Sub(batchStartBlock, batchSize)
		}
	}
	core.LogDebug("Completed Full Fill - Updating Tail Block: " + blockIndex.String())
	database.IndexerUpdateTailBlock(uuid, blockIndex.Uint64())
	database.IndexerUpdateJobStatus(uuid, "complete")
}

// --- Helper Functions --- //
func DispatchTransaction(database *db.Database, block map[string]interface{}, transaction map[string]interface{}, databaseHistoryDaysInt *int, blockIndex *big.Int) int {
	// ret 0 == success == transaction was a YP txn and was processed
	// ret 1 == skipped == transaction was not a YP txn
	// ret 2 == expired == transaction is older than the cached history limit
	txHash := strings.ToLower(transaction["hash"].(string))
	fromAddr := strings.ToLower(transaction["from"].(string))
	if transaction["to"] == nil { // Skip transactions with no recipient
		return 1
	}
	toAddr := strings.ToLower(transaction["to"].(string))
	if transaction["input"] == nil { // Skip transactions with no data payload
		return 1
	}
	data := transaction["input"].(string)[2:]
	decodedDataBytes, _ := hex.DecodeString(data)
	decodedDataStr := string(decodedDataBytes)
	amountHexStr := transaction["value"].(string)[2:]
	amountInt, _ := strconv.ParseUint(amountHexStr, 16, 64)
	parentTxHash := "" // todo - figure out comment logic hierarchy
	timestampHexStr := block["timestamp"].(string)[2:]
	timestamp, _ := strconv.ParseUint(timestampHexStr, 16, 64)
	if IsTimestampExpired(int64(*databaseHistoryDaysInt), int64(timestamp)) { // skip transactions older than the cached history limit
		return 2
	}
	if strings.HasPrefix(decodedDataStr, services.YpPrefix) { // Is the txn a YourPlace post
		core.LogDebug("YourPlace Transaction Found: " + txHash)
		database.IndexerAddPost(txHash, "base", fromAddr, toAddr, parentTxHash, amountInt, timestamp, decodedDataStr, blockIndex.Uint64())
		TokenizeYourPlaceTransaction(database, "base", transaction, timestamp, blockIndex.Uint64())
		return 0
	} else {
		return 1
	}
}
func IsTimestampExpired(databaseHistoryDaysInt int64, timestamp int64) bool {
	now := time.Now()
	diff := now.Sub(time.Unix(timestamp, 0))
	return diff > time.Duration(databaseHistoryDaysInt)*24*time.Hour
}
func ClearOldCachedPosts(database *db.Database) {
	// todo
	// Clear cached transactions that are older than configured (expired) cached post history limit
	// Only clear posts from backfill jobs where the address is "*" - do not clear cached posts for a specific address
	//historyDays := database.SettingsGetValue("historyDays")
	// todo - get all posts older than the calculated history days, and delete them
}
func CreateIndexerJob(database *db.Database, blockchain string) string {
	uuid := security.UUID()
	database.IndexerCreateJob(uuid, blockchain)
	return uuid
}
func StartupIndexerCleanup(database *db.Database) {
	// Check for any failed or hung backfill jobs. Only runs once on startup
	runningJobsUUIDs := database.IndexerGetRunningJobsUUIDs()
	if len(runningJobsUUIDs) > 0 { // If any running jobs exist, reset them to 'pending'
		for _, uuid := range runningJobsUUIDs {
			database.IndexerUpdateJobStatus(uuid, "failed")
		}
	}
}
func IndexerRestartJobs(database *db.Database, blockchain string) {
	// set any indexer jobs to "failed" that were left in a "running" state from a crashed server
	jobUUID := database.IndexerGetJobUUID(blockchain)
	database.IndexerUpdateJobStatus(jobUUID, "failed")
}
func IndexerPrintProgress(targetEarliestBlock *big.Int, targetLatestBlock *big.Int, blockIndex *big.Int, batchSize *big.Int) {
	core.LogDebug("------------------------")
	core.LogDebug("index: " + blockIndex.String())
	core.LogDebug("target latest: " + targetLatestBlock.String())
	core.LogDebug("target earliest: " + targetEarliestBlock.String())
	totalRange := new(big.Int).Sub(targetLatestBlock, targetEarliestBlock)
	core.LogDebug("total range: " + totalRange.String())
	indexOffset := new(big.Int).Sub(blockIndex, targetEarliestBlock)
	//core.LogDebug("index offset: " + indexOffset.String())
	progressMade := new(big.Int).Sub(totalRange, indexOffset)
	core.LogDebug("progress made: " + progressMade.String())
	progressPercent := CalculatePercentage(totalRange, progressMade)
	core.LogDebug("progress: " + progressPercent + " %")
	progressRemaining := new(big.Int).Sub(totalRange, progressMade)
	core.LogDebug("progress remaining: " + progressRemaining.String())
	batchesRemaining := new(big.Int).Div(progressRemaining, batchSize)
	batchSizeRemainder := new(big.Int).Mod(progressRemaining, batchSize)
	if batchSizeRemainder.Cmp(big.NewInt(0)) != 0 {
		batchesRemaining.Add(batchesRemaining, big.NewInt(1))
	}
	core.LogDebug("batches remaining: " + batchesRemaining.String())
}
func CalculatePercentage(totalRange *big.Int, index *big.Int) string {
	if totalRange.Sign() == 0 {
		return big.NewInt(0).String()
	}
	percentage := new(big.Int)
	hundred := big.NewInt(100)
	percentage.Mul(index, hundred)
	percentage.Div(percentage, totalRange)
	return percentage.String()
}
func TokenizeYourPlaceTransaction(database *db.Database, blockchain string, transaction map[string]interface{}, timestamp uint64, blockNumber uint64) {
	// Pattern-based tokenization and database storage of YourPlace transactions
	var protocolRegex = regexp.MustCompile(`^yp/([\d.]+)/([a-z]+):(.+)$`)
	data := transaction["input"].(string)[2:]       // get data from the transaction
	decodedDataBytes, err := hex.DecodeString(data) // hex decode data
	if err != nil {
		core.LogError("Could not decode YourPlace transaction: " + err.Error())
		return
	}
	decodedDataStr := string(decodedDataBytes)                  // convert bytes to string
	matches := protocolRegex.FindStringSubmatch(decodedDataStr) // match the string to the protocol regex
	if matches == nil {
		core.LogError("Could not tokenize YourPlace transaction: " + decodedDataStr)
		return
	}
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
	txHash := strings.ToLower(transaction["hash"].(string))
	fromAddress := strings.ToLower(transaction["from"].(string))
	toAddress := strings.ToLower(transaction["to"].(string))
	parentTxHash := ""
	if payloadObject["parentTxHash"] != nil {
		parentTxHash = payloadObject["parentTxHash"].(string)
	}
	amountHexStr := transaction["value"].(string)[2:]
	amountInt, _ := strconv.ParseUint(amountHexStr, 16, 64)

	// Execute the YourPlace transaction based on the action code
	if version == 1 {
		switch actionPrefix {
		case 'p': // Post Actions
			core.LogDebug("Post Action: " + action)
			switch actionPostfix {
			case "":
				if postText, ok := payloadObject["p"]; ok && postText != nil {
					postTextStr := security.SanitizeNonPrintable(postText.(string))
					database.OnchainP(txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, postTextStr, blockNumber)
				}
				break
			}
			break
		case 'r': // Reply Actions
		case 'f': // Follow Actions
			core.LogDebug("Follow Action: " + action)
			switch actionPostfix {
			case "":
				blockchainPayload := payloadObject["b"].(string)
				if !security.IsValidBlockchain(blockchainPayload) {
					break
				}
				addressPayload := payloadObject["a"].(string)
				if !security.IsValidAddress(addressPayload, blockchainPayload) {
					break
				}
				if fromAddress == addressPayload && blockchain == blockchainPayload { // Ignore self-follow attempts (follower count fraud)
					break
				}
				database.OnchainF(txHash, blockchain, fromAddress, blockchain, addressPayload, blockchainPayload, timestamp)
			}
		case 'm': // Metadata Actions
			core.LogDebug("Metadata Action: " + action)
			switch actionPostfix {
			case "n":
				if name, ok := payloadObject["n"]; ok && name != nil {
					nameStr := security.SanitizeNonPrintable(payloadObject["n"].(string))
					database.OnchainMN(blockchain, fromAddress, nameStr, timestamp)
				}
			case "a":
				if avatar, ok := payloadObject["a"]; ok && avatar != nil {
					avatarStr := security.SanitizeNonPrintable(payloadObject["a"].(string))
					if security.IsValidURL(avatarStr) || security.IsValidCID(avatarStr) {
						database.OnchainMA(blockchain, fromAddress, avatarStr, timestamp)
					}
				}
			case "b":
				if banner, ok := payloadObject["b"]; ok && banner != nil {
					bannerStr := security.SanitizeNonPrintable(payloadObject["b"].(string))
					if security.IsValidURL(bannerStr) || security.IsValidCID(bannerStr) {
						database.OnchainMB(blockchain, fromAddress, bannerStr, timestamp)
					}
				}
			case "bd":
				if birthdate, ok := payloadObject["bd"]; ok && birthdate != nil {
					birthdateStr := payloadObject["bd"].(string)
					birthdateInt, _err := strconv.ParseInt(birthdateStr, 10, 64)
					if _err != nil {
						core.LogError("Could not convert YourPlace transaction birthdate: " + _err.Error())
						return
					}
					if security.IsValidBirthDate(birthdateInt) {
						database.OnchainMBD(blockchain, fromAddress, uint64(birthdateInt), timestamp)
					}
				}
			case "l":
				if location, ok := payloadObject["l"]; ok && location != nil {
					locationStr := security.SanitizeNonPrintable(payloadObject["l"].(string))
					database.OnchainML(blockchain, fromAddress, locationStr, timestamp)
				}
			case "w":
				if website, ok := payloadObject["w"]; ok && website != nil {
					websiteStr := security.SanitizeNonPrintable(payloadObject["w"].(string))
					if security.IsValidURL(websiteStr) && len(websiteStr) > 0 {
						database.OnchainMW(blockchain, fromAddress, websiteStr, timestamp)
					}
				}
			case "d":
				if description, ok := payloadObject["d"]; ok && description != nil {
					descriptionStr := security.SanitizeNonPrintable(payloadObject["d"].(string))
					if len(descriptionStr) > 0 {
						database.OnchainMD(blockchain, fromAddress, descriptionStr, timestamp)
					}
				}
			}
		case 'b': // Blocking Actions
		case 's': // Settings Actions
		default:
			core.LogError("Unknown YourPlace transaction action: " + action)
		}
	}
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
