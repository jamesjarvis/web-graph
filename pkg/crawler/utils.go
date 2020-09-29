package crawler

import (
	"net/url"
)

var schemes = map[string]bool{
	"http":  true,
	"https": true,
}

// ScrapeDaTing gives us a yes/no on whether or not we should scrape the following URL, based on our opinionated filters
func ScrapeDaTing(u *url.URL) bool {
	if _, ok := schemes[u.Scheme]; !ok {
		return false
	}
	return true
}

//TODO: Write a batch consumer, that consumes links from a channel in batches of max 100 and writes to the database
// https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e

//TODO: Write a process that can spawn n batch consumers, to further speed up this shit

//TODO: Think about how to replace this queue service with a kafka queue perhaps?
