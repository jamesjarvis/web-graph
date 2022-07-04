package linkstorage

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jamesjarvis/web-graph/pkg/linkutils"
	"github.com/lib/pq"
)

// Storage implements a PostgreSQL storage backend for colly
type Storage struct {
	URI       string
	PageTable string
	LinkTable string
	db        *sql.DB
	linkLock  *sync.RWMutex
	pageLock  *sync.RWMutex
}

// NewStorage is a wrapper for easily creating a storage object.
func NewStorage(
	uri string,
	pageTable string,
	linkTable string,
) (*Storage, error) {
	storage := &Storage{
		URI:       uri,
		PageTable: pageTable,
		LinkTable: linkTable,
	}
	err := storage.Init()
	if err != nil {
		return nil, err
	}
	return storage, nil
}

// KeepPingingOn periodically sends a ping to the db to keep the connection alive.
// You can kill this process by sending a boolean to the returned channel.
func (s *Storage) KeepPingingOn(d time.Duration) chan<- bool {
	ticker := time.NewTicker(d)
	killChan := make(chan bool)
	go func() {
		var err error
		for {
			select {
			case <-killChan:
				log.Println("Killing ping worker!")
				ticker.Stop()
				return
			case <-ticker.C:
				err = s.db.Ping()
				if err != nil {
					log.Printf("Error from ping: %v", err)
				}
			}
		}
	}()

	return killChan
}

// Close closes connections.
func (s *Storage) Close() error {
	return s.db.Close()
}

// Init initializes the PostgreSQL storage
func (s *Storage) Init() error {
	var err error

	if s.linkLock == nil {
		s.linkLock = &sync.RWMutex{}
	}

	if s.pageLock == nil {
		s.pageLock = &sync.RWMutex{}
	}

	if s.db, err = sql.Open("postgres", s.URI); err != nil {
		return err
	}

	if err = s.db.Ping(); err != nil {
		return err
	}

	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		page_id text NOT NULL PRIMARY KEY UNIQUE, 
		host text NOT NULL, 
		path text NOT NULL, 
		url text NOT NULL
		);`, s.PageTable)

	if _, err = s.db.Exec(query); err != nil {
		return err
	}

	query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		from_page_id text NOT NULL, 
		to_page_id text NOT NULL, 
		text text, 
		CONSTRAINT PK_Link PRIMARY KEY (from_page_id,to_page_id),
		CONSTRAINT FK_from_page_id FOREIGN KEY (from_page_id) REFERENCES %s(page_id),
		CONSTRAINT FK_to_page_id FOREIGN KEY (to_page_id) REFERENCES %s(page_id)
		);`, s.LinkTable, s.PageTable, s.PageTable)

	if _, err = s.db.Exec(query); err != nil {
		return err
	}

	query = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_to_page_id 
	ON %s(to_page_id)`, s.LinkTable)

	if _, err = s.db.Exec(query); err != nil {
		return err
	}

	query = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_page_host 
	ON %s(host)`, s.PageTable)

	if _, err = s.db.Exec(query); err != nil {
		return err
	}

	return nil
}

// CheckPageExists checks that the page exists in the visited database
func (s *Storage) CheckPageExists(u *url.URL) (bool, error) {
	var isVisited bool

	query := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE page_id = $1)`, s.PageTable)

	s.pageLock.RLock()
	err := s.db.QueryRow(query, linkutils.Hash(u)).Scan(&isVisited)
	s.pageLock.RUnlock()
	return isVisited, err
}

// GetPage retrieves info about the page hash if it exists.
func (s *Storage) GetPage(pageHash string) (*Page, error) {
	query := fmt.Sprintf(`SELECT url FROM %s WHERE page_id = $1`, s.PageTable)

	// Prepare query
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	// Execute query
	var urlString string
	s.pageLock.RLock()
	err = stmt.QueryRow(pageHash).Scan(&urlString)
	s.pageLock.RUnlock()
	if err == sql.ErrNoRows {
		// Return nothing if nothing found
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	return &Page{
		U: u,
	}, nil
}

// GetPageHashesFromHost retrieves the page hashes of all pages with this host.
func (s *Storage) GetPageHashesFromHost(host string, limit int) ([]string, error) {
	query := fmt.Sprintf(`SELECT page_id FROM %s WHERE host = $1 LIMIT $2`, s.PageTable)

	// Prepare query
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	// Execute query
	var pageHashes []string
	s.pageLock.RLock()
	rows, err := stmt.Query(host, limit)
	s.pageLock.RUnlock()
	defer rows.Close()
	for rows.Next() {
		var pageID string
		err = rows.Scan(&pageID)
		if err != nil {
			return nil, err
		}
		pageHashes = append(pageHashes, pageID)
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return pageHashes, nil
}

// AddPage first checks that it does not exist, and then inserts the page
func (s *Storage) AddPage(page *Page) error {
	visited, err := s.CheckPageExists(page.U)
	if err != nil {
		return err
	}

	if visited {
		return nil
	}

	query := fmt.Sprintf(`INSERT INTO %s (page_id, host, path, url) VALUES($1, $2, $3, $4);`, s.PageTable)

	// Prepare query
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	s.pageLock.Lock()
	_, err = stmt.Exec(linkutils.Hash(page.U), page.U.Hostname(), page.U.EscapedPath(), page.U.String())
	s.pageLock.Unlock()
	return err
}

// CheckLinkExists checks that the link exists in the visited database
func (s *Storage) CheckLinkExists(fromU *url.URL, toU *url.URL) (bool, error) {
	var isVisited bool

	query := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE from_page_id = $1 AND to_page_id = $2)`, s.LinkTable)

	// s.linkLock.RLock()
	err := s.db.QueryRow(query, linkutils.Hash(fromU), linkutils.Hash(toU)).Scan(&isVisited)
	// s.linkLock.RUnlock()
	return isVisited, err
}

