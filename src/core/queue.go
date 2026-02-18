package core

import (
	"container/list"
	"sync"
)

type dedupeCall struct {
	wg  sync.WaitGroup
	err error
	val interface{}
}
type DedupeQueue struct {
	mu    sync.Mutex
	calls map[string]*dedupeCall
}
type ThreadSafeQueue struct {
	mu   sync.Mutex
	list *list.List
}

func NewDedupeQueue() *DedupeQueue {
	return &DedupeQueue{calls: make(map[string]*dedupeCall)}
}
func (d *DedupeQueue) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	d.mu.Lock()
	if call, ok := d.calls[key]; ok {
		d.mu.Unlock()
		LogDebug("DedupeQueue: coalescing duplicate request: " + key)
		call.wg.Wait()
		return call.val, call.err
	}
	call := &dedupeCall{}
	call.wg.Add(1)
	d.calls[key] = call
	d.mu.Unlock()
	call.val, call.err = fn()
	call.wg.Done()
	d.mu.Lock()
	delete(d.calls, key)
	d.mu.Unlock()
	return call.val, call.err
}

func NewThreadSafeQueue() *ThreadSafeQueue {
	return &ThreadSafeQueue{
		list: list.New(),
	}
}
func (q *ThreadSafeQueue) Enqueue(value interface{}) {
	// Add an item to the end of the queue
	q.mu.Lock()
	defer q.mu.Unlock()
	q.list.PushBack(value)
}
func (q *ThreadSafeQueue) Dequeue() (interface{}, bool) {
	// Remove and return the item at the front of the queue. Returns item and a bool indicating if an item was available
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.list.Len() == 0 {
		return nil, false
	}
	element := q.list.Front()
	q.list.Remove(element)
	return element.Value, true
}
func (q *ThreadSafeQueue) Peek() (interface{}, bool) {
	// Return the item at the front of the queue without removing it. Returns item and a bool indicating if an item was available
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.list.Len() == 0 {
		return nil, false
	}
	return q.list.Front().Value, true
}
func (q *ThreadSafeQueue) Size() int {
	// Return the number of items in the queue
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.list.Len()
}
func (q *ThreadSafeQueue) IsEmpty() bool {
	// Check if the queue is empty
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.list.Len() == 0
}
