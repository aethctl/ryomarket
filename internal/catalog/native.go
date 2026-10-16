package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Ryoku-dev/ryomarket/internal/model"
)

const nativeCacheVersion = 3
const nixCacheVersion = 1
const nativeCatalogLimit = 10000
const nixCatalogLimit = 60000

type nativeCache struct {
	Version   int                 `json:"version"`
	FetchedAt time.Time           `json:"fetched_at"`
	Distro    model.Distro        `json:"distro"`
	Apps      []model.Application `json:"apps"`
}

type nixCache struct {
	Version   int           `json:"version"`
	FetchedAt time.Time     `json:"fetched_at"`
	Packages  []repoPackage `json:"packages"`
}

type repoPackage struct {
	Name        string             `json:"name"`
	Package     string             `json:"package"`
	Summary     string             `json:"summary,omitempty"`
	Version     string             `json:"version,omitempty"`
	Provider    model.ProviderKind `json:"provider"`
	NativeLabel string             `json:"native_label,omitempty"`
}

func NativeApplications(ctx context.Context, distro model.Distro) ([]model.Application, error) {
	var packages []repoPackage
	var err error
	switch distro {
	case model.DistroFedora:
		packages, err = queryDNF(ctx)
	case model.DistroArch, model.DistroCachyOS:
		packages, err = queryPacman(ctx)
	case model.DistroVoid:
		packages, err = queryXBPS(ctx)
	case model.DistroNixOS:
		return nil, fmt.Errorf("NixOS native applications are provided by the Nixpkgs catalogue")
	default:
		return nil, fmt.Errorf("native repository discovery is unavailable for %s", distro)
	}
	if err != nil {
		return nil, err
	}
	return repoPackagesToApps(packages, distro), nil
}

func NixAvailable() bool {
	_, err := exec.LookPath("nix")
	return err == nil
}

func NixApplications(ctx context.Context) ([]model.Application, error) {
	packages, err := queryNix(ctx)
	if err != nil {
		return nil, err
	}
	packages = rankRepoPackages(packages, nixCatalogLimit)
	if err := saveNixPackages(packages); err != nil {
		return nil, err
	}
	return nixPackagesToApps(packages), nil
}

func LoadNixCache() ([]model.Application, bool) {
	data, err := os.ReadFile(nixCachePath())
	if err != nil {
		return nil, false
	}
	var cache nixCache
	if json.Unmarshal(data, &cache) != nil || cache.Version != nixCacheVersion || len(cache.Packages) == 0 {
		return nil, false
	}
	return nixPackagesToApps(cache.Packages), true
}

func saveNixPackages(packages []repoPackage) error {
	path := nixCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(nixCache{Version: nixCacheVersion, FetchedAt: time.Now(), Packages: packages})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func nixCachePath() string {
	return filepath.Join(cacheRoot(), "catalog", "nixpkgs.json")
}

func LoadLegacyNixOSCache() ([]model.Application, bool) {
	data, err := os.ReadFile(nativeCachePath(model.DistroNixOS))
	if err != nil {
		return nil, false
	}
	var cache nativeCache
	if json.Unmarshal(data, &cache) != nil || cache.Distro != model.DistroNixOS || len(cache.Apps) == 0 {
		return nil, false
	}
	return cache.Apps, true
}

func LoadNativeCache(distro model.Distro) ([]model.Application, bool) {
	data, err := os.ReadFile(nativeCachePath(distro))
	if err != nil {
		return nil, false
	}
	var cache nativeCache
	if json.Unmarshal(data, &cache) != nil || cache.Version != nativeCacheVersion || cache.Distro != distro || len(cache.Apps) == 0 {
		return nil, false
	}
	return cache.Apps, true
}

func SaveNativeCache(distro model.Distro, apps []model.Application) error {
	path := nativeCachePath(distro)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(nativeCache{Version: nativeCacheVersion, FetchedAt: time.Now(), Distro: distro, Apps: apps})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func nativeCachePath(distro model.Distro) string {
	return filepath.Join(cacheRoot(), "catalog", "native-"+string(distro)+".json")
}

func cacheRoot() string {
	if value := os.Getenv("XDG_CACHE_HOME"); value != "" {
		return filepath.Join(value, "ryomarket")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "ryomarket")
}

func queryNix(ctx context.Context) ([]repoPackage, error) {
	cmd := exec.CommandContext(ctx, "nix", "--extra-experimental-features", "nix-command flakes", "search", "nixpkgs", "^", "--json", "--no-pretty")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("query nixpkgs: %w: %s", err, detail)
		}
		return nil, fmt.Errorf("query nixpkgs: %w", err)
	}
	return parseNixSearch(stdout.Bytes())
}

