package core

import (
	"fmt"
	"strconv"
	"time"
)

func GetTimestamp() uint64 {
	return uint64(time.Now().Unix())
}
func TimestampPlusDays(timestamp uint64, days uint64) uint64 {
	t := time.Unix(int64(timestamp), 0)
	nDaysLater := t.AddDate(0, 0, int(days))
	return uint64(nDaysLater.Unix())
}
func TimestampMinusDays(timestamp uint64, days uint64) uint64 {
	t := time.Unix(int64(timestamp), 0)
	nDaysAgo := t.AddDate(0, 0, -int(days))
	return uint64(nDaysAgo.Unix())
}
func TimestampToString(timestamp uint64) string {
	return strconv.FormatUint(timestamp, 10)
}
func TimestampToTime(timestamp uint64) time.Time {
	return time.Unix(int64(timestamp), 0)
}
func TimestampToTimeStr(timestamp uint64) string {
	t := time.Unix(int64(timestamp), 0)
	formatted := t.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("%s", formatted)
}
func StartTimer(label string) func() {
	// Usage: start := StartTimer()
	//        EndTimer(start)
	start := time.Now()
	return func() {
		elapsed := time.Since(start)
		LogInfo("Elapsed time: " + elapsed.String() + " for " + label)
	}
}
func EndTimer(start func()) {
	if start != nil {
		start()
	}
}
