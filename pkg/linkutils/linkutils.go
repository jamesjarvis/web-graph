package linkutils

import (
	"crypto/sha1"
	"fmt"
	"net/url"
)

var (
	acceptSchemes = map[string]struct{}{
		"http":  {},
		"https": {},
	}
	ignoreHosts = map[string]struct{}{
		"t.co":          {},
		"pbs.twimg.com": {},
	}
)

// ScrapeDaTing gives us a yes/no on whether or not we should scrape the following URL, based on our opinionated filters
func ScrapeDaTing(u *url.URL) bool {
	var ok bool
	if _, ok = acceptSchemes[u.Scheme]; !ok {
		return false
	}
	if _, ok = ignoreHosts[u.Host]; ok {
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