// GetLinksFrom retrieves the links from this page hash.
func (s *Storage) GetLinksFrom(pageHash string, limit int) ([]string, error) {
	query := fmt.Sprintf(`SELECT to_page_id FROM %s WHERE from_page_id = $1 LIMIT $2`, s.LinkTable)

	// Prepare query
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	// Execute query
	var pageHashes []string
	s.linkLock.RLock()
	rows, err := stmt.Query(pageHash, limit)
	s.linkLock.RUnlock()
	defer rows.Close()
	for rows.Next() {
		var pageID string
		err = rows.Scan(&pageID)
		if err != nil {
			return nil, err
		}
		pageHashes = append(pageHashes, pageID)
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return pageHashes, nil
}

// GetLinksTo retrieves the links from this page hash.
func (s *Storage) GetLinksTo(pageHash string, limit int) ([]string, error) {
	query := fmt.Sprintf(`SELECT from_page_id FROM %s WHERE to_page_id = $1 LIMIT $2`, s.LinkTable)

	// Prepare query
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	// Execute query
	var pageHashes []string
	s.linkLock.RLock()
	rows, err := stmt.Query(pageHash, limit)
	s.linkLock.RUnlock()
	defer rows.Close()
	for rows.Next() {
		var pageID string
		err = rows.Scan(&pageID)
		if err != nil {
			return nil, err
		}
		pageHashes = append(pageHashes, pageID)
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return pageHashes, nil
}

// CountLinks retrieves an estimate of the number of links scraped.
func (s *Storage) CountLinks() (int, error) {
	var count int
	query := fmt.Sprintf(`SELECT reltuples::bigint AS estimate 
	FROM pg_class 
	WHERE relname='%s'`, s.LinkTable)

	// Prepare query
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return count, err
	}
	defer stmt.Close()

	// Execute query
	err = stmt.QueryRow().Scan(&count)
	if err != nil {
		return count, err
	}

	return count, nil
}

// CountPages retrieves an estimate of the number of pages scraped.
func (s *Storage) CountPages() (int, error) {
	var count int
	query := fmt.Sprintf(`SELECT reltuples::bigint AS estimate 
	FROM pg_class 
	WHERE relname='%s'`, s.PageTable)

	// Prepare query
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return count, err
	}
	defer stmt.Close()

	// Execute query
	err = stmt.QueryRow().Scan(&count)
	if err != nil {
		return count, err
	}

	return count, nil
}

// AddLink first checks that it does not exist, and then inserts the page
func (s *Storage) AddLink(link *Link) error {
	s.linkLock.Lock()
	defer s.linkLock.Unlock()
	// First, check the link already exists
	visited, err := s.CheckLinkExists(link.FromU, link.ToU)
	if err != nil {
		return err
	}

	if visited {
		return nil
	}

	// Then try to add the pages
	s.AddPage(&Page{U: link.FromU})
	s.AddPage(&Page{U: link.ToU})

	query := fmt.Sprintf(`INSERT INTO %s (from_page_id, to_page_id, text) VALUES($1, $2, $3);`, s.LinkTable)

	_, err = s.db.Exec(query, linkutils.Hash(link.FromU), linkutils.Hash(link.ToU), link.LinkText)
	return err
}

