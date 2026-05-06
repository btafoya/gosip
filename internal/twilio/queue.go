package twilio

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// QueuedMessage represents a message waiting to be sent
type QueuedMessage struct {
	ID        string
	From      string
	To        string
	Body      string
	MediaURLs []string
	Retries   int
	CreatedAt time.Time
	Callback  func(sid string, err error)
}

// MessageQueue manages a queue of outbound messages with retry logic
type MessageQueue struct {
	client   *Client
	messages chan *QueuedMessage
	mu       sync.RWMutex
	pending  map[string]*QueuedMessage
	running  bool
	stopOnce sync.Once
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewMessageQueue creates a new message queue
func NewMessageQueue(client *Client) *MessageQueue {
	return &MessageQueue{
		client:   client,
		messages: make(chan *QueuedMessage, 1000),
		pending:  make(map[string]*QueuedMessage),
		stopChan: make(chan struct{}),
	}
}

// Enqueue adds a message to the queue
func (q *MessageQueue) Enqueue(msg *QueuedMessage) {
	q.mu.Lock()
	q.pending[msg.ID] = msg
	q.mu.Unlock()

	select {
	case q.messages <- msg:
	default:
		// Queue full, drop message
		q.mu.Lock()
		delete(q.pending, msg.ID)
		q.mu.Unlock()
		if msg.Callback != nil {
			msg.Callback("", errors.New("queue full"))
		}
	}
}

// Start begins processing the queue
func (q *MessageQueue) Start(ctx context.Context) {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}
	q.running = true
	q.stopChan = make(chan struct{})
	q.stopOnce = sync.Once{}
	q.mu.Unlock()

	workerCount := 4
	for i := 0; i < workerCount; i++ {
		q.wg.Add(1)
		go q.worker(ctx)
	}
}

func (q *MessageQueue) worker(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("queue worker panic recovered", "panic", r)
		}
		q.wg.Done()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.stopChan:
			return
		case msg := <-q.messages:
			q.processMessage(msg)
		}
	}
}

// Stop stops the queue processor
func (q *MessageQueue) Stop() {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return
	}
	q.running = false
	q.mu.Unlock()

	q.stopOnce.Do(func() {
		close(q.stopChan)
	})
	q.wg.Wait()
}

func (q *MessageQueue) processMessage(msg *QueuedMessage) {
	sid, err := q.client.SendSMS(msg.From, msg.To, msg.Body, msg.MediaURLs)

	q.mu.Lock()
	delete(q.pending, msg.ID)
	q.mu.Unlock()

	if msg.Callback != nil {
		msg.Callback(sid, err)
	}
}

// GetPendingCount returns the number of pending messages
func (q *MessageQueue) GetPendingCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.pending)
}

// GetQueuedCount returns the number of messages in the queue
func (q *MessageQueue) GetQueuedCount() int {
	return len(q.messages)
}
