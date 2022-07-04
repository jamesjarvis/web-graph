package main

// This LinkProcessor should consume links from a rabbitmq channel.

// it should have a cache for checking whether or not it has seen the link, and immediately discard if so.
// if the URL is not valid, it should immediately discard.

// Otherwise, request URL + scrape for all URLS.
// Save all URLs found to the db in bulk if possible.

// Mark the current URL as seen in the in memory cache.
// Perhaps at this point mark the current URL as visited in the DB (as a secondary cache), with the timestamp.

// Then send all URLs back to the rabbitmq channel.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamesjarvis/massivelyconcurrentsystems/pool"
	"github.com/jamesjarvis/web-graph/pkg/linkprocessor"
	"github.com/jamesjarvis/web-graph/pkg/linkqueue"
	"github.com/jamesjarvis/web-graph/pkg/linkstorage"
	"github.com/jamesjarvis/web-graph/pkg/linkutils"
	_ "github.com/lib/pq"
)

var (
	dbUser     = os.Getenv("POSTGRES_USER")
	dbPassword = os.Getenv("POSTGRES_PASSWORD")
	dbDatabase = os.Getenv("POSTGRES_DB")
	dbHost     = os.Getenv("POSTGRES_HOST")

	dbTablePage = "pages_visited"
	dbTableLink = "links_visited"

	queueDataDir = os.Getenv("QUEUE_DATA")

	defaultBatchInterval = time.Second
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func seedInitialURLs(q *linkqueue.LinkQueue) error {
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

	for _, u := range interestingURLs {
		uri, err := linkutils.ParseURL(u)
		if err != nil {
			return err
		}

		err = q.EnQueue(uri)
		if err != nil {
			return err
		}
	}

	if q.Length() == 0 {
		return errors.New("Queue is still empty??")
	}

	return nil
}

func main() {
	// Initialise database connections
	linkStorage, err := linkstorage.NewStorage(
		fmt.Sprintf(
			"postgres://%s:%s@%s:5432/%s?sslmode=disable&client_encoding=UTF8",
			dbUser,
			dbPassword,
			dbHost,
			dbDatabase,
		),
		dbTablePage,
		dbTableLink,
	)
	failOnError(err, "Failed to connect to postgres")
	defer func() {
		err := linkStorage.Close()
		log.Println("===== closed link storage =====", err)
	}()

	pageBatcher, err := linkstorage.NewPageBatcher(
		linkStorage,
		pool.NewConfig(
			pool.SetBufferSize(100),
			pool.SetBatchSize(100),
			pool.SetNumConsumers(1),
			pool.SetBatchInterval(defaultBatchInterval),
		),
	)
	if err != nil {
		log.Fatal("failed to create page batcher", err)
	}
	defer func() {
		err := pageBatcher.Close()
		log.Println("===== closed page batcher =====", err)
	}()

	linkBatcher, err := linkstorage.NewLinkBatcher(
		linkStorage,
		pool.NewConfig(
			pool.SetBufferSize(100),
			pool.SetBatchSize(100),
			pool.SetNumConsumers(1),
			pool.SetBatchInterval(defaultBatchInterval),
		),
	)
	if err != nil {
		log.Fatal("failed to create page batcher", err)
	}
	defer func() {
		err := linkBatcher.Close()
		log.Println("===== closed link batcher =====", err)
	}()

	queue, err := linkqueue.NewLinkQueue(queueDataDir)
	failOnError(err, "Failed to initialise queue")
	defer func() {
		err := queue.Close()
		log.Println("===== closed link queue =====", err)
	}()

	linkProcessor, err := linkprocessor.NewLinkProcessor(
		pageBatcher,
		linkBatcher,
		queue,
	)
	if err != nil {
		log.Fatal("failed to create link processor", err)
	}

	worker := func(u *url.URL) {
		if u == nil {
			return
		}
		err := linkProcessor.ProcessURL(u)
		if err != nil {
			log.Printf("Error whilst processing: %v", err)
		}
	}

	linkProcessorPool := pool.NewSingleDispatcher(
		worker,
		pool.NewConfig(
			pool.SetNumConsumers(2),
			pool.SetBatchInterval(defaultBatchInterval),
			pool.SetBufferSize(10),
		),
	)
	defer func() {
		err := linkProcessorPool.Close()
		log.Println("===== closed link processor =====", err)
	}()

	log.Println("Processor initialised! 🤖")

	if !queue.ContainsItems() {
		log.Println("Queue empty, seeding initial URLs to Queue...")
		err := seedInitialURLs(queue)
		failOnError(err, "Failed to seed initial URLs")
	}

	log.Println("Begin processing...")
	linkBatcher.Start()
	pageBatcher.Start()
	linkProcessorPool.Start()

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL)
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

running:
	for {
		select {
		case s := <-sigs:
			log.Printf("Received signal %s, shutting down gracefully...\n", s)
			break running
		case url := <-queue.DeQueue():
			linkProcessorPool.Put(context.TODO(), url)
		case <-ticker.C:
			log.Printf("%d urls in the queue", queue.Length())
		}
	}

	log.Println("====== Thank you, come again! ======")
}
