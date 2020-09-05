package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gocolly/colly"
	"github.com/zolamk/colly-postgres-storage/colly/postgres"
)

func main() {
	c := colly.NewCollector(
		colly.Async(true),
		colly.UserAgent("WebGraph v0.1 https://github.com/jamesjarvis/web-graph - This bot just follows links ¯\\_(ツ)_/¯"),
	)

	storage := &postgres.Storage{
		URI:          fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), "database", os.Getenv("POSTGRES_DB")),
		VisitedTable: "colly_visited",
		CookiesTable: "colly_cookies",
	}
	log.Print(storage.URI)

	if err := c.SetStorage(storage); err != nil {
		log.Fatal(err)
	}

	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2, RandomDelay: 5 * time.Second})

	// Find and visit all links
	c.OnHTML("a", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		u, err := url.Parse(link)
		if err != nil {
			log.Println("bad url")
		} else {
			log.Println(e.Request.URL.Host + " --> " + u.Host)
			e.Request.Visit(link)
		}
	})
	c.OnRequest(func(r *colly.Request) {
		log.Println("Visiting", r.URL)
	})

	c.Visit("https://www.jamesjarvis.io")

	c.Wait()
}
