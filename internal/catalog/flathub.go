package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ryoku-dev/ryomarket/internal/metadata"
	"github.com/Ryoku-dev/ryomarket/internal/model"
)

const flathubCacheVersion = 2

type flathubCache struct {
	Version   int                 `json:"version"`
	FetchedAt time.Time           `json:"fetched_at"`
	Total     int                 `json:"total"`
	Apps      []model.Application `json:"apps"`
}

func FlathubApplications(items []metadata.FlathubSearchApp) []model.Application {
	apps := make([]model.Application, 0, len(items))
	for _, item := range items {
		appID := strings.TrimSpace(item.AppID)
		if appID == "" || strings.TrimSpace(item.Name) == "" {
			continue
		}
		variants := make(map[model.Distro][]model.Variant, len(model.SupportedDistros()))
		for _, distro := range model.SupportedDistros() {
			variants[distro] = []model.Variant{{
				Kind:        model.ProviderFlatpak,
				Package:     appID,
				Label:       "Flathub",
				Recommended: true,
				Official:    item.Verified,
				Sandboxed:   true,
			}}
		}
		apps = append(apps, model.Application{
			ID:          "flathub:" + appID,
			Name:        strings.TrimSpace(item.Name),
			Summary:     strings.TrimSpace(item.Summary),
			Description: strings.TrimSpace(item.Description),
			Developer:   strings.TrimSpace(item.DeveloperName),
			Icon:        strings.TrimSpace(item.Icon),
			Categories:  normalizeFlathubCategories(item),
			Keywords:    item.Keywords,
			Variants:    variants,
		})
	}
	return apps
}

func LoadFlathubCache() ([]model.Application, int, bool) {
	data, err := os.ReadFile(flathubCachePath())
	if err != nil {
		return nil, 0, false
	}
	var cache flathubCache
	if json.Unmarshal(data, &cache) != nil || cache.Version != flathubCacheVersion || len(cache.Apps) == 0 {
		return nil, 0, false
	}
	return cache.Apps, cache.Total, true
}

func SaveFlathubCache(apps []model.Application, total int) error {
	path := flathubCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(flathubCache{Version: flathubCacheVersion, FetchedAt: time.Now(), Total: total, Apps: apps})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func flathubCachePath() string {
	home, _ := os.UserHomeDir()
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cacheHome = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheHome, "ryomarket", "catalog", "flathub.json")
}

func normalizeFlathubCategories(item metadata.FlathubSearchApp) []string {
	seen := map[string]bool{}
	add := func(values ...string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" {
				seen[value] = true
			}
		}
	}

	for _, value := range item.Categories {
		switch categoryKey(value) {
		case "audiovideo":
			add("Media")
		case "development":
			add("Development")
		case "education":
			add("Productivity")
		case "healthfitness":
			add("Utilities")
		case "game":
			add("Gaming")
		case "graphics":
			add("Graphics", "Creator")
		case "network":
			add("Internet")
		case "office":
			add("Office", "Productivity")
		case "science":
			add("Productivity")
		case "system":
			add("System", "Utilities")
		case "utility":
			add("Utilities")
		}
	}

	for _, value := range item.SubCategories {
		switch categoryKey(value) {
		case "audio":
			add("Audio", "Audio & Music", "Media")
		case "music", "midi", "mixer", "sequencer", "tuner":
			add("Audio", "Audio & Music", "Media")
		case "video", "tv":
			add("Video", "Media")
		case "audiovideoediting", "recorder", "discburning", "player":
			add("Media")
		case "building", "debugger", "ide", "guidesigner", "profiling", "revisioncontrol", "translation", "webdevelopment":
			add("Development")
		case "2dgraphics", "vectorgraphics", "rastergraphics", "3dgraphics", "scanning", "ocr", "photography", "publishing":
			add("Graphics", "Creator")
		case "email", "telephony", "telephonytools", "videoconference":
			add("Communication", "Internet")
		case "instantmessaging", "chat", "ircclient":
			add("Communication", "Social", "Internet")
		case "feed", "filetransfer", "hamradio", "news", "p2p", "remoteaccess", "webbrowser", "dialup":
			add("Internet")
		case "calendar", "contactmanagement", "database", "dictionary", "chart", "finance", "flowchart", "projectmanagement", "presentation", "spreadsheet", "wordprocessor":
			add("Office", "Productivity")
		case "calculator", "clock", "texteditor", "documentation", "accessibility":
			add("Utilities")
		case "emulator", "filemanager", "filesystem", "monitor", "security", "terminalemulator":
			add("System", "Utilities")
		}
	}

	searchText := strings.ToLower(strings.Join(append([]string{item.Name, item.Summary}, item.Keywords...), " "))
	if containsAny(searchText, "social", "fediverse", "mastodon", "matrix", "messenger", "messaging", "community") {
		add("Social", "Communication")
	}
	if containsAny(searchText, "podcast", "music player", "audio player", "radio player") {
		add("Audio", "Audio & Music", "Media")
	}
	if containsAny(searchText, "video editor", "video player", "media player", "screen recorder") {
		add("Video", "Media")
	}

	if len(seen) == 0 {
		add("Utilities")
	}

	order := []string{
		"Audio", "Audio & Music", "Communication", "Creator", "Development",
		"Gaming", "Graphics", "Internet", "Media", "Office", "Productivity",
		"Social", "System", "Utilities", "Video",
	}
	out := make([]string, 0, len(seen))
	for _, category := range order {
		if seen[category] {
			out = append(out, category)
		}
	}
	return out
}

func categoryKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(" ", "", "-", "", "_", "", "&", "and").Replace(value)
	return value
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
