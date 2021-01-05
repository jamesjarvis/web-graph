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
	"log"
	"os"

	"github.com/streadway/amqp"
)

var (
	rabbitHost     = os.Getenv("RABBIT_HOST")
	rabbitPort     = os.Getenv("RABBIT_PORT")
	rabbitUser     = os.Getenv("RABBIT_USERNAME")
	rabbitPassword = os.Getenv("RABBIT_PASSWORD")
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

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}
