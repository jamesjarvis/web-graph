package linkprocessor

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jamesjarvis/web-graph/pkg/linkcache"
	"github.com/jamesjarvis/web-graph/pkg/linkqueue"
	"github.com/jamesjarvis/web-graph/pkg/linkstorage"
	"github.com/jamesjarvis/web-graph/pkg/linkutils"
	"github.com/ncruces/go-dns"
)

// LinkProcessor contains all connections necessary for accessing the cache, db and channel for sending urls back to rabbitmq.
type LinkProcessor struct {
	httpClient  *http.Client
	cache       *linkcache.LinkCache
	linkBatcher *linkstorage.LinkBatcher
	pageBatcher *linkstorage.PageBatcher
	storage     *linkstorage.Storage
	queue       *linkqueue.LinkQueue
	urlChan     chan *url.URL
}

// NewLinkProcessor is a helper function for creating the LinkProcessor.
func NewLinkProcessor(
	storage *linkstorage.Storage,
	batchSize int,
	queue *linkqueue.LinkQueue,
	numWorkers int,
) (*LinkProcessor, error) {
	pageBatcher, err := linkstorage.NewPageBatcher(
		batchSize,
		storage,
	)
	if err != nil {
		return nil, err
	}
	linkBatcher := linkstorage.NewLinkBatcher(
		batchSize,
		storage,
	)
	linkBatcher.SpawnWorkers(numWorkers)
	pageBatcher.SpawnWorkers(numWorkers)
	client, err := createHTTPClient()
	if err != nil {
		return nil, err
	}
	return &LinkProcessor{
		cache:       linkcache.NewLinkCache(2 * 24 * time.Hour),
		storage:     storage,
		linkBatcher: linkBatcher,
		pageBatcher: pageBatcher,
		queue:       queue,
		httpClient:  client,
	}, nil
}

func createHTTPClient() (*http.Client, error) {
	resolver, err := dns.NewDoHResolver(
		"https://cloudflare-dns.com/dns-query{?dns}",
		dns.DoHAddresses("1.1.1.1", "1.0.0.1", "2606:4700:4700::1111", "2606:4700:4700::1001"),
	)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 100 * time.Second,
				DualStack: true,
				Resolver:  resolver,
			}).DialContext,
			IdleConnTimeout:       100 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 2 * time.Second,
		},
	}, nil
}

// GracefulShutdown returns a channel that receives true when it has finished flushing the db batching cache / finished writing to the queue.
func (lp *LinkProcessor) GracefulShutdown() <-chan bool {
	readyToKill := make(chan bool)

	go func() {
		close(lp.urlChan)

		// Check if link/page batching has finished.
		<-lp.linkBatcher.WaitUntilEmpty()
		log.Println("Link batcher empty...")
		lp.linkBatcher.KillWorkers()
		log.Println("Link batcher shut down.")

		<-lp.pageBatcher.WaitUntilEmpty()
		log.Println("Page batcher empty...")
		lp.pageBatcher.KillWorkers()
		log.Println("Page batcher shut down.")

		readyToKill <- true
	}()

	return readyToKill
}

// SpawnWorkers vaguely spawns up n number of workers, that can then be communicated with by pushing urls to the channel.
func (lp *LinkProcessor) SpawnWorkers(n int) chan *url.URL {
	lp.urlChan = make(chan *url.URL)

	for w := 1; w <= n; w++ {
		go func(workerID int) {
			var err error
			for u := range lp.urlChan {
				err = lp.ProcessURL(u)
				if err != nil {
					log.Printf("Error whilst processing: %v", err)
				}
			}
			log.Printf("Worker %d finished :)", workerID)
		}(w)
	}

	return lp.urlChan
}

// Close immediately kills batching workers.
func (lp *LinkProcessor) Close() {
	lp.linkBatcher.KillWorkers()
	lp.pageBatcher.KillWorkers()
}

// CheckURLExists initially checks the in memory cache forthe url, and returns true if found.
// If the url is not in the in-memory cache it will check the db, and returns true/update cache if found.
// If not found in db or cache, then returns false.
func (lp *LinkProcessor) CheckURLExists(u *url.URL) (bool, error) {
	found := lp.cache.Get(u)
	if found {
		return true, nil
	}
	return false, nil
	// found, err := lp.storage.CheckPageExists(u)
	// if found {
	// 	// If not in cache, but in db, update in memory cache and return true.
	// 	lp.cache.Set(u)
	// }
	// return found, err
}

// MarkURLVisited sets the link as visited in cache
func (lp *LinkProcessor) MarkURLVisited(u *url.URL) {
	lp.cache.Set(u)
}

func (lp *LinkProcessor) queueURL(u *url.URL) error {
	return lp.queue.EnQueue(u)
}

// ScrapeLinksFromURL takes a url to scrape, retrieves the page and returns all links found.
func (lp *LinkProcessor) ScrapeLinksFromURL(u *url.URL) ([]*linkstorage.Link, error) {
	if !linkutils.ScrapeDaTing(u) {
		return nil, fmt.Errorf("We do not care about %s", u)
	}

	// Create and modify HTTP request before sending
	request, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "WebGraph v0.2 https://github.com/jamesjarvis/web-graph - This bot just follows links ¯\\_(ツ)_/¯")

	// Make HTTP request
	response, err := lp.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if !linkutils.HappyResponse(response) {
		return nil, fmt.Errorf("Bad content type from %s", u)
	}

	// Create a goquery document from the HTTP response
	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, err
	}

	foundLinks := []*linkstorage.Link{}

	// Find all links and process them
	document.Find("a").Each(
		func(index int, element *goquery.Selection) {
			// See if the href attribute exists on the element
			href, exists := element.Attr("href")
			if !exists {
				return
			}
			href = strings.TrimSpace(href)

			link, err := url.Parse(href)
			if err != nil {
				// log.Printf("Failed to parse URL: %v", href)
				return
			}

			if !link.IsAbs() {
				link = u.ResolveReference(link)
			}

			if !linkutils.ScrapeDaTing(link) {
				return
			}

			tempURL := &linkstorage.Link{
				FromU:    u,
				ToU:      link,
				LinkText: element.Text(),
			}
			foundLinks = append(foundLinks, tempURL)
		},
	)

	return foundLinks, nil
}

// ProcessURL takes a url and processes it.
func (lp *LinkProcessor) ProcessURL(u *url.URL) error {
	// Check if the URL has been visited already.
	exists, err := lp.CheckURLExists(u)
	if err != nil {
		log.Printf("Could not check if URL has been visited: %v\n", err)
		return err
	}
	if exists {
		return nil
	}

	// Mark as visited and save page to DB
	lp.MarkURLVisited(u)
	lp.pageBatcher.AddPage(&linkstorage.Page{U: u})

	// Retrieve html, parse links
	var links []*linkstorage.Link
	links, err = lp.ScrapeLinksFromURL(u)
	if err != nil {
		return err
	}

	for _, link := range links {
		exists, err := lp.CheckURLExists(link.ToU)
		if err != nil {
			log.Printf("Could not check if URL has been visited: %v\n", err)
			return err
		}
		if !exists {
			// this appends the link URL's to be scraped
			err = lp.queueURL(link.ToU)
			if err != nil {
				log.Printf("Could not queue url: %v", err)
			}

			// This saves each link page to db and the link
			lp.pageBatcher.AddPage(&linkstorage.Page{U: link.ToU})
		}

		lp.linkBatcher.AddLink(link)
	}

	links = nil
	return nil
}
