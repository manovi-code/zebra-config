// Package recent stores the last few printers in the user's config directory.
// Each entry is an IP, a friendly name, and a model. Nothing else.
package recent

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxEntries = 8

// Printer is one remembered printer.
type Printer struct {
	IP       string    `json:"ip"`
	Name     string    `json:"name"`
	Model    string    `json:"model"`
	LastUsed time.Time `json:"lastUsed"`
}

type file struct {
	Printers []Printer `json:"printers"`
}

// Store is the JSON file.
type Store struct {
	path string
	mu   sync.Mutex
}

// Open returns a store at path. An empty path uses
// UserConfigDir/zebra-config/recent.json.
func Open(path string) (*Store, error) {
	if path == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "zebra-config", "recent.json")
	}
	return &Store{path: path}, nil
}

// Path is the JSON file location.
func (s *Store) Path() string { return s.path }

// List returns printers, newest first. A missing file is an empty list.
func (s *Store) List() ([]Printer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list()
}

func (s *Store) list() ([]Printer, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, err
	}
	return f.Printers, nil
}

// Add upserts a printer and keeps the eight newest.
func (s *Store) Add(p Printer) error {
	if p.IP == "" {
		return errors.New("missing ip")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.list()
	if err != nil {
		return err
	}
	next := []Printer{p}
	for _, old := range cur {
		if old.IP == p.IP {
			continue
		}
		next = append(next, old)
		if len(next) == maxEntries {
			break
		}
	}
	return s.write(next)
}

// Remove deletes one IP. A missing IP is not an error.
func (s *Store) Remove(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.list()
	if err != nil {
		return err
	}
	next := cur[:0]
	for _, p := range cur {
		if p.IP != ip {
			next = append(next, p)
		}
	}
	return s.write(next)
}

func (s *Store) write(list []Printer) error {
	if list == nil {
		list = []Printer{}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(file{Printers: list}, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
