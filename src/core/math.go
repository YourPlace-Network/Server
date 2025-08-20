package core

import (
	"regexp"
	"strconv"
	"sync"
)

func BytesToGigabytes(bytes uint64) uint64 {
	return bytes / 1024 / 1024 / 1024
}
func ParseVersionString(version string) (int, int, int) {
	// Use regex to extract version numbers, ignoring any extra characters
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(version)
	// If no matches found, return zeros
	if len(matches) != 4 {
		return 0, 0, 0
	}
	// Parse the captured groups
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, 0
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, 0
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, 0, 0
	}
	return major, minor, patch
}
func CompareVersionString(version1, version2 string) bool {
	// compare if version1 is greater than version2
	major1, minor1, patch1 := ParseVersionString(version1)
	major2, minor2, patch2 := ParseVersionString(version2)
	if major1 > major2 {
		return true
	} else if major1 == major2 {
		if minor1 > minor2 {
			return true
		} else if minor1 == minor2 {
			if patch1 > patch2 {
				return true
			}
		}
	}
	return false
}
func Abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// Thread Safe Counter
type ThreadSafeCounter struct {
	mu    sync.Mutex
	count *int64
}
type ThreadSafeMinCounter struct {
	mu       sync.Mutex
	minValue int64
}
type ThreadSafeMaxCounter struct {
	mu       sync.Mutex
	maxValue int64
}

func NewThreadSafeCounter() *ThreadSafeCounter {
	return &ThreadSafeCounter{
		count: new(int64),
	}
}
func (tsc *ThreadSafeCounter) Increment() {
	// Increment the counter
	tsc.mu.Lock()
	defer tsc.mu.Unlock()
	*tsc.count++
}
func (tsc *ThreadSafeCounter) Decrement() {
	// Decrement the counter
	tsc.mu.Lock()
	defer tsc.mu.Unlock()
	*tsc.count--
}
func (tsc *ThreadSafeCounter) Get() int64 {
	// Get the current value of the counter
	tsc.mu.Lock()
	defer tsc.mu.Unlock()
	return *tsc.count
}

func NewThreadSafeMinTracker(initialValue int64) *ThreadSafeMinCounter {
	return &ThreadSafeMinCounter{
		minValue: initialValue,
	}
}
func (t *ThreadSafeMinCounter) Update(value int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if value < t.minValue {
		t.minValue = value
	}
}
func (t *ThreadSafeMinCounter) Get() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.minValue
}

func NewThreadSafeMaxTracker(initialValue int64) *ThreadSafeMaxCounter {
	return &ThreadSafeMaxCounter{
		maxValue: initialValue,
	}
}
func (t *ThreadSafeMaxCounter) Update(value int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if value > t.maxValue {
		t.maxValue = value
	}
}
func (t *ThreadSafeMaxCounter) Get() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.maxValue
}
