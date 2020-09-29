# Web Graph

> Experiment with web scraping

The basic idea of this is that I wanted to be able to crawl from a single URL, and scrape the entire tree of links it can traverse.

Eventually, I'll store these in a database, and create a UI so you can see the graph.

Rough overview:

Crawler is given a url.
It first checks that this url has not been crawled already, if it has, then it just returns AlreadyCrawledError
Then it checks that the url is accessible, it'll do some small exponential backoof, but then returns PageDeadError
If it can, it will download the page source, and scrape all 'a' elements, and the href attribute from that.
Then it sends all these scraped URL's to a pool of crawler workers, and the process repeats.

Eventually I'll plug ths all into a database.

## To run

```bash
docker-compose up --build -d && docker-compose logs -f crawler
```

Then open <localhost:8080> and enter your credentials from [Your database environment file](./database.env.example)

## TODO

- [x] Replace in memory visited check with postgresql (<https://github.com/zolamk/colly-postgres-storage>)
- [ ] Create schema for links graph database
- [ ] Implement saving to the db

## DB Schema

| Host ID (PK) | Host URL       |
| ------------ | -------------- |
| 1            | jamesjarvis.io |
| 2            | wikipedia.com  |

### Page

| Page ID (PK) | Host ID (FK) | Path                          |
| ------------ | ------------ | ----------------------------- |
| 11           | 1            | /projects/one-second-everyday |
| 12           | 2            | /united-kingdom               |

### Link

| Link ID (PK) | Link text                    | Link type |
| ------------ | ---------------------------- | --------- |
| 111          | I live in the United Kingdom | anchor    |

### Link to

Page to page with link

| FromPageID (FK) | ToPageID(FK) | WithLinkID (FK) |
| --------------- | ------------ | --------------- |
| 11              | 12           | 111             |

Orrrrr to make use of content addressible primary keys, we could rearrange it a bit like so, which should improve performance, but reduce the ability to backlink.

### Page

| Page ID (PK) (generated as hash of host+path) | Host           | Path            |
| --------------------------------------------- | -------------- | --------------- |
| 1 (hash of host+path)                         | jamesjarvis.io | /               |
| 2 (hash of host+path)                         | wikipedia.com  | /united-kingdom |

### Link

| FromPageID (FK) | ToPageID (FK) | Link text        | Link type |
| --------------- | ------------- | ---------------- | --------- |
| 1               | 2             | I live in the UK | anchor    |
