package portable

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

const maxPortableDownload = int64(8 << 30)
const maxArchiveExtracted = int64(16 << 30)

type Source struct {
	URL          string `json:"url,omitempty"`
	ReleaseRepo  string `json:"release_repo,omitempty"`
	AssetPattern string `json:"asset_pattern,omitempty"`
}

type Manifest struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	Source     Source    `json:"source"`
	Resolved   string    `json:"resolved_url"`
	Entrypoint string    `json:"entrypoint,omitempty"`
	Installed  time.Time `json:"installed_at"`
}

type Manager struct {
	Client *http.Client
}

func New() *Manager {
	return &Manager{Client: &http.Client{Timeout: 30 * time.Minute}}
}

func (m *Manager) InstallAppImage(ctx context.Context, id, name string, source Source) error {
	if err := validateID(id); err != nil {
		return err
	}
	url, err := m.Resolve(ctx, source)
	if err != nil {
		return err
	}
	root, err := appImageRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	target := filepath.Join(root, id+".AppImage")
	if err := m.download(ctx, url, target, 0o755); err != nil {
		return err
	}
	if err := writeDesktop(id, name, target); err != nil {
		return err
	}
	return writeManifest(Manifest{ID: id, Name: name, Kind: "appimage", Source: source, Resolved: url, Installed: time.Now()})
}

func (m *Manager) InstallArchive(ctx context.Context, id, name, entrypoint string, source Source) error {
	if err := validateID(id); err != nil {
		return err
	}
	if entrypoint == "" || filepath.IsAbs(entrypoint) || hasTraversal(entrypoint) {
		return errors.New("archive entrypoint must be a safe relative path")
	}
	url, err := m.Resolve(ctx, source)
	if err != nil {
		return err
	}
	root, err := archiveRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "ryomarket-*.tar.gz")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)
	if err := m.download(ctx, url, tmpPath, 0o600); err != nil {
		return err
	}
	dest := filepath.Join(root, id)
	stage := dest + ".new"
	_ = os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}
	if err := extractTarGz(tmpPath, stage); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	execPath := filepath.Join(stage, filepath.Clean(entrypoint))
	info, err := os.Stat(execPath)
	if err != nil || !info.Mode().IsRegular() {
		_ = os.RemoveAll(stage)
		return fmt.Errorf("entrypoint %q was not found in the archive", entrypoint)
	}
	if err := os.Chmod(execPath, info.Mode()|0o111); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	backup := dest + ".old"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(stage, dest); err != nil {
		_ = os.Rename(backup, dest)
		return err
	}
	_ = os.RemoveAll(backup)
	if err := writeDesktop(id, name, filepath.Join(dest, filepath.Clean(entrypoint))); err != nil {
		return err
	}
	return writeManifest(Manifest{ID: id, Name: name, Kind: "archive", Source: source, Resolved: url, Entrypoint: entrypoint, Installed: time.Now()})
}

func (m *Manager) Remove(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	if root, err := appImageRoot(); err == nil {
		_ = os.Remove(filepath.Join(root, id+".AppImage"))
	}
	if root, err := archiveRoot(); err == nil {
		_ = os.RemoveAll(filepath.Join(root, id))
	}
	if path, err := desktopPath(id); err == nil {
		_ = os.Remove(path)
	}
	if path, err := manifestPath(id); err == nil {
		_ = os.Remove(path)
	}
	return nil
}

func (m *Manager) Installed(id, kind string) bool {
	if validateID(id) != nil {
		return false
	}
	switch kind {
	case "appimage":
		root, err := appImageRoot()
		if err != nil {
			return false
		}
		_, err = os.Stat(filepath.Join(root, id+".AppImage"))
		return err == nil
	case "archive":
		manifest, err := readManifest(id)
		if err != nil || manifest.Entrypoint == "" {
			return false
		}
		root, err := archiveRoot()
		if err != nil {
			return false
		}
		_, err = os.Stat(filepath.Join(root, id, filepath.Clean(manifest.Entrypoint)))
		return err == nil
	default:
		return false
	}
}

func (m *Manager) UpdateAvailable(ctx context.Context, id string, source Source) (bool, error) {
	manifest, err := readManifest(id)
	if err != nil {
		return false, err
	}
	if source.ReleaseRepo == "" {
		return false, nil
	}
	resolved, err := m.Resolve(ctx, source)
	if err != nil {
		return false, err
	}
	return resolved != manifest.Resolved, nil
}

func (m *Manager) Resolve(ctx context.Context, source Source) (string, error) {
	if source.URL != "" {
		if !strings.HasPrefix(source.URL, "https://") {
			return "", errors.New("portable sources must use HTTPS")
		}
		return source.URL, nil
	}
	if source.ReleaseRepo == "" || source.AssetPattern == "" {
		return "", errors.New("portable source is missing a URL or release recipe")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(source.ReleaseRepo) {
		return "", errors.New("invalid GitHub repository")
	}
	pattern, err := regexp.Compile(source.AssetPattern)
	if err != nil {
		return "", fmt.Errorf("invalid asset pattern: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+source.ReleaseRepo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Ryomarket")
	resp, err := m.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub release lookup failed: %s", resp.Status)
	}
	var release struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&release); err != nil {
		return "", err
	}
	type candidate struct {
		name  string
		url   string
		score int
	}
	candidates := make([]candidate, 0, len(release.Assets))
	for _, asset := range release.Assets {
		if !pattern.MatchString(asset.Name) || !strings.HasPrefix(asset.URL, "https://") {
			continue
		}
		score, compatible := portableAssetScore(asset.Name)
		if compatible {
			candidates = append(candidates, candidate{name: asset.Name, url: asset.URL, score: score})
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no compatible release asset matched %q", source.AssetPattern)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].name < candidates[j].name
	})
	return candidates[0].url, nil
}

