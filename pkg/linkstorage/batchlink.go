package linkstorage

import (
	"log"
	"net/url"

	"github.com/jamesjarvis/massivelyconcurrentsystems/pool"
)

// Link is a link object
type Link struct {
	FromU    *url.URL
	ToU      *url.URL
	LinkText string
}

// NewLinkBatcher is a helpfer function for constructing a LinkBatcher object
func NewLinkBatcher(s *Storage, config pool.Config) (*pool.WorkDispatcher[pool.UnitOfWork[*Link, bool]], error) {
	batchWorker := func(us []pool.UnitOfWork[*Link, bool]) error {
		// The batch processing
		links := make([]*Link, 0, len(us))
		for _, p := range us {
			links = append(links, p.GetRequest())
		}

		err := s.ResilientBatchAddLinks(links)
		if err != nil {
			log.Printf("Batch adding links failed!: %e", err)
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
