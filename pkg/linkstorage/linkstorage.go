package linkstorage

import (
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/jamesjarvis/web-graph/pkg/linkutils"
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

	return nil
}

// CheckPageExists checks that the page exists in the visited database
func (s *Storage) CheckPageExists(u *url.URL) (bool, error) {
	var isVisited bool

	query := fmt.Sprintf(`SELECT EXISTS(SELECT page_id FROM %s WHERE page_id = $1)`, s.PageTable)

	s.pageLock.RLock()
	err := s.db.QueryRow(query, linkutils.Hash(u)).Scan(&isVisited)
	s.pageLock.RUnlock()
	return isVisited, err
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

	s.pageLock.Lock()
	_, err = s.db.Exec(query, linkutils.Hash(page.U), page.U.Hostname(), page.U.EscapedPath(), page.U.String())
	s.pageLock.Unlock()
	return err
}

// CheckLinkExists checks that the link exists in the visited database
func (s *Storage) CheckLinkExists(fromU *url.URL, toU *url.URL) (bool, error) {
	var isVisited bool

	query := fmt.Sprintf(`SELECT EXISTS(SELECT to_page_id FROM %s WHERE from_page_id = $1 AND to_page_id = $2)`, s.LinkTable)

	// s.linkLock.RLock()
	err := s.db.QueryRow(query, linkutils.Hash(fromU), linkutils.Hash(toU)).Scan(&isVisited)
	// s.linkLock.RUnlock()
	return isVisited, err
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
		vals = append(vals, linkutils.Hash(link.FromU), linkutils.Hash(link.ToU), link.LinkText)
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

// BatchAddPages takes a batch of pages and inserts them, not giving a fuck whether or not they clash
func (s *Storage) BatchAddPages(pages []*Page) error {
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
