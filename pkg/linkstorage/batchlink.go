package linkstorage

import (
	"log"
	"net/url"
	"time"

	"github.com/lib/pq"
)

//TODO: Write a batch consumer, that consumes links from a channel in batches of max 100 and writes to the database
// https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e

// Link is a link object
type Link struct {
	FromU    *url.URL
	ToU      *url.URL
	LinkText string
}

// LinkBatcher is a simple batching system for recording links to the db
type LinkBatcher struct {
	maxBatch     int
	bufChan      chan *Link
	s            *Storage
	killChannels []chan bool
	doneChannels []chan bool
}

// NewLinkBatcher is a helpfer function for constructing a LinkBatcher object
func NewLinkBatcher(maxBatch int, s *Storage) *LinkBatcher {
	return &LinkBatcher{
		maxBatch: maxBatch,
		bufChan:  make(chan *Link, maxBatch*8),
		s:        s,
	}
}

// Worker is the worker process for the link batcher
// This is straight up nicked from https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e
func (lb *LinkBatcher) Worker(endSignal <-chan bool, doneChan chan<- bool) {
	// We want it to die on the endSignal, but otherwise keep looping
	for {
		select {
		case <-endSignal:
			doneChan <- true
			return
		case <-time.After(10 * time.Millisecond):
			var links []*Link

		Remaining:
			for i := 0; i < lb.maxBatch; i++ {
				select {
				case link := <-lb.bufChan:
					links = append(links, link)
				default:
					break Remaining
				}
			}

			if len(links) == 0 {
				break
			}

			// Ok I know this is a bit dirty, but basically sometimes we get foreign key issues
			// So I'm just going to keep retrying it until eventually the page is added right?

			err := lb.ResilientBatchAddLinks(links)
			if err != nil {
				log.Printf("Batch adding links failed!: %e", err)
			}
		}
	}
}

// WaitUntilEmpty returns a channel that receives input once the buffered channel is empty.
func (lb *LinkBatcher) WaitUntilEmpty() <-chan bool {
	emptyChan := make(chan bool)
	go func() {
		for {
			if len(lb.bufChan) == 0 {
				emptyChan <- true
			}
		}
	}()
	return emptyChan
}

// ResilientBatchAddLinks shrinks the batch sizes until it eventually works :shrug:
func (lb *LinkBatcher) ResilientBatchAddLinks(links []*Link) error {
	maxRetries := 20
	var retryCount int
	var err error
	batchSize := len(links)
	tempBatch := links
	// This simply backs off retries with this shitty foreign key error.
	for batchSize >= 1 {
		err = lb.s.BatchAddLinks(tempBatch[:batchSize])
		if err != nil {
			// If we know this kind of error, we backoff for a bit and retry the same batch.
			if pqErr, ok := err.(*pq.Error); ok {
				// Here err is of type *pq.Error, you may inspect all its fields, e.g.:
				if pqErr.Code == "23503" {
					retryCount++
					// Here the error code is a foreign_key_violation, and we can maaaybe assume that the link will eventually be added so we retry this for 10 seconds or so.
					if retryCount > 10 {
						log.Printf("retrying foreign_key_violation %d/%d\n", retryCount+1, maxRetries)
					}
					if retryCount == maxRetries {
						log.Printf("Gave up after %d retries!\n", retryCount)
						break
					}
					time.Sleep(time.Duration(retryCount) * 50 * time.Millisecond)
					continue
				}
			}
			// Here we do not know the kind of error, but know there is one.
			if batchSize > 1 {
				log.Printf("Encountered weird error with batch size %d, splitting the batch...", batchSize)
				batchSize = batchSize / 2
				// time.Sleep(50 * time.Millisecond)
				continue
			}
			log.Printf("Skipping failed message %d\n", len(links)-len(tempBatch))
		}
		// Here the batch size == 1, and there is an error we do not know, so we decide to skip that message.
		// Or if there was no error, we continue through the batch.
		tempBatch = tempBatch[batchSize:]
		batchSize = len(tempBatch)
	}
	return err
}

// SpawnWorkers spawns n workers, and returns a kill channel
func (lb *LinkBatcher) SpawnWorkers(nWorkers int) {
	for i := 0; i < nWorkers; i++ {
		killChan := make(chan bool)
		doneChan := make(chan bool)
		lb.killChannels = append(lb.killChannels, killChan)
		lb.doneChannels = append(lb.doneChannels, doneChan)
		go lb.Worker(killChan, doneChan)
	}
}

// KillWorkers simply kills all previously spawned workers
func (lb *LinkBatcher) KillWorkers() {
	for _, workerKillChan := range lb.killChannels {
		workerKillChan <- true
	}
	for _, doneChan := range lb.doneChannels {
		<-doneChan
	}
}

// AddLink is a lightweight function to just whack that link into the channel
func (lb *LinkBatcher) AddLink(link *Link) error {
	lb.bufChan <- link
	return nil
}
