// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package cache

import (
	"database/sql"
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

		if err = database.Ping(); err != nil {
			return errors.Wrap(err, "verify connection to cache database")
		}
		s.dbHandle = database
	}
	if s.ClearCache {
		if err := s.Clear(); err != nil {
			return err
		}
	}
	// Create db with index.
	statement, err := s.dbHandle.Prepare("CREATE TABLE IF NOT EXISTS visited (id INTEGER PRIMARY KEY, url TEXT, visited INT, timestamp DATETIME)")
	if err != nil {
		return err
	}
	_, err = statement.Exec()
	if err != nil {
		return err
	}
	statement, err = s.dbHandle.Prepare("CREATE INDEX IF NOT EXISTS idx_visited ON visited (url)")
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
	return s.dbHandle.Close()
}

// CacheURL inserts new URL into cache database.
func (s *Storage) CacheURL(URL string) error {
	// If particular URL is already inserted, then delete.
	// Visit method will only be called if validity expires for a URL or in case of a new URL.
	if err := s.DeleteURL(URL); err != nil {
		return err
	}

	// Insert with current UTC Unix timestamp.
	statement, err := s.dbHandle.Prepare("INSERT INTO visited (url, visited, timestamp) VALUES (?, 1, strftime('%s', 'now'))")
	if err != nil {
		return err
	}
	_, err = statement.Exec(URL)
	if err != nil {
		return err
	}
	return nil
}

// IsCached checks if URL has already been visited.
func (s *Storage) IsCached(URL string) (bool, error) {
	var timestamp time.Time
	statement, err := s.dbHandle.Prepare("SELECT timestamp FROM visited where url = ?")
	if err != nil {
		return false, err
	}
	row := statement.QueryRow(URL)
	if err = row.Scan(&timestamp); err != nil {
		// If ErrNoRows then it means URL is new, so need to call Visited.
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	// Check if URL is within validity threshold.
	return (!timestamp.IsZero() && time.Now().UTC().Sub(timestamp) <= s.Validity), nil
}

// DeleteURL deletes a URL from cache database.
func (s *Storage) DeleteURL(URL string) error {
	statement, err := s.dbHandle.Prepare("DELETE FROM visited where url = ?")
	if err != nil {
		return err
	}
	_, err = statement.Exec(URL)
	if err != nil {
		return err
	}
	return nil
}
