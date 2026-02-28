package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

const defaultSessionsDir = ".shimibot/sessions"

var validSessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

type Store interface {
	Load(sessionID string) ([]llm.Message, error)
	Save(sessionID string, history []llm.Message) error
}

type JSONFileStore struct {
	sessionsDir string
}

type sessionData struct {
	SessionID string        `json:"session_id"`
	Messages  []llm.Message `json:"messages"`
}

func DefaultSessionID(now time.Time) string {
	return now.Format("20060102-150405")
}

func NewJSONFileStore() *JSONFileStore {
	return &JSONFileStore{sessionsDir: defaultSessionsDir}
}

func NewJSONFileStoreWithDir(sessionsDir string) *JSONFileStore {
	trimmedDir := strings.TrimSpace(sessionsDir)
	if trimmedDir == "" {
		trimmedDir = defaultSessionsDir
	}
	return &JSONFileStore{sessionsDir: trimmedDir}
}

func normalizeSessionID(sessionID string) (string, error) {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return "", nil
	}

	if !validSessionIDPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid session id: use 1-128 chars [A-Za-z0-9_-], starting with alphanumeric")
	}

	return trimmed, nil
}

func (store *JSONFileStore) sessionFilePath(sessionID string) (string, error) {
	normalizedSessionID, err := normalizeSessionID(sessionID)
	if err != nil {
		return "", err
	}
	return filepath.Join(store.sessionsDir, normalizedSessionID+".json"), nil
}

func (store *JSONFileStore) Load(sessionID string) ([]llm.Message, error) {
	normalizedSessionID, err := normalizeSessionID(sessionID)
	if err != nil {
		return nil, err
	}

	if normalizedSessionID == "" {
		return []llm.Message{}, nil
	}

	path, err := store.sessionFilePath(normalizedSessionID)
	if err != nil {
		return nil, err
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []llm.Message{}, nil
		}
		return nil, err
	}

	var stored sessionData
	if err := json.Unmarshal(bytes, &stored); err != nil {
		return nil, err
	}

	return stored.Messages, nil
}

func (store *JSONFileStore) Save(sessionID string, history []llm.Message) error {
	normalizedSessionID, err := normalizeSessionID(sessionID)
	if err != nil {
		return err
	}

	if normalizedSessionID == "" {
		return nil
	}

	payload, err := json.MarshalIndent(sessionData{SessionID: normalizedSessionID, Messages: history}, "", "  ")
	if err != nil {
		return err
	}

	path, err := store.sessionFilePath(normalizedSessionID)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	return os.WriteFile(path, payload, 0o600)
}
