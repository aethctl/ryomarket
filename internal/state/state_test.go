package state

import (
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"path/filepath"
	"testing"
)

func TestPreferencePersistsWithoutLosingTransactions(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	s, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	s.Data.PreferredProvider = model.ProviderNixProfile
	s.Data.Transactions = []Transaction{{AppID: "example", Success: true}}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	restored, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Data.PreferredProvider != model.ProviderNixProfile || len(restored.Data.Transactions) != 1 {
		t.Fatal("preference or history lost")
	}
}
