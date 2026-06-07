// Package history provides mutation testing history management.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sivchari/gomu/internal/mutation"
)

// Store manages mutation testing history for incremental analysis.
type Store struct {
	filepath string
	entries  map[string]Entry
}

// Entry represents a history entry for a file.
type Entry struct {
	FileHash      string            `json:"fileHash"`
	TestHash      string            `json:"testHash"`
	Mutants       []mutation.Mutant `json:"mutants"`
	Results       []mutation.Result `json:"results"`
	Timestamp     time.Time         `json:"timestamp"`
	MutationScore float64           `json:"mutationScore"`
}

// New creates a new history store.
func New(filepath string) (*Store, error) {
	store := &Store{
		filepath: filepath,
		entries:  make(map[string]Entry),
	}

	// Load existing history if file exists
	if err := store.load(); err != nil {
		// If file doesn't exist, that's okay - we'll create it on save
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to load history: %w", err)
		}
	}

	return store, nil
}

// load reads the history file into memory.
func (s *Store) load() error {
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to read history file: %w", err)
	}

	var historyData struct {
		Entries map[string]Entry `json:"entries"`
	}

	if err := json.Unmarshal(data, &historyData); err != nil {
		return fmt.Errorf("failed to unmarshal history data: %w", err)
	}

	s.entries = historyData.Entries
	if s.entries == nil {
		s.entries = make(map[string]Entry)
	}

	return nil
}

// Save writes the history to disk.
func (s *Store) Save() error {
	historyData := struct {
		Entries map[string]Entry `json:"entries"`
		SavedAt time.Time        `json:"savedAt"`
		Version string           `json:"version"`
	}{
		Entries: s.entries,
		SavedAt: time.Now(),
		Version: "v0.0.0",
	}

	data, err := json.MarshalIndent(historyData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history data: %w", err)
	}

	if err := os.WriteFile(s.filepath, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

// GetEntry retrieves a history entry for a file.
func (s *Store) GetEntry(filePath string) (Entry, bool) {
	entry, exists := s.entries[filePath]

	return entry, exists
}

// UpdateFile updates the history entry for a file.
func (s *Store) UpdateFile(filePath string, mutants []mutation.Mutant, results []mutation.Result) {
	s.UpdateFileWithHashes(filePath, mutants, results, "", "")
}

// UpdateFileWithHashes updates the history entry for a file with specific hashes.
func (s *Store) UpdateFileWithHashes(filePath string, mutants []mutation.Mutant, results []mutation.Result, fileHash, testHash string) {
	// Calculate mutation score
	var killed, total int
	for _, result := range results {
		total++

		if result.Status == mutation.StatusKilled {
			killed++
		}
	}

	var score float64
	if total > 0 {
		score = float64(killed) / float64(total) * 100
	}

	entry := Entry{
		FileHash:      fileHash,
		TestHash:      testHash,
		Mutants:       mutants,
		Results:       results,
		Timestamp:     time.Now(),
		MutationScore: score,
	}

	s.entries[filePath] = entry
}

// HasChanged checks if a file has changed since last analysis.
func (s *Store) HasChanged(filePath, currentHash string) bool {
	entry, exists := s.entries[filePath]
	if !exists {
		return true // New file, consider it changed
	}

	return entry.FileHash != currentHash
}

// GetStats returns overall statistics from history.
func (s *Store) GetStats() Stats {
	var totalFiles, totalMutants, totalKilled int

	var totalScore float64

	for _, entry := range s.entries {
		totalFiles++
		totalMutants += len(entry.Results)
		totalScore += entry.MutationScore

		for _, result := range entry.Results {
			if result.Status == mutation.StatusKilled {
				totalKilled++
			}
		}
	}

	var avgScore float64
	if totalFiles > 0 {
		avgScore = totalScore / float64(totalFiles)
	}

	return Stats{
		TotalFiles:   totalFiles,
		TotalMutants: totalMutants,
		TotalKilled:  totalKilled,
		AverageScore: avgScore,
		LastUpdated:  time.Now(),
	}
}

// Stats represents overall mutation testing statistics.
type Stats struct {
	TotalFiles   int       `json:"totalFiles"`
	TotalMutants int       `json:"totalMutants"`
	TotalKilled  int       `json:"totalKilled"`
	AverageScore float64   `json:"averageScore"`
	LastUpdated  time.Time `json:"lastUpdated"`
}

// NewStore creates a new history store (alias for New).
func NewStore(filepath string) (*Store, error) {
	return New(filepath)
}

// UpdateEntry updates an entry (wrapper for UpdateFileWithHashes).
func (s *Store) UpdateEntry(filePath string, entry Entry) error {
	s.entries[filePath] = entry

	return nil
}

// StoreAdapter adapts Store to analysis.HistoryStore interface.
type StoreAdapter struct {
	store *Store
}

// NewHistoryStoreAdapter creates a new adapter.
func NewHistoryStoreAdapter(store *Store) *StoreAdapter {
	return &StoreAdapter{store: store}
}

// GetEntry implements analysis.HistoryStore interface.
func (a *StoreAdapter) GetEntry(filePath string) (AnalysisHistoryEntry, bool) {
	entry, exists := a.store.GetEntry(filePath)
	if !exists {
		return AnalysisHistoryEntry{}, false
	}

	// Convert to analysis.HistoryEntry format
	analysisEntry := AnalysisHistoryEntry{
		FileHash:      entry.FileHash,
		TestHash:      entry.TestHash,
		MutationScore: entry.MutationScore,
	}

	return analysisEntry, true
}

// AnalysisHistoryEntry represents the entry format expected by analysis package.
type AnalysisHistoryEntry struct {
	FileHash      string
	TestHash      string
	MutationScore float64
}

// HasChanged implements analysis.HistoryStore interface.
func (a *StoreAdapter) HasChanged(filePath, currentHash string) bool {
	return a.store.HasChanged(filePath, currentHash)
}
