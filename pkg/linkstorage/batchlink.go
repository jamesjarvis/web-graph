package linkstorage

import (
	"fmt"
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
}

// NewLinkBatcher is a helpfer function for constructing a LinkBatcher object
func NewLinkBatcher(maxBatch int, s *Storage) *LinkBatcher {
	return &LinkBatcher{
		maxBatch: maxBatch,
		bufChan:  make(chan *Link, maxBatch*4),
		s:        s,
	}
}

// Worker is the worker process for the link batcher
// This is straight up nicked from https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e
func (lb *LinkBatcher) Worker(endSignal chan bool) {
	// We want it to die on the endSignal, but otherwise keep looping
	for {
		select {
		case <-endSignal:
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

			err := lb.ResilientBatchAddLinks(links)
			if err != nil {
				log.Printf("Batch adding links failed!: %e", err)
			}

			// // Ok I know this is a bit dirty, but basically sometimes we get foreign key issues
			// // So I'm just going to keep retrying it until eventually the page is added right?
			// var err error
			// var retrying = true
			// var count int
			// for retrying {
			// 	count++
			// 	// The batch processing
			// 	// log.Printf("Batch adding links of size %d", len(links))
			// 	err = lb.s.BatchAddLinks(links)
			// 	if err != nil {
			// 		if count >= 15 {
			// 			log.Printf("Batch adding links failed: %v", err)
			// 			retrying = false
			// 		} else {
			// 			if count >= 10 {
			// 				log.Printf("Batch adding links failed. Retrying now %d....", count)
			// 			}
			// 			<-time.After(time.Millisecond * time.Duration(count*200))
			// 		}
			// 	} else {
			// 		retrying = false
			// 	}
			// }

		}
	}
}

// ResilientBatchAddLinks shrinks the batch sizes until it eventually works :shrug:
func (lb *LinkBatcher) ResilientBatchAddLinks(links []*Link) error {
	batchSize := len(links)
	tempBatch := links
	var err error
	maxRetries := 20
	var retryCount int
	for batchSize >= 1 {
		err = lb.s.BatchAddLinks(tempBatch[:batchSize])
		if err != nil && batchSize > 1 {
			batchSize = batchSize / 2
			continue
		}
		if batchSize == 1 {
			if pqErr, ok := err.(*pq.Error); ok {
				// Here err is of type *pq.Error, you may inspect all its fields, e.g.:
				if pqErr.Code == "23503" {
					// Here the error code is a foreign_key_violation, and we can maaaybe assume that the link will eventually be added so we retry this for 10 seconds or so.
					fmt.Printf("retrying foreign_key_violation %d/%d\n", retryCount+1, maxRetries)
					retryCount++
					if retryCount == maxRetries {
						break
					}
					time.Sleep(20 * time.Millisecond)
					continue
				}
			}
		}
		// We can reach here if the batch size == 1, in which case we skip that message and continue because fuck it.
		tempBatch = tempBatch[batchSize:]
		batchSize = len(tempBatch)
	}
	return err
}

// SpawnWorkers spawns n workers, and returns a kill channel
func (lb *LinkBatcher) SpawnWorkers(nWorkers int) {
	for i := 0; i < nWorkers; i++ {
		killChan := make(chan bool)
		go lb.Worker(killChan)
		lb.killChannels = append(lb.killChannels, killChan)
	}
}

// KillWorkers simply kills all previously spawned workers
func (lb *LinkBatcher) KillWorkers() {
	for _, workerKillChan := range lb.killChannels {
		workerKillChan <- true
	}
}

// AddLink is a lightweight function to just whack that link into the channel
func (lb *LinkBatcher) AddLink(link *Link) error {
	lb.bufChan <- link
	return nil
}
