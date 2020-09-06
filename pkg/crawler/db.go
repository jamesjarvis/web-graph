package crawler

import (
	"crypto/sha1"
	"database/sql"
	"fmt"
	"net/url"
	"sync"
)

// Storage implements a PostgreSQL storage backend for colly
type Storage struct {
	URI       string
	PageTable string
	LinkTable string
	db        *sql.DB
	lock      *sync.RWMutex
}

// Init initializes the PostgreSQL storage
func (s *Storage) Init() error {

	var err error

	if s.lock == nil {
		s.lock = &sync.RWMutex{}
	}

	if s.db, err = sql.Open("postgres", s.URI); err != nil {
		return err
	}

	if err = s.db.Ping(); err != nil {
		return err
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (page_id text NOT NULL PRIMARY KEY UNIQUE, host text NOT NULL, path text NOT NULL, url text NOT NULL);", s.PageTable)

	if _, err = s.db.Exec(query); err != nil {
		return err
	}

	query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		from_page_id text NOT NULL, 
		to_page_id text NOT NULL, 
		text text, 
		type text NOT NULL, 
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

	s.lock.RLock()
	err := s.db.QueryRow(query, Hash(u)).Scan(&isVisited)
	s.lock.RUnlock()
	return isVisited, err
}

// AddPage first checks that it does not exist, and then inserts the page
func (s *Storage) AddPage(u *url.URL) error {
	visited, err := s.CheckPageExists(u)
	if err != nil {
		return err
	}

	if visited {
		return nil
	}

	query := fmt.Sprintf(`INSERT INTO %s (page_id, host, path, url) VALUES($1, $2, $3, $4);`, s.PageTable)

	s.lock.Lock()
	_, err = s.db.Exec(query, Hash(u), u.Hostname(), u.EscapedPath(), u.String())
	s.lock.Unlock()
	return err
}

// CheckLinkExists checks that the link exists in the visited database
func (s *Storage) CheckLinkExists(fromU *url.URL, toU *url.URL) (bool, error) {
	var isVisited bool

	query := fmt.Sprintf(`SELECT EXISTS(SELECT to_page_id FROM %s WHERE from_page_id = $1 AND to_page_id = $2)`, s.LinkTable)

	s.lock.RLock()
	err := s.db.QueryRow(query, Hash(fromU), Hash(toU)).Scan(&isVisited)
	s.lock.RUnlock()
	return isVisited, err
}

// AddLink first checks that it does not exist, and then inserts the page
func (s *Storage) AddLink(fromU *url.URL, toU *url.URL, linkText string, linkType string) error {
	// First, check the link already exists
	visited, err := s.CheckLinkExists(fromU, toU)
	if err != nil {
		return err
	}

	if visited {
		return nil
	}

	// Then try to add the pages
	s.AddPage(fromU)
	s.AddPage(toU)

	query := fmt.Sprintf(`INSERT INTO %s (from_page_id, to_page_id, text, type) VALUES($1, $2, $3, $4);`, s.LinkTable)

	s.lock.Lock()
	_, err = s.db.Exec(query, Hash(fromU), Hash(toU), linkText, linkType)
	s.lock.Unlock()
	return err
}

// Hash returns a SHA1 hash of the host and path
func Hash(u *url.URL) string {
	h := sha1.New()
	h.Write([]byte(u.Hostname() + u.EscapedPath()))
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
}