// BatchAddLinks takes a batch of links and inserts them, not giving a fuck whether or not they clash
func (s *Storage) BatchAddLinks(links []*Link) error {
	// Hmmm, not sure what to do about this page bullshit, maybe I'll make a batch process for that too
	// // Then try to add the pages
	// s.AddPage(fromU)
	// s.AddPage(toU)

	valueStrings := make([]string, 0, len(links))
	vals := []interface{}{}

	for _, link := range links {
		valueStrings = append(valueStrings, "(?, ?, ?)")
		vals = append(vals, linkutils.Hash(link.FromU), linkutils.Hash(link.ToU), strings.ToValidUTF8(link.LinkText, ""))
	}

	sqlStr := fmt.Sprintf(
		"INSERT INTO %s (from_page_id, to_page_id, text) VALUES %s ON CONFLICT DO NOTHING",
		s.LinkTable,
		strings.Join(valueStrings, ","),
	)

	//Replacing ? with $n for postgres
	sqlStr = ReplaceSQL(sqlStr, "?")

	//prepare the statement
	stmt, err := s.db.Prepare(sqlStr)
	if err != nil {
		return err
	}
	defer stmt.Close()

	//format all vals at once
	_, err = stmt.Exec(vals...)

	return err
}

// ResilientBatchAddLinks shrinks the batch sizes until it eventually works :shrug:
func (s *Storage) ResilientBatchAddLinks(links []*Link) error {
	maxRetries := 20
	var retryCount int
	var err error
	batchSize := len(links)
	tempBatch := links
	// This simply backs off retries with this shitty foreign key error.
	for batchSize >= 1 {
		err = s.BatchAddLinks(tempBatch[:batchSize])
		if err != nil {
			// If we know this kind of error, we backoff for a bit and retry the same batch.
			if pqErr, ok := err.(*pq.Error); ok {
				// Here err is of type *pq.Error, you may inspect all its fields, e.g.:
				if pqErr.Code == "23503" {
					retryCount++
					// Here the error code is a foreign_key_violation, and we can maaaybe assume that the link will eventually be added so we retry this for 10 seconds or so.
					if retryCount > 10 {
						log.Printf("retrying foreign_key_violation %d/%d\n", retryCount+1, maxRetries)
					}
					if retryCount == maxRetries {
						log.Printf("Gave up after %d retries!\n", retryCount)
						break
					}
					time.Sleep(time.Duration(retryCount) * 50 * time.Millisecond)
					continue
				}
			}
			// Here we do not know the kind of error, but know there is one.
			if batchSize > 1 {
				log.Printf("Encountered weird error with batch size %d, splitting the batch...", batchSize)
				batchSize = batchSize / 2
				// time.Sleep(50 * time.Millisecond)
				continue
			}
			log.Printf("Skipping failed message %d\n", len(links)-len(tempBatch))
		}
		// Here the batch size == 1, and there is an error we do not know, so we decide to skip that message.
		// Or if there was no error, we continue through the batch.
		tempBatch = tempBatch[batchSize:]
		batchSize = len(tempBatch)
	}
	return err
}

// BatchAddPages takes a batch of pages and inserts them, not giving a fuck whether or not they clash
func (s *Storage) BatchAddPages(pages []Page) error {
	valueStrings := make([]string, 0, len(pages))
	vals := []interface{}{}

	for _, page := range pages {
		valueStrings = append(valueStrings, "(?, ?, ?, ?)")
		vals = append(vals, linkutils.Hash(page.U), page.U.Hostname(), page.U.EscapedPath(), page.U.String())
	}

	sqlStr := fmt.Sprintf(
		"INSERT INTO %s (page_id, host, path, url) VALUES %s ON CONFLICT DO NOTHING ",
		s.PageTable,
		strings.Join(valueStrings, ","),
	)

	//Replacing ? with $n for postgres
	sqlStr = ReplaceSQL(sqlStr, "?")

	//prepare the statement
	stmt, err := s.db.Prepare(sqlStr)
	if err != nil {
		// TODO(jamesjarvis): bug here, think it is the way we create the query.
		return err
	}
	defer stmt.Close()

	//format all vals at once
	_, err = stmt.Exec(vals...)

	return err
}

// ReplaceSQL replaces the instance occurrence of any string pattern with an increasing $n based sequence
func ReplaceSQL(old, searchPattern string) string {
	tmpCount := strings.Count(old, searchPattern)
	for m := 1; m <= tmpCount; m++ {
		old = strings.Replace(old, searchPattern, "$"+strconv.Itoa(m), 1)
	}
	return old
}
