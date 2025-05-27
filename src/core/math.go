package core

import (
	"regexp"
	"sort"
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
type ThreadSafeInt64Bottle struct {
	// The queue fills up with INTs in sequential order, starting at a 0. It's like filling liquid into a bottle.
	// The cache is for when INTs arrive in non-sequential order, where they are queued in sorted order, for their turn to be added to the main queue.
	// The bottle always starts at the configured startingValue and fills upward in sequential order.
	mu    sync.Mutex
	queue []int64
	cache []int64
}

func NewThreadSafeCachedBottle(startingValue int64) *ThreadSafeInt64Bottle {
	bottle := &ThreadSafeInt64Bottle{
		queue: []int64{},
		cache: []int64{},
	}
	bottle.queue = append(bottle.queue, startingValue) // Pre-fill the queue with the starting value
	return bottle
}
func (b *ThreadSafeInt64Bottle) Add(value int64) {
	// Add an item to the cache queue
	b.mu.Lock()
	defer b.mu.Unlock()
	topValue := b.queue[len(b.queue)-1]
	// Check if the value is sequentially next in line
	if value > topValue {
		// If the value is not sequentially next, add it to the cache
		b.cache = append(b.cache, value)
		sort.Slice(b.cache, func(i, j int) bool { // Sort the cache in ascending order
			return b.cache[i] < b.cache[j]
		})
	}

	//b.cache.Enqueue(value)
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
