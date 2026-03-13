package resume

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type Storage struct {
	dir string
}

func NewStorage(dir string) *Storage {
	return &Storage{dir: dir}
}

func (st *Storage) Dir() string {
	return st.dir
}

func (st *Storage) UserResumeFilePath(userID string) string {
	return filepath.Join(st.dir, UserStorageDirName(userID), "base_resume.tex")
}

func UserStorageDirName(userID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID)))
	return fmt.Sprintf("user_%x", sum)
}

func (st *Storage) LoadPersistedBaseResume(state *State, userID string) error {
	data, err := os.ReadFile(st.UserResumeFilePath(userID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !utf8.Valid(data) {
		return errors.New("persisted resume is not valid UTF-8")
	}
	content := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
	if content == "" {
		return nil
	}
	state.SetBaseResume(userID, content)
	log.Printf("Loaded persisted base resume for user %s (%d chars)", UserStorageDirName(userID), len(content))
	return nil
}

func (st *Storage) EnsureBaseResumeLoaded(state *State, userID string) error {
	if _, ok := state.GetBaseResume(userID); ok {
		return nil
	}
	return st.LoadPersistedBaseResume(state, userID)
}

func (st *Storage) PersistBaseResume(userID, content string) error {
	path := st.UserResumeFilePath(userID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}
