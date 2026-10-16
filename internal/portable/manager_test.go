package portable

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallAppImageManagesDesktopAndManifest(t *testing.T) {
	data := t.TempDir()
	state := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_STATE_HOME", state)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("appimage"))
	}))
	defer server.Close()

	m := New()
	m.Client = server.Client()
	if err := m.InstallAppImage(context.Background(), "test.app", "Test App", Source{URL: server.URL}); err != nil {
		t.Fatal(err)
	}
	appImage := filepath.Join(data, "ryomarket", "appimages", "test.app.AppImage")
	info, err := os.Stat(appImage)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("appimage is not executable")
	}
	if !m.Installed("test.app", "appimage") {
		t.Fatal("managed appimage was not detected")
	}
	if _, err := os.Stat(filepath.Join(data, "applications", "ryomarket-test.app.desktop")); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove("test.app"); err != nil {
		t.Fatal(err)
	}
	if m.Installed("test.app", "appimage") {
		t.Fatal("removed appimage still detected")
	}
}

func TestArchiveExtractionRejectsTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bad.tar.gz")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	payload := []byte("bad")
	if err := tw.WriteHeader(&tar.Header{Name: "../escape", Mode: 0o644, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = gz.Close()
	_ = file.Close()

	if err := extractTarGz(archive, t.TempDir()); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
}

func TestPortableAssetScorePrefersHostArchitecture(t *testing.T) {
	compatibleScore, compatible := portableAssetScore("Example-x86_64.AppImage")
	otherScore, other := portableAssetScore("Example.AppImage")
	if runtime.GOARCH == "amd64" {
		if !compatible || !other || compatibleScore <= otherScore {
			t.Fatalf("expected x86_64 asset to be preferred: %d %v, %d %v", compatibleScore, compatible, otherScore, other)
		}
	}
}
