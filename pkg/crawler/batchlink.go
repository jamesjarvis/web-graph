package crawler

import (
	"log"
	"net/url"
	"time"
)

//TODO: Write a batch consumer, that consumes links from a channel in batches of max 100 and writes to the database
// https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e

// Link is a link object
type Link struct {
	FromU    *url.URL
	ToU      *url.URL
	LinkText string
	LinkType string
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
		bufChan:  make(chan *Link, 20000),
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
		default:
			var links []*Link
			links = append(links, <-lb.bufChan)
			remains := lb.maxBatch

		Remaining:
			for i := 0; i < remains; i++ {
				select {
				case link := <-lb.bufChan:
					links = append(links, link)
				default:
					break Remaining
				}
			}

			// Ok I know this is a bit dirty, but basically sometimes we get foreign key issues
			// So I'm just going to keep retrying it until eventually the page is added right?
			var err error
			var retrying = true
			var count int
			for retrying {
				count++
				// The batch processing
				// log.Printf("Batch adding links of size %d", len(links))
				err = lb.s.BatchAddLinks(links)
				if err != nil {
					if count >= 15 {
						retrying = false
					} else {
						if count >= 10 {
							log.Printf("Batch adding links failed. Retrying now %d....", count)
						}
						<-time.After(time.Millisecond * time.Duration(count*200))
					}
				} else {
					retrying = false
				}
			}
		}
	}
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
