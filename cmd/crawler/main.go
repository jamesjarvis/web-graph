package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/gocolly/colly"
)

func main() {
	c := colly.NewCollector(
		colly.Async(true),
		colly.UserAgent("WebGraph v0.1 https://github.com/jamesjarvis/web-graph - This bot just follows links ¯\\_(ツ)_/¯"),
	)

	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2, RandomDelay: 5 * time.Second})

	// Find and visit all links
	c.OnHTML("a", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		u, err := url.Parse(link)
		if err != nil {
			fmt.Println("bad url")
		} else {
			fmt.Println(e.Request.URL.Host + " --> " + u.Host)
			e.Request.Visit(link)
		}
	})
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	c.Visit("https://www.jamesjarvis.io")

	c.Wait()
}
