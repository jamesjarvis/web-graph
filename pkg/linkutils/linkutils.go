package linkutils

import (
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

// HappyResponse returns true if we want to continue scraping this thing.
func HappyResponse(resp *http.Response) bool {
	h := strings.Split(resp.Header.Get("Content-Type"), ";")
	switch h[0] {
	case "application/xhtml+xml":
		return true
	case "text/html":
		return true
	default:
		return false
	}
}

// Hash returns a SHA1 hash of the host and path
func Hash(u *url.URL) string {
	h := sha1.New()
	h.Write([]byte(u.Hostname() + u.EscapedPath()))
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
}
