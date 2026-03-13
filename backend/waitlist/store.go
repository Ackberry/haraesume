package waitlist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusInvited  Status = "invited"
	StatusApproved Status = "approved"
)

type Entry struct {
	Email     string    `json:"email"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct {
	mu       sync.RWMutex
	entries  map[string]*Entry
	filePath string
}

func NewStore(filePath string) (*Store, error) {
	store := &Store{
		entries:  make(map[string]*Entry),
		filePath: filePath,
	}
	if err := store.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to load waitlist: %w", err)
	}
	return store, nil
}

func (ws *Store) Len() int {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return len(ws.entries)
}

func (ws *Store) load() error {
	data, err := os.ReadFile(ws.filePath)
	if err != nil {
		return err
	}
	var entries map[string]*Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("invalid waitlist data: %w", err)
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.entries = entries
	return nil
}

func (ws *Store) persist() error {
	ws.mu.RLock()
	data, err := json.MarshalIndent(ws.entries, "", "  ")
	ws.mu.RUnlock()
	if err != nil {
		return err
	}
	dir := filepath.Dir(ws.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(ws.filePath, data, 0o600)
}

func (ws *Store) GetEntry(userID string) (*Entry, bool) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	entry, ok := ws.entries[userID]
	if !ok {
		return nil, false
	}
	copied := *entry
	return &copied, true
}

func (ws *Store) RegisterOrGet(userID, email string) (*Entry, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if entry, ok := ws.entries[userID]; ok {
		copied := *entry
		return &copied, nil
	}

	now := time.Now().UTC()
	entry := &Entry{
		Email:     email,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	ws.entries[userID] = entry

	data, err := json.MarshalIndent(ws.entries, "", "  ")
	if err != nil {
		delete(ws.entries, userID)
		return nil, err
	}
	dir := filepath.Dir(ws.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		delete(ws.entries, userID)
		return nil, err
	}
	if err := os.WriteFile(ws.filePath, data, 0o600); err != nil {
		delete(ws.entries, userID)
		return nil, err
	}

	copied := *entry
	return &copied, nil
}

func (ws *Store) UpdateStatus(userID string, newStatus Status) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	entry, ok := ws.entries[userID]
	if !ok {
		return fmt.Errorf("user %s not found in waitlist", userID)
	}

	entry.Status = newStatus
	entry.UpdatedAt = time.Now().UTC()

	data, err := json.MarshalIndent(ws.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ws.filePath, data, 0o600)
}

func (ws *Store) ListAll() map[string]*Entry {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	result := make(map[string]*Entry, len(ws.entries))
	for k, v := range ws.entries {
		copied := *v
		result[k] = &copied
	}
	return result
}