func parseNixSearch(out []byte) ([]repoPackage, error) {
	var values map[string]struct {
		PName       string `json:"pname"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(out, &values); err != nil {
		return nil, fmt.Errorf("decode nix search: %w", err)
	}
	packages := make([]repoPackage, 0, len(values))
	seen := map[string]bool{}
	for attr, value := range values {
		packageAttr := nixAttribute(attr)
		if packageAttr == "" || seen[packageAttr] {
			continue
		}
		seen[packageAttr] = true
		name := strings.TrimSpace(value.PName)
		if name == "" {
			name = lastAttribute(packageAttr)
		}
		packages = append(packages, repoPackage{
			Name: name, Package: packageAttr, Summary: strings.TrimSpace(value.Description), Version: value.Version,
			Provider: model.ProviderNixProfile, NativeLabel: "Nixpkgs",
		})
	}
	return packages, nil
}

func queryDNF(ctx context.Context) ([]repoPackage, error) {
	out, err := exec.CommandContext(ctx, "dnf", "repoquery", "--available", "--qf", "%{name}\t%{version}\t%{summary}").Output()
	if err != nil {
		return nil, fmt.Errorf("query Fedora repositories: %w", err)
	}
	return parseTabbedRepo(string(out), model.ProviderDNF, "Fedora repositories"), nil
}

func queryPacman(ctx context.Context) ([]repoPackage, error) {
	out, err := exec.CommandContext(ctx, "pacman", "-Ss", ".").Output()
	if err != nil {
		return nil, fmt.Errorf("query pacman repositories: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	packages := make([]repoPackage, 0, len(lines)/2)
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.Contains(line, "/") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		if slash := strings.IndexByte(name, '/'); slash >= 0 {
			name = name[slash+1:]
		}
		summary := ""
		if i+1 < len(lines) && strings.HasPrefix(lines[i+1], "    ") {
			summary = strings.TrimSpace(lines[i+1])
			i++
		}
		packages = append(packages, repoPackage{Name: name, Package: name, Summary: summary, Version: fields[1], Provider: model.ProviderPacman, NativeLabel: "Native repository"})
	}
	return packages, nil
}

func queryXBPS(ctx context.Context) ([]repoPackage, error) {
	out, err := exec.CommandContext(ctx, "xbps-query", "-Rs", ".").Output()
	if err != nil {
		return nil, fmt.Errorf("query Void repositories: %w", err)
	}
	packages := make([]repoPackage, 0, 4000)
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pkgverIndex := 0
		if strings.HasPrefix(fields[0], "[") {
			pkgverIndex = 1
		}
		if pkgverIndex >= len(fields) {
			continue
		}
		pkgver := fields[pkgverIndex]
		name, version := splitPackageVersion(pkgver)
		if name == "" {
			continue
		}
		summaryStart := pkgverIndex + 1
		packages = append(packages, repoPackage{Name: name, Package: name, Summary: strings.Join(fields[summaryStart:], " "), Version: version, Provider: model.ProviderXBPS, NativeLabel: "Void repository"})
	}
	return packages, nil
}

func parseTabbedRepo(data string, provider model.ProviderKind, label string) []repoPackage {
	seen := map[string]bool{}
	packages := make([]repoPackage, 0, 4000)
	for _, line := range strings.Split(data, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		summary := ""
		if len(parts) == 3 {
			summary = strings.TrimSpace(parts[2])
		}
		packages = append(packages, repoPackage{Name: name, Package: name, Version: strings.TrimSpace(parts[1]), Summary: summary, Provider: provider, NativeLabel: label})
	}
	return packages
}

func repoPackagesToApps(packages []repoPackage, distro model.Distro) []model.Application {
	ranked := rankRepoPackages(packages, nativeCatalogLimit)
	apps := make([]model.Application, 0, len(ranked))
	for _, pkg := range ranked {
		name := titlePackageName(pkg.Name)
		variant := model.Variant{
			Kind: pkg.Provider, Package: pkg.Package, Label: pkg.NativeLabel,
			Version: pkg.Version, Official: true,
		}
		apps = append(apps, model.Application{
			ID:          "native:" + string(distro) + ":" + pkg.Package,
			Name:        name,
			Summary:     pkg.Summary,
			Description: pkg.Summary,
			Categories:  categoriesFromRepoText(pkg.Name + " " + pkg.Summary),
			Keywords:    []string{pkg.Name, "native", "repository"},
			Variants:    map[model.Distro][]model.Variant{distro: {variant}},
		})
	}
	return apps
}

func nixPackagesToApps(packages []repoPackage) []model.Application {
	ranked := rankRepoPackages(packages, nixCatalogLimit)
	apps := make([]model.Application, 0, len(ranked))
	for _, pkg := range ranked {
		variants := make(map[model.Distro][]model.Variant, len(model.SupportedDistros()))
		for _, distro := range model.SupportedDistros() {
			profile := model.Variant{
				Kind: model.ProviderNixProfile, Package: pkg.Package, ProfileName: pkg.Name, Label: "Nixpkgs",
				Version: pkg.Version, Official: true,
				Notes: "Installed through the Nix package manager.",
			}
			if distro == model.DistroNixOS {
				profile.Label = "Nix profile"
				variants[distro] = []model.Variant{
					{
						Kind: model.ProviderNixConfig, Package: pkg.Package, Label: "NixOS configuration",
						Version: pkg.Version, Official: true, Recommended: true,
						Notes: "Managed declaratively by Ryomarket when NixOS integration is enabled.",
					},
					profile,
				}
				continue
			}
			variants[distro] = []model.Variant{profile}
		}
		apps = append(apps, model.Application{
			ID:          "nixpkgs:" + pkg.Package,
			Name:        titlePackageName(pkg.Name),
			Summary:     pkg.Summary,
			Description: pkg.Summary,
			Categories:  categoriesFromRepoText(pkg.Name + " " + pkg.Summary),
			Keywords:    []string{pkg.Name, pkg.Package, "nix", "nixpkgs"},
			Variants:    variants,
		})
	}
	return apps
}

func rankRepoPackages(packages []repoPackage, limit int) []repoPackage {
	type ranked struct {
		pkg   repoPackage
		score int
	}
	rankedPackages := make([]ranked, 0, len(packages))
	seen := map[string]bool{}
	for _, pkg := range packages {
		pkg.Name = strings.TrimSpace(pkg.Name)
		pkg.Package = strings.TrimSpace(pkg.Package)
		if pkg.Name == "" || pkg.Package == "" || seen[pkg.Package] {
			continue
		}
		seen[pkg.Package] = true
		score := repoAppScore(pkg.Package, pkg.Name, pkg.Summary)
		if score <= 0 {
			continue
		}
		rankedPackages = append(rankedPackages, ranked{pkg: pkg, score: score})
	}
	sort.SliceStable(rankedPackages, func(i, j int) bool {
		if rankedPackages[i].score != rankedPackages[j].score {
			return rankedPackages[i].score > rankedPackages[j].score
		}
		return strings.ToLower(rankedPackages[i].pkg.Name) < strings.ToLower(rankedPackages[j].pkg.Name)
	})
	if limit > 0 && len(rankedPackages) > limit {
		rankedPackages = rankedPackages[:limit]
	}
	out := make([]repoPackage, len(rankedPackages))
	for i := range rankedPackages {
		out[i] = rankedPackages[i].pkg
	}
	return out
}

var repoNoise = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(-devel|-dev|-dbg|-debug|-headers|-static|-libs?)$`),
	regexp.MustCompile(`(?i)(-docs?|-examples?|-tests?|-common|-data|-locale|-lang|-translations?)$`),
	regexp.MustCompile(`(?i)^(python\d*|python3-|perl-|ruby-|node-|golang-|go-|rust-|haskell-|ocaml-|texlive-|font[s-])`),
	regexp.MustCompile(`(?i)^(kernel|linux-firmware|firmware|gcc|glibc|llvm|clang|qt[0-9]?|gtk[0-9]?|kde-frameworks)`),
	regexp.MustCompile(`(?i)^(python\d*Packages|pythonPackages|perlPackages|rubyPackages|nodePackages|haskellPackages|vimPlugins|emacsPackages|rPackages|ocamlPackages|luaPackages|beamPackages)\.`),
	regexp.MustCompile(`(?i)\.(dev|doc|lib|static|headers)$`),
}

