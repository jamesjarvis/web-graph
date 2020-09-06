package crawler

import (
	"fmt"
	"net/url"
)

// ScrapeDaTing gives us a yes/no on whether or not we should scrape the following URL, based on our opinionated filters
func ScrapeDaTing(u *url.URL) bool {
	schemes := map[string]bool{
		"http":  true,
		"https": true,
	}

	if _, ok := schemes[u.Scheme]; !ok {
		return false
	}
	fmt.Println(u)
	return true
}
