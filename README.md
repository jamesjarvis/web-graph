# Web Graph

> Experiment with web scraping

The basic idea of this is that I wanted to be able to crawl from a single URL, and scrape the entire tree of links it can traverse.

Eventually, I'll store these in a database, and create a UI so you can see the graph.

Rough overview:

Crawler is given a url.
It first checks that this url has not been crawled already, if it has, then it just moves on.
Then it checks that the url is accessible, it'll do some small exponential backoof, but then returns PageDeadError
If it can, it will download the page source, and scrape all 'a' elements, and the href attribute from that.
Then it sends all these scraped URL's to a pool of crawler workers, and the process repeats.

## To run

```bash
docker-compose up --build -d && docker-compose logs -f crawler
```

Then open <localhost:8080> and enter your credentials from [Your database environment file](./database.env.example)
And open <localhost:15672> with guest, guest to see the queue status.

## TODO

- [x] Replace in memory visited check with postgresql (<https://github.com/zolamk/colly-postgres-storage>)
- [x] Create schema for links graph database
- [x] Implement saving to the db
- [x] Hit performance target of 100 links/second

## DB Schema

### Page

| Page ID (PK) (generated as hash of host+path) | Host           | Path            | Url                                  |
| --------------------------------------------- | -------------- | --------------- | ------------------------------------ |
| 1 (hash of host+path)                         | jamesjarvis.io | /               | https://jamesjarvis.io/              |
| 2 (hash of host+path)                         | wikipedia.com  | /united-kingdom | https://wikipedia.com/united-kingdom |

### Link

| FromPageID (FK) | ToPageID (FK) | Link text        |
| --------------- | ------------- | ---------------- |
| 1               | 2             | I live in the UK |
