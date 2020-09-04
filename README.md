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
go run cmd/crawler/main.go
```
