package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Ryoku-dev/ryomarket/internal/model"
	"github.com/Ryoku-dev/ryomarket/internal/provider"
)

type Transaction struct {
	AppID     string             `json:"app_id"`
	AppName   string             `json:"app_name"`
	Provider  model.ProviderKind `json:"provider"`
	Package   string             `json:"package"`
	Operation provider.Operation `json:"operation"`
	Distro    model.Distro       `json:"distro"`
	At        time.Time          `json:"at"`
	Success   bool               `json:"success"`
	Error     string             `json:"error,omitempty"`
}

type Appearance struct {
	Mode          string `json:"mode,omitempty"`
	Accent        string `json:"accent,omitempty"`
	Surface       string `json:"surface,omitempty"`
	ReducedMotion bool   `json:"reduced_motion,omitempty"`
}

type Data struct {
	WelcomeDone       bool               `json:"welcome_done,omitempty"`
	Appearance        Appearance         `json:"appearance,omitempty"`
	PreferredProvider model.ProviderKind `json:"preferred_provider,omitempty"`
	Transactions      []Transaction      `json:"transactions,omitempty"`
}

type Store struct {
	path string
	Data Data
}

func Open() (*Store, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, ".local", "state")
	}
	path := filepath.Join(base, "ryomarket", "state.json")
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &s.Data); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Record(tx Transaction) error {
	s.Data.Transactions = append([]Transaction{tx}, s.Data.Transactions...)
	if len(s.Data.Transactions) > 100 {
		s.Data.Transactions = s.Data.Transactions[:100]
	}
	return s.Save()
}
