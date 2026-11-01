package metadata

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestAppMetadataCacheExpiresAndPreservesDescription(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	id := "org.example.App"
	app := FlathubApp{ID: id, Description: "A complete, useful description.", Name: "Example"}
	if err := SaveAppCache(id, app); err != nil {
		t.Fatal(err)
	}
	cached, ok := LoadAppCache(id)
	if !ok || cached.Description != app.Description {
		t.Fatal("description cache miss")
	}
	data, _ := json.Marshal(appCache{ID: id, FetchedAt: time.Now().Add(-25 * time.Hour), App: app})
	if err := os.WriteFile(appCachePath(id), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadAppCache(id); ok {
		t.Fatal("stale metadata accepted")
	}
	if _, ok := LoadAppCache("another-id"); ok {
		t.Fatal("cache crossed application identity")
	}
}
