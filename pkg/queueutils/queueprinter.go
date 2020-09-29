package queueutils

import (
	"log"
	"time"

	"github.com/gocolly/colly/v2/queue"
)

// QueuePrinter is the config for this queue stats printing service
type QueuePrinter struct {
	q         *queue.Queue
	delay     time.Duration
	endSignal chan bool
}

// NewQueuePrinter is a helpfer function for constructing the struct
func NewQueuePrinter(q *queue.Queue, delay time.Duration) *QueuePrinter {
	return &QueuePrinter{
		q:         q,
		delay:     delay,
		endSignal: make(chan bool),
	}
}

// PrintQueueStats starts a goroutine that simply prints the number of urls in the queue every n seconds
// until either the queue is empty or the cancel trigger channel is pushed to.
func (qp *QueuePrinter) PrintQueueStats() {
	go func() {
		var size int
		for !qp.q.IsEmpty() {
			select {
			case <-qp.endSignal:
				return
			case <-time.After(qp.delay):
				size, _ = qp.q.Size()
				log.Printf("Queue size: %d", size)
			}
		}
	}()
}

// KillQueuePrinter sends a signal to the endSignal channel.
func (qp *QueuePrinter) KillQueuePrinter() {
	qp.endSignal <- true
}
