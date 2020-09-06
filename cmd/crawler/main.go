package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/jamesjarvis/web-graph/pkg/crawler"
	"github.com/zolamk/colly-postgres-storage/colly/postgres"
)

func main() {
	c := colly.NewCollector(
		colly.Async(true),
		colly.UserAgent("WebGraph v0.1 https://github.com/jamesjarvis/web-graph - This bot just follows links ¯\\_(ツ)_/¯"),
	)

	storage := &postgres.Storage{
		URI:          fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), "colly-db", os.Getenv("POSTGRES_DB")),
		VisitedTable: "colly_visited",
		CookiesTable: "colly_cookies",
	}
	crawlerStorage := crawler.Storage{
		URI:       fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), "database", os.Getenv("POSTGRES_DB")),
		PageTable: "pages_visited",
		LinkTable: "links_visited",
	}

	if err := crawlerStorage.Init(); err != nil {
		log.Fatal(err)
	}

	if err := c.SetStorage(storage); err != nil {
		log.Fatal(err)
	}

	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 5, RandomDelay: 20 * time.Second})

	// Find and visit all links
	c.OnHTML("a", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		u, err := url.Parse(link)
		if err != nil {
			log.Printf("ERROR: bad url | %s", link)
		} else {

			if u.Hostname() == "" {
				u = e.Request.URL.ResolveReference(u)
			}

			// log.Println(e.Request.URL.Hostname() + e.Request.URL.EscapedPath() + " --> " + u.Hostname() + u.EscapedPath())
			err = crawlerStorage.AddLink(e.Request.URL, u, e.Text, "anchor")
			if err != nil {
				log.Printf("ERROR: Could not log link %s --> %s | %v", e.Request.URL.String(), u.String(), err)
			}

			e.Request.Visit(link)
		}
	})

	c.OnRequest(func(r *colly.Request) {
		err := crawlerStorage.AddPage(r.URL)
		// log.Println("Visiting", r.URL.Hostname()+r.URL.EscapedPath())
		if err != nil {
			log.Printf("Could not log page %s | %v", r.URL.String(), err)
		}
	})

	log.Print("🔥🔥🔥 !!! SCRAPE AWAY !!! 🔥🔥🔥")

	interestingURLs := []string{
		"https://news.ycombinator.com/",
		"https://jamesjarvis.io/",
		"https://www.startups-list.com/",
		"https://www.indiehackers.com/",
		"https://www.cisco.com/",
		"https://thoughtmachine.net/",
		"https://www.bbc.co.uk/",
		"https://www.kent.ac.uk/",
		"https://home.cern/",
		"https://www.nasa.gov/",
		"https://www.engadget.com/",
		"https://www.webdesign-inspiration.com/",
		"https://moz.com/top500",
	}

	for _, url := range interestingURLs {
		c.Visit(url)
	}

	c.Wait()
}