func repoAppScore(packageID, name, summary string) int {
	name = strings.TrimSpace(name)
	packageID = strings.TrimSpace(packageID)
	text := strings.ToLower(packageID + " " + name + " " + summary)
	for _, pattern := range repoNoise {
		if pattern.MatchString(name) || pattern.MatchString(packageID) {
			return -1
		}
	}
	score := 1
	for _, word := range []string{
		"application", "desktop", "gui", "editor", "browser", "player", "client", "launcher", "terminal", "emulator",
		"game", "monitor", "manager", "viewer", "recorder", "ide", "debugger", "chat", "messaging", "music", "video",
		"audio", "graphics", "image", "drawing", "office", "password", "backup", "virtual machine", "development tool", "utility", "tool",
		"server", "shell", "command line", "cli", "compiler", "database", "container", "virtualization", "network", "download",
	} {
		if strings.Contains(text, word) {
			score += 8
		}
	}
	for _, word := range []string{
		"library", "bindings", "header", "runtime library", "development files", "language pack", "firmware", "kernel module",
		"debug symbols", "source files", "shared library", "static library",
	} {
		if strings.Contains(text, word) {
			score -= 10
		}
	}
	if !strings.Contains(name, "-") && !strings.Contains(packageID, ".") {
		score += 2
	}
	return score
}

