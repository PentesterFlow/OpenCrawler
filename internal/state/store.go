package state

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketState = []byte("state")
	keyState    = []byte("crawler_state")
)

// BoltStore implements Store using BoltDB.
type BoltStore struct {
	db   *bolt.DB
	path string
}

// NewBoltStore creates a new BoltDB-backed state store.
func NewBoltStore(path string) (*BoltStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := bolt.Open(path, 0600, &bolt.Options{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create bucket
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketState)
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	return &BoltStore{db: db, path: path}, nil
}

// Save saves the crawler state.
func (s *BoltStore) Save(state *CrawlerState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketState)
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.Put(keyState, data)
	})
}

// Load loads the crawler state.
func (s *BoltStore) Load() (*CrawlerState, error) {
	var state CrawlerState
	var found bool

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketState)
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		data := b.Get(keyState)
		if data == nil {
			return nil // Not found, but not an error
		}

		found = true
		return json.Unmarshal(data, &state)
	})
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, nil
	}

	return &state, nil
}

// Close closes the database.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// FileStore implements Store using JSON files.
type FileStore struct {
	path       string
	compressed bool
}

// NewFileStore creates a new file-based state store.
func NewFileStore(path string, compressed bool) *FileStore {
	return &FileStore{
		path:       path,
		compressed: compressed,
	}
}

// Save saves the crawler state to a file.
func (s *FileStore) Save(state *CrawlerState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if s.compressed {
		return s.saveCompressed(data)
	}

	return os.WriteFile(s.path, data, 0644)
}

// saveCompressed saves the state with gzip compression.
func (s *FileStore) saveCompressed(data []byte) error {
	file, err := os.Create(s.path + ".gz")
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	_, err = gw.Write(data)
	return err
}

// Load loads the crawler state from a file.
func (s *FileStore) Load() (*CrawlerState, error) {
	var data []byte
	var err error

	if s.compressed {
		data, err = s.loadCompressed()
	} else {
		data, err = os.ReadFile(s.path)
	}

	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var state CrawlerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// loadCompressed loads gzip-compressed state.
func (s *FileStore) loadCompressed() ([]byte, error) {
	file, err := os.Open(s.path + ".gz")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	var data []byte
	buf := make([]byte, 4096)
	for {
		n, err := gr.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	return data, nil
}

// Close is a no-op for FileStore.
func (s *FileStore) Close() error {
	return nil
}

// MemoryStore implements Store using in-memory storage.
type MemoryStore struct {
	state *CrawlerState
}

// NewMemoryStore creates a new in-memory state store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Save saves the state in memory.
func (s *MemoryStore) Save(state *CrawlerState) error {
	s.state = state
	return nil
}

// Load returns the stored state.
func (s *MemoryStore) Load() (*CrawlerState, error) {
	return s.state, nil
}

// Close is a no-op for MemoryStore.
func (s *MemoryStore) Close() error {
	return nil
}
