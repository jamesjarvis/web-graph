package linkstorage

import (
	"log"
	"net/url"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jamesjarvis/massivelyconcurrentsystems/pool"
	"github.com/jamesjarvis/web-graph/pkg/linkutils"
)

// Page is a page object
type Page struct {
	U *url.URL
}

// NewPageBatcher is a helpfer function for constructing a PageBatcher object
func NewPageBatcher(s *Storage, config pool.Config) (*pool.WorkDispatcher[pool.UnitOfWork[Page, bool]], error) {
	cache, err := lru.New(100000)
	if err != nil {
		return nil, err
	}

	batchWorker := func(us []pool.UnitOfWork[Page, bool]) error {
		// The batch processing
		// log.Printf("Batch adding pages of size %d", len(pages))

		pages := make([]Page, 0, len(us))
		for _, p := range us {
			ok, _ := cache.ContainsOrAdd(linkutils.Hash(p.GetRequest().U), struct{}{})
			if ok {
				continue
			}
			pages = append(pages, p.GetRequest())
		}

		err := s.BatchAddPages(pages)
		if err != nil {
			log.Printf("Batch adding pages failed!: %e", err)
			return err
		}

		return nil
	}

	batchDispatcher := pool.NewBatchDispatcher(
		batchWorker,
		config,
	)
	return batchDispatcher, nil
}
