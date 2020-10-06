package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
	"github.com/jamesjarvis/web-graph/pkg/crawler"
	"github.com/jamesjarvis/web-graph/pkg/queueutils"
	_ "github.com/lib/pq"
)

func main() {
	c := colly.NewCollector(
		colly.UserAgent("WebGraph v0.1 https://github.com/jamesjarvis/web-graph - This bot just follows links ¯\\_(ツ)_/¯"),
	)
	c.WithTransport(&http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 100 * time.Second,
			DualStack: true,
		}).DialContext,
		// MaxIdleConns:          100,
		IdleConnTimeout:       100 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	})

	crawlerStorage := crawler.Storage{
		URI:       fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), "database", os.Getenv("POSTGRES_DB")),
		PageTable: "pages_visited",
		LinkTable: "links_visited",
	}

	if err := crawlerStorage.Init(); err != nil {
		log.Fatal(err)
	}

	batchPages, err := crawler.NewPageBatcher(5000, &crawlerStorage)
	if err != nil {
		log.Fatal(err)
	}
	batchLinks := crawler.NewLinkBatcher(5000, &crawlerStorage)

	q, _ := queue.New(
		128, // Number of consumer threads
		&queue.InMemoryQueueStorage{MaxSize: 1000000},
	)

	c.Limit(&colly.LimitRule{
		DomainGlob: "*",
	})

	c.OnResponseHeaders(func(r *colly.Response) {
		h := strings.Split(r.Headers.Get("Content-Type"), ";")
		switch h[0] {
		case "application/xhtml+xml":
			return
		case "text/html":
			return
		default:
			r.Request.Abort()
		}
	})

	// // Set error handler
	// c.OnError(func(r *colly.Response, err error) {
	// 	log.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	// })

	// Find and visit all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := strings.TrimSpace(e.Attr("href"))
		u, err := url.Parse(link)
		if err != nil {
			// log.Printf("ERROR: bad url | %s", link)
			return
		}

		if !u.IsAbs() {
			u = e.Request.URL.ResolveReference(u)
		}

		if !crawler.ScrapeDaTing(u) {
			return
		}

		if ok, _ := c.HasVisited(e.Request.URL.String()); !ok {
			batchPages.AddPage(&crawler.Page{
				U: e.Request.URL,
			})
		}
		if ok, _ := c.HasVisited(u.String()); !ok {
			if added := batchPages.AddPage(&crawler.Page{
				U: u,
			}); added {
				q.AddURL(u.String())
			}
		}

		batchLinks.AddLink(&crawler.Link{
			FromU:    e.Request.URL,
			ToU:      u,
			LinkText: e.Text,
			LinkType: e.Name,
		})

	})

	c.OnRequest(func(r *colly.Request) {
		if !crawler.ScrapeDaTing(r.URL) {
			r.Abort()
			return
		}

		batchPages.AddPage(&crawler.Page{
			U: r.URL,
		})
	})

	log.Print("🔥🔥🔥 !!! SCRAPE AWAY !!! 🔥🔥🔥")

	interestingURLs := []string{
		"https://news.ycombinator.com/",
		"https://www.startups-list.com/",
		"https://www.indiehackers.com/",
		"https://www.cisco.com/",
		"https://thoughtmachine.net/",
		"https://www.bbc.co.uk/",
		"https://www.bbc.co.uk/news",
		"https://www.kent.ac.uk/",
		"https://home.cern/",
		"https://www.nasa.gov/",
		"https://www.engadget.com/",
		"https://www.webdesign-inspiration.com/",
		"https://moz.com/top500",
		"https://www.wired.co.uk/",
		"https://www.macrumors.com/",
		"https://jamesjarvis.io/projects",
		"https://en.wikipedia.org/wiki/Elon_Musk's_Tesla_Roadster",
		"https://en.wikipedia.org/wiki/Six_Degrees_of_Kevin_Bacon",
		"https://www.nhm.ac.uk/",
		"https://www.sciencemuseum.org.uk/",
		"https://www.businessinsider.com/uk-tech-100-2019-most-important-interesting-and-impactful-people-uk-tech-2019-9?r=US&IR=T#97-the-undergraduate-students-who-beat-apple-to-building-a-web-player-for-apple-music-4",
		"http://info.cern.ch/hypertext/WWW/TheProject.html",
		"https://www.nytimes.com/",
		"https://www.kent.ac.uk/courses/profiles/undergraduate/computer-science-year-industry-musish",
		"https://www.si.edu/",
	}

	for _, url := range interestingURLs {
		q.AddURL(url)
	}

	// This little snippet enabled the go pprof tools
	// http.HandleFunc("/test", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
	// 	rw.Write([]byte("hi"))
	// }))
	// go http.ListenAndServe(":6060", nil)

	qp := queueutils.NewQueuePrinter(q, time.Minute)
	qp.PrintQueueStats()

	// Set up batch workers
	batchLinks.SpawnWorkers(10)
	batchPages.SpawnWorkers(5)

	q.Run(c)

	qp.KillQueuePrinter()
	batchLinks.KillWorkers()
	batchPages.KillWorkers()

	log.Println("Done! 🤯")
}
