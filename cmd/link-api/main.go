package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jamesjarvis/web-graph/pkg/linkstorage"
)

var (
	dbUser     = os.Getenv("POSTGRES_USER")
	dbPassword = os.Getenv("POSTGRES_PASSWORD")
	dbDatabase = os.Getenv("POSTGRES_DB")
)

const (
	dbTablePage = "pages_visited"
	dbTableLink = "links_visited"
	queryLimit  = 100
)

type OutputJSON struct {
	Node  NodeJSON `json:"node"`
	Links []string `json:"links"`
}

type NodeJSON struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	URL   string `json:"url"`
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func main() {
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
	defer linkStorage.Close()

	r := gin.Default()

	r.GET("/page/:id", func(c *gin.Context) {
		id := c.Param("id")
		page, err := linkStorage.GetPage(id)
		if err != nil {
			log.Println(err)
			c.String(http.StatusInternalServerError, "Something wrong with DB while fetching page info?")
			return
		}
		if page == nil {
			c.String(http.StatusNotFound, "Nothing found for %s", id)
			return
		}

		linksFrom, err := linkStorage.GetLinksFrom(id, queryLimit)
		if err != nil {
			log.Println(err)
			c.String(http.StatusInternalServerError, "Something wrong with DB while fetching links?")
			return
		}

		outputjson := OutputJSON{
			Node: NodeJSON{
				ID:    id,
				Group: page.U.Host,
				URL:   page.U.String(),
			},
			Links: linksFrom,
		}

		c.JSON(http.StatusOK, outputjson)
		// we want to return something like:
		// {
		// 	"node": {
		// 		"id": "hash",
		// 		"group": "jamesjarvis.io",
		// 		"url": "https://jamesjarvis.io",
		// 	},
		// 	"links": [
		// 		"hash_1",
		// 		"hash_2",
		// 	]
		// }
	})

	r.GET("/pages/:host", func(c *gin.Context) {
		host := c.Param("host")
		hashes, err := linkStorage.GetPageHashesFromHost(host, queryLimit)
		if err != nil {
			log.Println(err)
			c.String(http.StatusInternalServerError, "Something wrong with DB?")
			return
		}

		c.JSON(http.StatusOK, hashes)
	})

	r.GET("/linksFrom/:id", func(c *gin.Context) {
		id := c.Param("id")
		hashes, err := linkStorage.GetLinksFrom(id, queryLimit)
		if err != nil {
			log.Println(err)
			c.String(http.StatusInternalServerError, "Something wrong with DB?")
			return
		}

		c.JSON(http.StatusOK, hashes)
	})

	r.GET("/linksTo/:id", func(c *gin.Context) {
		id := c.Param("id")
		hashes, err := linkStorage.GetLinksTo(id, queryLimit)
		if err != nil {
			log.Println(err)
			c.String(http.StatusInternalServerError, "Something wrong with DB?")
			return
		}

		c.JSON(http.StatusOK, hashes)
	})

	log.Fatal(r.Run())
}