func portableAssetScore(name string) (int, bool) {
	lower := strings.ToLower(name)
	want := runtime.GOARCH
	score := 0
	switch want {
	case "amd64":
		if strings.Contains(lower, "aarch64") || strings.Contains(lower, "arm64") || strings.Contains(lower, "armv7") {
			return 0, false
		}
		if strings.Contains(lower, "x86_64") || strings.Contains(lower, "amd64") || strings.Contains(lower, "x64") {
			score += 50
		}
	case "arm64":
		if strings.Contains(lower, "x86_64") || strings.Contains(lower, "amd64") || strings.Contains(lower, "x64") || strings.Contains(lower, "i386") {
			return 0, false
		}
		if strings.Contains(lower, "aarch64") || strings.Contains(lower, "arm64") {
			score += 50
		}
	}
	if strings.Contains(lower, "appimage") {
		score += 10
	}
	if strings.Contains(lower, "zsync") || strings.Contains(lower, "sha256") || strings.Contains(lower, "checksum") {
		return 0, false
	}
	return score, true
}

func (m *Manager) download(ctx context.Context, url, target string, mode os.FileMode) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Ryomarket")
	resp, err := m.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	if resp.Request == nil || resp.Request.URL == nil || resp.Request.URL.Scheme != "https" {
		return errors.New("portable download redirected to a non-HTTPS URL")
	}
	if resp.ContentLength > maxPortableDownload {
		return fmt.Errorf("portable download exceeds %d GiB", maxPortableDownload>>30)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".ryomarket-download-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	written, err := io.Copy(file, io.LimitReader(resp.Body, maxPortableDownload+1))
	if err != nil {
		_ = file.Close()
		return err
	}
	if written > maxPortableDownload {
		_ = file.Close()
		return fmt.Errorf("portable download exceeds %d GiB", maxPortableDownload>>30)
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

func extractTarGz(archive, dest string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	root, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	var extracted int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if hdr.Name == "" || filepath.IsAbs(hdr.Name) || hasTraversal(hdr.Name) {
			return fmt.Errorf("unsafe archive path %q", hdr.Name)
		}
		target := filepath.Join(root, filepath.Clean(hdr.Name))
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe archive path %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if hdr.Size < 0 || hdr.Size > maxArchiveExtracted-extracted {
				return fmt.Errorf("archive exceeds %d GiB extracted", maxArchiveExtracted>>30)
			}
			extracted += hdr.Size
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode) & 0o755
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, io.LimitReader(tr, hdr.Size))
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("archive links are not allowed: %q", hdr.Name)
		}
	}
}

func hasTraversal(path string) bool {
	clean := filepath.Clean(path)
	return clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator))
}

func validateID(id string) error {
	if !safeID.MatchString(id) {
		return fmt.Errorf("invalid portable application id %q", id)
	}
	return nil
}

func dataHome() (string, error) {
	if value := os.Getenv("XDG_DATA_HOME"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func stateHome() (string, error) {
	if value := os.Getenv("XDG_STATE_HOME"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state"), nil
}

func appImageRoot() (string, error) {
	root, err := dataHome()
	return filepath.Join(root, "ryomarket", "appimages"), err
}

func archiveRoot() (string, error) {
	root, err := dataHome()
	return filepath.Join(root, "ryomarket", "apps"), err
}

func desktopPath(id string) (string, error) {
	root, err := dataHome()
	return filepath.Join(root, "applications", "ryomarket-"+id+".desktop"), err
}

func manifestPath(id string) (string, error) {
	root, err := stateHome()
	return filepath.Join(root, "ryomarket", "portable", id+".json"), err
}

func writeDesktop(id, name, executable string) error {
	path, err := desktopPath(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	name = strings.NewReplacer("\n", " ", "\r", " ").Replace(name)
	executable = strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`).Replace(executable)
	content := "[Desktop Entry]\nType=Application\nName=" + name + "\nExec=\"" + executable + "\"\nIcon=application-x-executable\nTerminal=false\nCategories=Utility;\nX-Ryomarket-Managed=true\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeManifest(manifest Manifest) error {
	path, err := manifestPath(manifest.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readManifest(id string) (Manifest, error) {
	path, err := manifestPath(id)
	if err != nil {
		return Manifest{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func ArchMatches() bool {
	return runtime.GOOS == "linux" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64")
}

func ListManifests() ([]Manifest, error) {
	root, err := stateHome()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, "ryomarket", "portable")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Manifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var manifest Manifest
		if json.Unmarshal(data, &manifest) != nil || validateID(manifest.ID) != nil {
			continue
		}
		out = append(out, manifest)
	}
	return out, nil
}
