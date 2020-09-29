package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
	"github.com/jamesjarvis/web-graph/pkg/crawler"
	_ "github.com/lib/pq"
)

func main() {
	c := colly.NewCollector(
		colly.UserAgent("WebGraph v0.1 https://github.com/jamesjarvis/web-graph - This bot just follows links ¯\\_(ツ)_/¯"),
	)

	crawlerStorage := crawler.Storage{
		URI:       fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), "database", os.Getenv("POSTGRES_DB")),
		PageTable: "pages_visited",
		LinkTable: "links_visited",
	}

	if err := crawlerStorage.Init(); err != nil {
		log.Fatal(err)
	}

	q, _ := queue.New(
		8, // Number of consumer threads
		&queue.InMemoryQueueStorage{MaxSize: 10000},
	)

	c.Limit(&colly.LimitRule{
		DomainGlob: "*",
	})

	// Find and visit all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := strings.TrimSpace(e.Attr("href"))
		u, err := url.Parse(link)
		if err != nil {
			log.Printf("ERROR: bad url | %s", link)
			return
		}

		if u.Hostname() == "" {
			u = e.Request.URL.ResolveReference(u)
		}

		if !crawler.ScrapeDaTing(u) {
			return
		}

		// log.Println(e.Request.URL.Hostname() + e.Request.URL.EscapedPath() + " --> " + u.Hostname() + u.EscapedPath())
		err = crawlerStorage.AddLink(e.Request.URL, u, e.Text, e.Name)
		if err != nil {
			log.Printf("ERROR: Could not log link %s --> %s | %v", e.Request.URL.String(), u.String(), err)
			return
		}

		q.AddURL(link)

	})

	c.OnRequest(func(r *colly.Request) {
		if !crawler.ScrapeDaTing(r.URL) {
			r.Abort()
			return
		}

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
		q.AddURL(url)
	}

	// This little snippet enabled the go pprof tools
	// http.HandleFunc("/test", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
	// 	rw.Write([]byte("hi"))
	// }))
	// go http.ListenAndServe(":6060", nil)

	// TODO: put in utils
	go func() {
		var size int
		for !q.IsEmpty() {
			size, _ = q.Size()
			log.Printf("Queue size: %d", size)
			time.Sleep(time.Second * 60)
		}
	}()

	q.Run(c)

	log.Println("Done! 🤯")
}
