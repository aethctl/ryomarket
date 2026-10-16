package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Ryoku-dev/ryomarket/internal/metadata"
	"github.com/Ryoku-dev/ryomarket/internal/model"
)

const appImageCacheVersion = 1

var githubRepoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type appImageCache struct {
	Version   int                 `json:"version"`
	FetchedAt time.Time           `json:"fetched_at"`
	Apps      []model.Application `json:"apps"`
}

func AppImageApplications(items []metadata.AppImageItem) []model.Application {
	apps := make([]model.Application, 0, len(items))
	for _, item := range items {
		repo := appImageGitHubRepo(item.Links)
		if repo == "" {
			continue
		}
		id := portableCatalogID("appimage", repo)
		variant := model.Variant{
			Kind:         model.ProviderAppImage,
			Package:      id,
			Label:        "AppImageHub",
			ReleaseRepo:  repo,
			AssetPattern: `(?i)\.appimage$`,
			Official:     false,
			Verified:     false,
			Notes:        "Community AppImage catalogue entry. Ryomarket resolves the latest matching GitHub release asset.",
		}
		if strings.TrimSpace(item.GlibcRequired) != "" || (strings.EqualFold(strings.TrimSpace(item.Libc), "host") && !item.SelfContained) {
			variant.Libcs = []string{"glibc"}
		}
		variants := make(map[model.Distro][]model.Variant, len(model.SupportedDistros()))
		for _, distro := range model.SupportedDistros() {
			variants[distro] = []model.Variant{variant}
		}
		developer := ""
		if len(item.Authors) > 0 {
			developer = strings.TrimSpace(string(item.Authors[0].Name))
		}
		apps = append(apps, model.Application{
			ID:          "appimage:" + id,
			Name:        item.Name,
			Summary:     summaryFromDescription(item.Description),
			Description: item.Description,
			Developer:   developer,
			Homepage:    "https://github.com/" + repo,
			Icon:        appImageMediaURL(firstString(item.Icons)),
			Screenshots: appImageMediaURLs(item.Screenshots),
			Categories:  normalizePortableCategories(item.Categories, item.Name+" "+item.Description),
			Keywords:    []string{"appimage", "portable"},
			Variants:    variants,
		})
	}
	return apps
}

func LoadAppImageCache() ([]model.Application, bool) {
	data, err := os.ReadFile(appImageCachePath())
	if err != nil {
		return nil, false
	}
	var cache appImageCache
	if json.Unmarshal(data, &cache) != nil || cache.Version != appImageCacheVersion || len(cache.Apps) == 0 {
		return nil, false
	}
	return cache.Apps, true
}

func SaveAppImageCache(apps []model.Application) error {
	path := appImageCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(appImageCache{Version: appImageCacheVersion, FetchedAt: time.Now(), Apps: apps})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func appImageCachePath() string {
	return filepath.Join(cacheRoot(), "catalog", "appimage.json")
}

func appImageGitHubRepo(links []metadata.AppImageLink) string {
	for _, link := range links {
		if !strings.EqualFold(strings.TrimSpace(link.Type), "GitHub") {
			continue
		}
		value := strings.TrimSpace(link.URL)
		value = strings.TrimPrefix(value, "https://github.com/")
		value = strings.TrimSuffix(value, "/")
		if githubRepoPattern.MatchString(value) {
			return value
		}
	}
	return ""
}

func portableCatalogID(prefix, identity string) string {
	clean := strings.ToLower(identity)
	clean = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(clean, "-")
	clean = strings.Trim(clean, "-")
	if len(clean) > 42 {
		clean = clean[:42]
	}
	sum := sha256.Sum256([]byte(identity))
	return prefix + "-" + clean + "-" + hex.EncodeToString(sum[:4])
}

func appImageMediaURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		return value
	}
	return "https://raw.githubusercontent.com/AppImage/appimage.github.io/master/database/" + strings.TrimPrefix(value, "/")
}

func appImageMediaURLs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if url := appImageMediaURL(value); url != "" {
			out = append(out, url)
		}
	}
	return out
}

func normalizePortableCategories(values []any, text string) []string {
	seen := map[string]bool{}
	for _, raw := range values {
		value, ok := raw.(string)
		if !ok {
			continue
		}
		for _, category := range categoriesFromRepoText(value + " " + text) {
			seen[category] = true
		}
	}
	if len(seen) == 0 {
		for _, category := range categoriesFromRepoText(text) {
			seen[category] = true
		}
	}
	if len(seen) == 0 {
		seen["Utilities"] = true
	}
	return orderedCategories(seen)
}

func summaryFromDescription(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if len(value) <= 150 {
		return value
	}
	return strings.TrimSpace(value[:147]) + "…"
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
