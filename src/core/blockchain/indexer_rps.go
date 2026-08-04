package blockchain

import (
	"YourPlace/src/core/db"
	"sync"
	"sync/atomic"
	"time"
)

const indexerRPSUpdateInterval = "@every 15s"

var indexerBlockRequestRPSSampler = newIndexerBlockRequestRPSSampler()

type indexerBlockRequestRPSSample struct {
	initialized  bool
	lastCount    int64
	lastSampleAt time.Time
}
type indexerBlockRequestRPSSamplerState struct {
	mu       sync.Mutex
	algorand indexerBlockRequestRPSSample
	base     indexerBlockRequestRPSSample
	ethereum indexerBlockRequestRPSSample
}

func newIndexerBlockRequestRPSSampler() *indexerBlockRequestRPSSamplerState {
	return &indexerBlockRequestRPSSamplerState{}
}
func IndexerBlockRequestRPSUpdateInterval() string {
	return indexerRPSUpdateInterval
}
func IndexerUpdateBlockRequestRPS(database *db.Database) {
	indexerBlockRequestRPSSampler.mu.Lock()
	defer indexerBlockRequestRPSSampler.mu.Unlock()
	now := time.Now()
	indexerBlockRequestRPSSampler.algorand = updateIndexerBlockRequestRPS(database, "algorand", atomic.LoadInt64(&algoTotalRequestsCount), now, indexerBlockRequestRPSSampler.algorand)
	indexerBlockRequestRPSSampler.base = updateIndexerBlockRequestRPS(database, "base", atomic.LoadInt64(&totalRequestsCount), now, indexerBlockRequestRPSSampler.base)
	indexerBlockRequestRPSSampler.ethereum = updateIndexerBlockRequestRPS(database, "ethereum", atomic.LoadInt64(&ethereumTotalRequestsCount), now, indexerBlockRequestRPSSampler.ethereum)
}
func updateIndexerBlockRequestRPS(database *db.Database, blockchain string, currentCount int64, now time.Time, sample indexerBlockRequestRPSSample) indexerBlockRequestRPSSample {
	uuid := database.IndexerGetJobUUID(blockchain)
	if uuid == "" {
		return indexerBlockRequestRPSSample{
			initialized:  true,
			lastCount:    currentCount,
			lastSampleAt: now,
		}
	}
	if !sample.initialized {
		database.IndexerUpdateJobSpeed(uuid, 0)
		return indexerBlockRequestRPSSample{
			initialized:  true,
			lastCount:    currentCount,
			lastSampleAt: now,
		}
	}
	elapsedSeconds := now.Sub(sample.lastSampleAt).Seconds()
	if elapsedSeconds <= 0 {
		database.IndexerUpdateJobSpeed(uuid, 0)
		return sample
	}
	requestDelta := currentCount - sample.lastCount
	if requestDelta < 0 {
		requestDelta = currentCount
	}
	blockRequestsPerSecond := float64(requestDelta) / elapsedSeconds
	database.IndexerUpdateJobSpeed(uuid, uint64(blockRequestsPerSecond+0.5))
	return indexerBlockRequestRPSSample{
		initialized:  true,
		lastCount:    currentCount,
		lastSampleAt: now,
	}
}
