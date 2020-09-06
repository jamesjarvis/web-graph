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

	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2, RandomDelay: 20 * time.Second})

	// Find and visit all links
	c.OnHTML("a", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		u, err := url.Parse(link)
		if err != nil {
			log.Println("bad url")
		} else {

			if u.Hostname() == "" {
				u = e.Request.URL.ResolveReference(u)
			}

			log.Println(e.Request.URL.Hostname() + e.Request.URL.EscapedPath() + " --> " + u.Hostname() + u.EscapedPath())
			err = crawlerStorage.AddLink(e.Request.URL, u, e.Text, "anchor")
			if err != nil {
				log.Fatal("Could not log link | " + err.Error())
			}

			e.Request.Visit(link)
		}
	})

	c.OnRequest(func(r *colly.Request) {
		err := crawlerStorage.AddPage(r.URL)
		log.Println("Visiting", r.URL.Hostname()+r.URL.EscapedPath())
		if err != nil {
			log.Fatal("Could not log page | " + err.Error())
		}
	})

	c.Visit("https://jamesjarvis.io/")

	c.Wait()
}
