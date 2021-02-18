# Web Graph

> Experiment with web scraping

View it live! <https://jamesjarvis.github.io/web-graph/>

If you want to start from a different url, you can change the query string!
(Note that you can only look at urls that are indirectly discoverable from the root jamesjarvis.io).

Example: <https://jamesjarvis.github.io/web-graph/?url=https://en.wikipedia.org/wiki/London>

The basic idea of this is that I wanted to be able to crawl from a single URL, and scrape the entire tree of links it can traverse.

Rough overview:

Crawler is given a url.
It first checks that this url has not been crawled already, if it has, then it just moves on.
Then it checks that the url is accessible, it'll do some small exponential backoof, but then returns PageDeadError
If it can, it will download the page source, and scrape all 'a' elements, and the href attribute from that.
Then it sends all these scraped URL's to the back of a list, and the process repeats.

Essentially, this is a breadth first crawl of the whole internet, or at least until either my 1TB hard drive runs out of space, or virgin media cuts me off.

## The API

<https://api.jamesjarvis.io>

If you want to mess about with the API directly, you need to know that the "id" of each page is calculated as the following:

> SHA1(hostname + pathname).hex()

If you want to find out the id's of pages found on a particular host, you can use: <https://api.jamesjarvis.io/pages/jamesjarvis.io>

If you want to find info of a page, along with the id's of pages linked *from* this page, use: <https://api.jamesjarvis.io/page/5bc63ce53c8aaede0889ee9e90276affbbba7573>

If you want to find the links *to* a page (v useful for discovering backlinks), use: <https://api.jamesjarvis.io/linksTo/5bc63ce53c8aaede0889ee9e90276affbbba7573>

## To run

```bash
docker-compose up --build -d && docker-compose logs -f link-processor
```

Then open <localhost:8080> and enter your credentials from [Your database environment file](./database.env.example)
Note, if running this on an rpi, stop the pgadmin service with `docker compose stop pgadmin` as it is not compiled for ARM.

To see the UI, open the `frontend/index.html` file in a browser.
## DB Schema

### Page

| Page ID (PK) (generated as hash of host+path) | Host             | Path            | Url                                  |
| --------------------------------------------- | ---------------- | --------------- | ------------------------------------ |
| 1 (hash of host+path)                         | jamesjarvis.io   | /               | https://jamesjarvis.io/              |
| 2 (hash of host+path)                         | en.wikipedia.com | /united-kingdom | https://wikipedia.com/united-kingdom |

### Link

| FromPageID (FK) | ToPageID (FK) | Link text        |
| --------------- | ------------- | ---------------- |
| 1               | 2             | I live in the UK |
