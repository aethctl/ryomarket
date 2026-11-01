package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type appCache struct {
	ID        string
	FetchedAt time.Time
	App       FlathubApp
}

func appCachePath(id string) string {
	root, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(root, "ryomarket", "appstream", hex.EncodeToString(sum[:])+".json")
}

func LoadAppCache(id string) (FlathubApp, bool) {
	path := appCachePath(id)
	if path == "" {
		return FlathubApp{}, false
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > 4<<20 {
		return FlathubApp{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return FlathubApp{}, false
	}
	var entry appCache
	if json.Unmarshal(data, &entry) != nil || entry.ID != id || entry.FetchedAt.IsZero() || time.Since(entry.FetchedAt) > 24*time.Hour || entry.FetchedAt.After(time.Now().Add(time.Minute)) {
		return FlathubApp{}, false
	}
	return entry.App, true
}

func SaveAppCache(id string, app FlathubApp) error {
	path := appCachePath(id)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(appCache{ID: id, FetchedAt: time.Now(), App: app})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "appstream-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
