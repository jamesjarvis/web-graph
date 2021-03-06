package linkqueue

import (
	"log"
	"net/url"

	"github.com/beeker1121/goque"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jamesjarvis/web-graph/pkg/linkutils"
)

// This is a simple thread safe queue for appending and retrieving messages from a persistent, local queue.

// LinkQueue is the in memory link cache object.
type LinkQueue struct {
	queue *goque.Queue
	cache *lru.Cache
}

// NewLinkQueue initialises the cache with a default expiration duration.
func NewLinkQueue(dataDir string) (*LinkQueue, error) {
	cache, err := lru.New(10000)
	if err != nil {
		return nil, err
	}
	queue, err := goque.OpenQueue(dataDir)
	if err != nil {
		return nil, err
	}
	return &LinkQueue{
		queue: queue,
		cache: cache,
	}, nil
}

// Close closes connection to the queue.
func (q *LinkQueue) Close() error {
	return q.queue.Close()
}

// DeQueue is a blocking operation and returns a channel that receives a URL from the queue.
func (q *LinkQueue) DeQueue() <-chan *url.URL {
	foundURL := make(chan *url.URL)

	go func() {
		var link *url.URL
		var item *goque.Item
		var err error
		for {
			item, err = q.queue.Dequeue()
			if err != nil {
				continue
			}

			link, err = linkutils.ParseURL(item.ToString())
			if err != nil {
				log.Printf("Error whilst converting message %v", err)
				continue
			}
			break
		}
		foundURL <- link
	}()

	return foundURL
}

// EnQueue appends a url to the queue.
func (q *LinkQueue) EnQueue(link *url.URL) error {
	ok, _ := q.cache.ContainsOrAdd(linkutils.Hash(link), struct{}{})
	if !ok {
		_, err := q.queue.EnqueueString(link.String())
		return err
	}
	return nil
}

// Length returns length of queue.
func (q *LinkQueue) Length() uint64 {
	return q.queue.Length()
}

// ContainsItems returns true if length > 0.
func (q *LinkQueue) ContainsItems() bool {
	return q.queue.Length() > 0
}
