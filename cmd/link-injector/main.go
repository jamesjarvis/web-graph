package main

// The link injector should simply be used to seed a handful of links to rabbitmq to start the scraping process.

import (
	"log"
	"os"

	"github.com/assembla/cony"
	"github.com/streadway/amqp"
)

var (
	rabbitHost     = os.Getenv("RABBIT_HOST")
	rabbitPort     = os.Getenv("RABBIT_PORT")
	rabbitUser     = os.Getenv("RABBIT_USERNAME")
	rabbitPassword = os.Getenv("RABBIT_PASSWORD")

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

	interestingURLs = []string{
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
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func main() {
	// Construct new client with the flag url
	// and default backoff policy
	pc := cony.NewClient(
		cony.URL(rabbitMQURL),
		cony.Backoff(cony.DefaultBackoff),
	)

	// Declarations
	// The queue name will be supplied by the AMQP server
	pc.Declare([]cony.Declaration{
		cony.DeclareQueue(que),
		cony.DeclareExchange(exc),
		cony.DeclareBinding(bnd),
	})

	// Declare and register a publisher
	// with the cony client
	pbl := cony.NewPublisher(exc.Name, "")
	pc.Publish(pbl)

	log.Printf("Seeding initial URL's to the queue!")

	go func() {
		for _, u := range interestingURLs {
			err := pbl.Publish(
				amqp.Publishing{
					ContentType:  "text/plain",
					Body:         []byte(u),
					DeliveryMode: amqp.Persistent,
				},
			)
			log.Printf(" [x] Sent %s", u)
			if err != nil {
				log.Printf("Client publish error: %v\n", err)
			}
		}
		pc.Close()
	}()

	// Client loop sends out declarations(exchanges, queues, bindings
	// etc) to the AMQP server. It also handles reconnecting.
	for pc.Loop() {
		select {
		case err := <-pc.Errors():
			log.Printf("Client error: %v\n", err)
		case blocked := <-pc.Blocking():
			log.Printf("Client is blocked %v\n", blocked)
		}
	}

	log.Printf("======= All done! =======")
}
