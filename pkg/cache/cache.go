// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package cache

import (
	"database/sql"
	"net/url"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

const (
	driverName = "sqlite3"
)

// Storage implements a SQLite3 caching backend for Colly.
type Storage struct {
	// Sqlite filename.
	Filename string
	// Duration till which a link is skipped.
	Validity time.Duration
	// Clear cache at start if true.
	ClearCache bool
	// Database handle.
	dbHandle *sql.DB
	// Mutex used for clearing cache database.
	mu sync.RWMutex
}

// Init initializes cache database.
func (s *Storage) Init() error {
	// Check if db exists.
	if s.dbHandle == nil {
		database, err := sql.Open(driverName, s.Filename)
		if err != nil {
			return errors.Wrap(err, "unable to open cache database file")
		}

		err = database.Ping()
		if err != nil {
			return errors.Wrap(err, "verify connection to cache database")
		}
		s.dbHandle = database
	}
	if s.ClearCache {
		err := s.Clear()
		if err != nil {
			return err
		}
	}
	// Create db with index.
	statement, err := s.dbHandle.Prepare("CREATE TABLE IF NOT EXISTS visited (id INTEGER PRIMARY KEY, requestID INTEGER, visited INT, timestamp DATETIME)")
	if err != nil {
		return err
	}
	_, err = statement.Exec()
	if err != nil {
		return err
	}
	statement, err = s.dbHandle.Prepare("CREATE INDEX IF NOT EXISTS idx_visited ON visited (requestID)")
	if err != nil {
		return err
	}
	_, err = statement.Exec()
	if err != nil {
		return err
	}
	return nil
}

// Clear removes all entries from cache.
func (s *Storage) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	statement, err := s.dbHandle.Prepare("DROP TABLE visited")
	if err != nil {
		return err
	}
	_, err = statement.Exec()
	if err != nil {
		return err
	}
	return nil
}

// Close cache database.
func (s *Storage) Close() error {
	err := s.dbHandle.Close()
	return err
}

// Visited inserts new URL to cache database.
func (s *Storage) Visited(requestID uint64) error {
	// If particular URL is already inserted, then delete.
	// Visit method will only be called if validity expires for a URL or in case of a new URL.
	err := s.DeleteRequest(requestID)
	if err != nil {
		return err
	}

	// Insert with current UTC Unix timestamp.
	statement, err := s.dbHandle.Prepare("INSERT INTO visited (requestID, visited, timestamp) VALUES (?, 1, strftime('%s', 'now'))")
	if err != nil {
		return err
	}
	_, err = statement.Exec(int64(requestID))
	if err != nil {
		return err
	}
	return nil
}

// IsVisited checks if URL has already been visited.
func (s *Storage) IsVisited(requestID uint64) (bool, error) {
	var timestamp time.Time
	statement, err := s.dbHandle.Prepare("SELECT timestamp FROM visited where requestId = ?")
	if err != nil {
		return false, err
	}
	row := statement.QueryRow(int64(requestID))
	err = row.Scan(&timestamp)
	if err != nil {
		// If ErrNoRows then it means URL is new, so need to call Visited.
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	// Check if URL is within validity threshold.
	now := time.Now().UTC()
	if !timestamp.IsZero() && now.Sub(timestamp) <= s.Validity {
		return true, nil
	}
	return false, nil
}

// DeleteRequest deletes a request from cache database.
func (s *Storage) DeleteRequest(requestID uint64) error {
	statement, err := s.dbHandle.Prepare("DELETE FROM visited where requestId = ?")
	if err != nil {
		return err
	}
	_, err = statement.Exec(int64(requestID))
	if err != nil {
		return err
	}
	return nil
}

// Cookie methods are not implemented, as saving cookies is not a requirement for link caching.
func (s *Storage) SetCookies(u *url.URL, cookies string) {}

func (s *Storage) Cookies(u *url.URL) string {
	return ""
}
