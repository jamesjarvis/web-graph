package crawler

import (
	"log"
	"net/url"
)

//TODO: Write a batch consumer, that consumes pages from a channel in batches of max 100 and writes to the database
// https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e

// Page is a page object
type Page struct {
	U *url.URL
}

// PageBatcher is a simple batching system for recording links to the db
type PageBatcher struct {
	maxProcesses int
	maxBatch     int
	bufChan      chan *Page
	s            *Storage
}

// NewPageBatcher is a helpfer function for constructing a PageBatcher object
func NewPageBatcher(maxBatch int, s *Storage) *PageBatcher {
	return &PageBatcher{
		maxProcesses: 4,
		maxBatch:     maxBatch,
		bufChan:      make(chan *Page, 5000),
		s:            s,
	}
}

// Worker is the worker process for the page batcher
// This is straight up nicked from https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e
func (pb *PageBatcher) Worker(endSignal chan bool) {
	// We want it to die on the endSignal, but otherwise keep looping
	for {
		select {
		case <-endSignal:
			return
		default:
			var pages []*Page
			pages = append(pages, <-pb.bufChan)
			remains := pb.maxBatch

		Remaining:
			for i := 0; i < remains; i++ {
				select {
				case page := <-pb.bufChan:
					pages = append(pages, page)
				default:
					break Remaining
				}
			}

			// The batch processing
			err := pb.s.BatchAddPages(pages)
			if err != nil {
				log.Printf("Batch adding pages failed!: %e", err)
			}
		}
	}
}

// AddPage is a lightweight function to just whack that page into the channel
func (pb *PageBatcher) AddPage(page *Page) {
	pb.bufChan <- page
}
