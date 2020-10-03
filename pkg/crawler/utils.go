package crawler

import (
	"crypto/sha1"
	"fmt"
	"net/url"
)

var schemes = map[string]bool{
	"http":  true,
	"https": true,
}

var ignoreHosts = map[string]bool{
	"t.co":          true,
	"pbs.twimg.com": true,
}

// ScrapeDaTing gives us a yes/no on whether or not we should scrape the following URL, based on our opinionated filters
func ScrapeDaTing(u *url.URL) bool {
	if _, ok := schemes[u.Scheme]; !ok {
		return false
	}
	if _, toIgnore := ignoreHosts[u.Scheme]; toIgnore {
		return false
	}
	return true
}

// Hash returns a SHA1 hash of the host and path
func Hash(u *url.URL) string {
	h := sha1.New()
	h.Write([]byte(u.Hostname() + u.EscapedPath()))
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
}

//TODO: Write a batch consumer, that consumes links from a channel in batches of max 100 and writes to the database
// https://blog.drkaka.com/batch-get-from-golangs-buffered-channel-9638573f0c6e

//TODO: Write a process that can spawn n batch consumers, to further speed up this shit

//TODO: Think about how to replace this queue service with a kafka queue perhaps?
