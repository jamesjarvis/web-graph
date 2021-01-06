package linkcache

import (
	"net/url"
	"time"

	"github.com/jamesjarvis/web-graph/pkg/linkutils"
	cache "github.com/patrickmn/go-cache"
)

// This is a simple thread safe cache for checking if a page has been visited.

// LinkCache is the in memory link cache object.
type LinkCache struct {
	cache *cache.Cache
	// I'm not even sure if this helps, but it means that we have a pointer to nothing stored in each of the values.
	emptyObject *struct{}
}

// NewLinkCache initialises the cache with a default expiration duration.
func NewLinkCache(defaultExpiration time.Duration) *LinkCache {
	return &LinkCache{
		cache:       cache.New(defaultExpiration, time.Hour),
		emptyObject: &struct{}{},
	}
}

// Set allows you to add a url to the cache to be marked as "seen".
func (lc *LinkCache) Set(u *url.URL) {
	lc.cache.SetDefault(linkutils.Hash(u), lc.emptyObject)
}

// Get returns true if the url has been "seen" in the cache, otherwise false.
func (lc *LinkCache) Get(u *url.URL) bool {
	_, seen := lc.cache.Get(linkutils.Hash(u))
	return seen
}
