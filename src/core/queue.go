package core

import (
	"container/list"
	"sync"
)

type ThreadSafeQueue struct {
	mu   sync.Mutex
	list *list.List
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
