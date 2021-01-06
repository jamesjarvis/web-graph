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
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/assembla/cony"
	"github.com/jamesjarvis/web-graph/pkg/linkprocessor"
	"github.com/jamesjarvis/web-graph/pkg/linkstorage"
	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
)

var (
	rabbitHost     = os.Getenv("RABBIT_HOST")
	rabbitPort     = os.Getenv("RABBIT_PORT")
	rabbitUser     = os.Getenv("RABBIT_USERNAME")
	rabbitPassword = os.Getenv("RABBIT_PASSWORD")

	dbUser     = os.Getenv("POSTGRES_USER")
	dbPassword = os.Getenv("POSTGRES_PASSWORD")
	dbDatabase = os.Getenv("POSTGRES_DB")

	dbTablePage = "pages_visited"
	dbTableLink = "links_visited"

	rabbitMQURL = "amqp://" + rabbitUser + ":" + rabbitPassword + "@" + rabbitHost + ":" + rabbitPort + "/"
	channelName = "links"

	que = &cony.Queue{
		AutoDelete: false,
		Name:       channelName,
		Durable:    true,
		Args:       amqp.Table{"x-queue-mode": "lazy"},
	}
	exc = cony.Exchange{
		Name:       "links-exchange",
		Kind:       "fanout",
		AutoDelete: false,
		Durable:    true,
	}
	bnd = cony.Binding{
		Queue:    que,
		Exchange: exc,
		Key:      "",
	}
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func main() {
	// Construct new client with the flag url
	// and default backoff policy
	c := cony.NewClient(
		cony.URL(rabbitMQURL),
		cony.Backoff(cony.DefaultBackoff),
	)

	pc := cony.NewClient(
		cony.URL(rabbitMQURL),
		cony.Backoff(cony.DefaultBackoff),
	)

	// Declarations
	// The queue name will be supplied by the AMQP server
	c.Declare([]cony.Declaration{
		cony.DeclareQueue(que),
		cony.DeclareExchange(exc),
		cony.DeclareBinding(bnd),
	})
	pc.Declare([]cony.Declaration{
		cony.DeclareQueue(que),
		cony.DeclareExchange(exc),
		cony.DeclareBinding(bnd),
	})

	// Declare and register a publisher
	// with the cony client
	pbl := cony.NewPublisher(exc.Name, "")
	pc.Publish(pbl)

	go func() {
		for pc.Loop() {
			select {
			case err := <-pc.Errors():
				log.Printf("Publishing client error: %v\n", err)
			case blocked := <-pc.Blocking():
				log.Printf("Publishing client is blocked %v\n", blocked)
			}
		}
	}()

	// Initialise database connections
	linkStorage, err := linkstorage.NewStorage(
		fmt.Sprintf(
			"postgres://%s:%s@%s:5432/%s?sslmode=disable",
			dbUser,
			dbPassword,
			"database",
			dbDatabase,
		),
		dbTablePage,
		dbTableLink,
	)
	failOnError(err, "Failed to connect to postgres")

	linkProcessor, err := linkprocessor.NewLinkProcessor(
		linkStorage,
		500,
		pbl,
		1,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Declare and register a consumer
	cns := cony.NewConsumer(
		que,
	)
	c.Consume(cns)
	for c.Loop() {
		select {
		case msg := <-cns.Deliveries():
			// Parse the URL from rabbitmq.
			uri, err := url.Parse(string(msg.Body))
			if err != nil {
				log.Printf("Bad URL received: %v", string(msg.Body))
				msg.Reject(false)
				break
			}

			err = linkProcessor.ProcessURL(uri)
			if err != nil {
				log.Printf("Error whilst processing: %v", err)
				msg.Reject(false)
				break
			}

			msg.Ack(false)
		case err := <-cns.Errors():
			log.Printf("Consumer error: %v\n", err)
		case err := <-c.Errors():
			log.Printf("Consumer client error: %v\n", err)
		case blocked := <-c.Blocking():
			log.Printf("Client is blocked %v\n", blocked)
		}
	}
}
