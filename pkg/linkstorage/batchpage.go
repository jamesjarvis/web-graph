package linkstorage

import (
	"log"
	"net/url"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jamesjarvis/web-graph/pkg/linkutils"
)

//TODO: Write a batch consumer, that consumes pages from a channel in batches of max 100 and writes to the database
// https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e

// Page is a page object
type Page struct {
	U *url.URL
}

// PageBatcher is a simple batching system for recording links to the db
type PageBatcher struct {
	maxBatch     int
	bufChan      chan *Page
	s            *Storage
	killChannels []chan bool
	doneChannels []chan bool
	Cache        *lru.Cache
}

// NewPageBatcher is a helpfer function for constructing a PageBatcher object
func NewPageBatcher(maxBatch int, s *Storage) (*PageBatcher, error) {
	cache, err := lru.New(100000)
	if err != nil {
		return nil, err
	}
	return &PageBatcher{
		maxBatch: maxBatch,
		bufChan:  make(chan *Page, maxBatch*4),
		s:        s,
		Cache:    cache,
	}, nil
}

// WaitUntilEmpty returns a channel that receives input once the buffered channel is empty.
func (pb *PageBatcher) WaitUntilEmpty() <-chan bool {
	emptyChan := make(chan bool)
	go func() {
		for {
			if len(pb.bufChan) == 0 {
				emptyChan <- true
			}
		}
	}()
	return emptyChan
}

// Worker is the worker process for the page batcher
// This is straight up nicked from https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e
func (pb *PageBatcher) Worker(endSignal <-chan bool, doneChan chan<- bool) {
	// We want it to die on the endSignal, but otherwise keep looping
	for {
		select {
		case <-endSignal:
			doneChan <- true
			return
		case <-time.After(time.Millisecond):
			var pages []*Page

		Remaining:
			for i := 0; i < pb.maxBatch; i++ {
				select {
				case page := <-pb.bufChan:
					pages = append(pages, page)
				default:
					break Remaining
				}
			}

			if len(pages) == 0 {
				break
			}

			// The batch processing
			// log.Printf("Batch adding pages of size %d", len(pages))
			err := pb.s.BatchAddPages(pages)
			if err != nil {
				log.Printf("Batch adding pages failed!: %e", err)
			}

		}
	}
}

// SpawnWorkers spawns n workers, and returns a kill channel
func (pb *PageBatcher) SpawnWorkers(nWorkers int) {
	for i := 0; i < nWorkers; i++ {
		killChan := make(chan bool)
		doneChan := make(chan bool)
		pb.killChannels = append(pb.killChannels, killChan)
		pb.doneChannels = append(pb.doneChannels, doneChan)
		go pb.Worker(killChan, doneChan)
	}
}

// KillWorkers simply kills all previously spawned workers
func (pb *PageBatcher) KillWorkers() {
	for _, workerKillChan := range pb.killChannels {
		workerKillChan <- true
	}
	for _, doneChan := range pb.doneChannels {
		<-doneChan
	}
}

// AddPage is a lightweight function to just whack that page into the channel
// Returns true if it added the page (hadn't been added previously)
func (pb *PageBatcher) AddPage(page *Page) bool {
	ok, _ := pb.Cache.ContainsOrAdd(linkutils.Hash(page.U), struct{}{})
	if !ok {
		pb.bufChan <- page
		return true
	}
	return false
}
