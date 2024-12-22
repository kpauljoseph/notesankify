package acceptance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type TestFileHashes struct {
	Filename string              `json:"filename"`
	Pages    map[string]PageHash `json:"pages"` // key is page number as string
}

type PageHash struct {
	Hash string `json:"hash"`
}

type HashStore struct {
	path         string
	updateHashes bool
	hashes       map[string]TestFileHashes // filename -> hashes
}

func NewHashStore(testDataPath string) *HashStore {
	return &HashStore{
		path:         filepath.Join(testDataPath, "expected_hashes.json"),
		updateHashes: os.Getenv("UPDATE_TEST_DATA") == "true",
		hashes:       make(map[string]TestFileHashes),
	}
}

func (s *HashStore) Load() error {
	if s.updateHashes {
		return nil // Don't load when updating
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error if file doesn't exist in update mode
		}
		return fmt.Errorf("failed to read hash file: %w", err)
	}

	var hashList []TestFileHashes
	if err := json.Unmarshal(data, &hashList); err != nil {
		return fmt.Errorf("failed to parse hash file: %w", err)
	}

	// Convert to map for easier lookup
	for _, h := range hashList {
		s.hashes[h.Filename] = h
	}

	return nil
}

func (s *HashStore) Save() error {
	if !s.updateHashes {
		return nil // Only save when updating
	}

	// Convert map to slice for storage
	var hashList []TestFileHashes
	for _, h := range s.hashes {
		hashList = append(hashList, h)
	}

	// Sort the list for consistent output
	sort.Slice(hashList, func(i, j int) bool {
		return hashList[i].Filename < hashList[j].Filename
	})

	data, err := json.MarshalIndent(hashList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hashes: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write hash file: %w", err)
	}

	return nil
}

func (s *HashStore) UpdateFileHashes(filename string, pageHashes map[string]PageHash) {
	if !s.updateHashes {
		return
	}

	s.hashes[filename] = TestFileHashes{
		Filename: filename,
		Pages:    pageHashes,
	}
}

func (s *HashStore) GetFileHashes(filename string) (TestFileHashes, bool) {
	hashes, exists := s.hashes[filename]
	return hashes, exists
}

func (s *HashStore) IsUpdateMode() bool {
	return s.updateHashes
}

func GetPageNumbers(pages map[string]PageHash) []string {
	numbers := make([]string, 0, len(pages))
	for num := range pages {
		numbers = append(numbers, num)
	}
	sort.Strings(numbers)
	return numbers
}
