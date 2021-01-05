package main

// The link injector should simply be used to seed a handful of links to rabbitmq to start the scraping process.

import (
	"log"
	"os"

	"github.com/streadway/amqp"
)

var (
	rabbitHost     = os.Getenv("RABBIT_HOST")
	rabbitPort     = os.Getenv("RABBIT_PORT")
	rabbitUser     = os.Getenv("RABBIT_USERNAME")
	rabbitPassword = os.Getenv("RABBIT_PASSWORD")

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
	conn, err := amqp.Dial("amqp://" + rabbitUser + ":" + rabbitPassword + "@" + rabbitHost + ":" + rabbitPort + "/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"hello", // name
		true,    // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	failOnError(err, "Failed to declare a queue")

	log.Printf("Seeding initial URL's to the queue!")

	for _, u := range interestingURLs {
		err = ch.Publish(
			"",     // exchange
			q.Name, // routing key
			false,  // mandatory
			false,  // immediate
			amqp.Publishing{
				ContentType:  "text/plain",
				Body:         []byte(u),
				DeliveryMode: amqp.Persistent,
			},
		)
		log.Printf(" [x] Sent %s", u)
		failOnError(err, "Failed to publish a message")
	}

	log.Printf("======= All done! =======")
}