func categoriesFromRepoText(text string) []string {
	text = strings.ToLower(text)
	seen := map[string]bool{}
	add := func(values ...string) {
		for _, value := range values {
			seen[value] = true
		}
	}
	if containsAny(text, "audio", "music", "podcast", "midi", "sound") {
		add("Audio", "Audio & Music", "Media")
	}
	if containsAny(text, "video", "movie", "media player", "screen recorder", "streaming") {
		add("Video", "Media")
	}
	if containsAny(text, "chat", "messaging", "matrix", "irc", "social", "mastodon", "communication") {
		add("Communication", "Social", "Internet")
	}
	if containsAny(text, "browser", "network", "internet", "download", "torrent", "remote desktop", "ftp") {
		add("Internet")
	}
	if containsAny(text, "developer", "development", "compiler", "debugger", "ide", "code editor", "programming", "git client") {
		add("Development")
	}
	if containsAny(text, "game", "gaming", "emulator", "launcher") {
		add("Gaming")
	}
	if containsAny(text, "graphics", "image", "photo", "drawing", "paint", "vector", "3d", "cad") {
		add("Graphics", "Creator")
	}
	if containsAny(text, "office", "spreadsheet", "word processor", "presentation", "calendar", "finance", "project management") {
		add("Office", "Productivity")
	}
	if containsAny(text, "terminal", "system monitor", "file manager", "disk", "partition", "system", "hardware", "virtual machine") {
		add("System", "Utilities")
	}
	if containsAny(text, "editor", "creator", "recording", "design") {
		add("Creator")
	}
	if containsAny(text, "utility", "tool", "manager", "viewer", "calculator", "clock", "backup", "password") {
		add("Utilities")
	}
	if len(seen) == 0 {
		add("Utilities")
	}
	return orderedCategories(seen)
}

func orderedCategories(seen map[string]bool) []string {
	order := []string{"Audio", "Audio & Music", "Communication", "Creator", "Development", "Gaming", "Graphics", "Internet", "Media", "Office", "Productivity", "Social", "System", "Utilities", "Video"}
	out := make([]string, 0, len(seen))
	for _, category := range order {
		if seen[category] {
			out = append(out, category)
		}
	}
	return out
}

func titlePackageName(value string) string {
	value = strings.NewReplacer("-", " ", "_", " ").Replace(value)
	words := strings.Fields(value)
	for i := range words {
		if len(words[i]) > 0 {
			words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
		}
	}
	return strings.Join(words, " ")
}

func nixAttribute(value string) string {
	parts := strings.Split(value, ".")
	if len(parts) >= 3 && (parts[0] == "legacyPackages" || parts[0] == "packages") {
		return strings.Join(parts[2:], ".")
	}
	return value
}

func lastAttribute(value string) string {
	if index := strings.LastIndexByte(value, '.'); index >= 0 {
		return value[index+1:]
	}
	return value
}

func splitPackageVersion(value string) (string, string) {
	for i := len(value) - 1; i > 0; i-- {
		if value[i] == '-' && i+1 < len(value) && value[i+1] >= '0' && value[i+1] <= '9' {
			return value[:i], value[i+1:]
		}
	}
	return value, ""
}
