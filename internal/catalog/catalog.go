package catalog

import (
	"container/heap"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Ryoku-dev/ryomarket/internal/model"
)

//go:embed seed.json
var seed []byte

type Catalog struct {
	mu             sync.RWMutex
	apps           []model.Application
	searchText     []string
	searchName     []string
	providerMasks  []uint16
	byID           map[string]int
	byProvider     map[string]int
	byName         map[string]map[int]struct{}
	byPortableRepo map[string]map[int]struct{}
	curated        map[string]bool
}

func Load() (*Catalog, error) {
	var apps []model.Application
	if err := json.Unmarshal(seed, &apps); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	c := &Catalog{
		apps: apps, searchText: make([]string, len(apps)), searchName: make([]string, len(apps)), providerMasks: make([]uint16, len(apps)),
		byID: make(map[string]int, len(apps)), byProvider: map[string]int{},
		byName: map[string]map[int]struct{}{}, byPortableRepo: map[string]map[int]struct{}{}, curated: map[string]bool{},
	}
	for i := range apps {
		if apps[i].ID == "" || apps[i].Name == "" {
			return nil, fmt.Errorf("catalog item %d is missing identity", i)
		}
		if _, exists := c.byID[apps[i].ID]; exists {
			return nil, fmt.Errorf("duplicate app id %q", apps[i].ID)
		}
		if err := validateApp(apps[i]); err != nil {
			return nil, fmt.Errorf("catalog app %q: %w", apps[i].ID, err)
		}
		c.byID[apps[i].ID] = i
		c.curated[apps[i].ID] = true
		c.searchText[i] = apps[i].SearchText()
		c.searchName[i] = strings.ToLower(apps[i].Name)
		c.providerMasks[i] = applicationProviderMask(apps[i])
		c.indexApplication(i, apps[i])
	}
	return c, nil
}

func validateApp(app model.Application) error {
	validDistros := map[model.Distro]bool{}
	for _, distro := range model.SupportedDistros() {
		validDistros[distro] = true
	}
	validProviders := map[model.ProviderKind]bool{
		model.ProviderDNF: true, model.ProviderPacman: true, model.ProviderXBPS: true,
		model.ProviderNixProfile: true, model.ProviderNixConfig: true, model.ProviderFlatpak: true,
		model.ProviderAppImage: true, model.ProviderUpstream: true,
	}
	allowedProviders := map[model.Distro]map[model.ProviderKind]bool{
		model.DistroFedora:  {model.ProviderDNF: true, model.ProviderNixProfile: true, model.ProviderFlatpak: true, model.ProviderAppImage: true, model.ProviderUpstream: true},
		model.DistroArch:    {model.ProviderPacman: true, model.ProviderNixProfile: true, model.ProviderFlatpak: true, model.ProviderAppImage: true, model.ProviderUpstream: true},
		model.DistroCachyOS: {model.ProviderPacman: true, model.ProviderNixProfile: true, model.ProviderFlatpak: true, model.ProviderAppImage: true, model.ProviderUpstream: true},
		model.DistroVoid:    {model.ProviderXBPS: true, model.ProviderNixProfile: true, model.ProviderFlatpak: true, model.ProviderAppImage: true, model.ProviderUpstream: true},
		model.DistroNixOS:   {model.ProviderNixProfile: true, model.ProviderNixConfig: true, model.ProviderFlatpak: true, model.ProviderAppImage: true, model.ProviderUpstream: true},
	}
	for distro, variants := range app.Variants {
		if !validDistros[distro] {
			return fmt.Errorf("unsupported distro key %q", distro)
		}
		recommended := 0
		seen := map[string]bool{}
		for _, variant := range variants {
			if !validProviders[variant.Kind] {
				return fmt.Errorf("unknown provider %q", variant.Kind)
			}
			if !allowedProviders[distro][variant.Kind] {
				return fmt.Errorf("provider %s is not valid for %s", variant.Kind, distro)
			}
			if variant.Kind == model.ProviderAppImage || variant.Kind == model.ProviderUpstream {
				if variant.SourceURL == "" && (variant.ReleaseRepo == "" || variant.AssetPattern == "") {
					return fmt.Errorf("%s provider is missing a portable source", variant.Kind)
				}
				if variant.Kind == model.ProviderUpstream && strings.TrimSpace(variant.Entrypoint) == "" {
					return fmt.Errorf("upstream provider is missing an entrypoint")
				}
			}
			if strings.TrimSpace(variant.Package) == "" {
				return fmt.Errorf("%s provider is missing a package id", variant.Kind)
			}
			key := string(variant.Kind) + "\x00" + variant.Package
			if seen[key] {
				return fmt.Errorf("duplicate %s provider for %q on %s", variant.Kind, variant.Package, distro)
			}
			seen[key] = true
			if variant.Recommended {
				recommended++
			}
		}
		if recommended > 1 {
			return fmt.Errorf("multiple recommended providers for %s", distro)
		}
	}
	return nil
}

func (c *Catalog) All() []model.Application {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := append([]model.Application(nil), c.apps...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (c *Catalog) App(id string) (model.Application, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	i, ok := c.byID[id]
	if !ok {
		return model.Application{}, false
	}
	return c.apps[i], true
}

func (c *Catalog) Merge(apps []model.Application) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureIndexes()
	added := 0
	for _, app := range apps {
		if app.ID == "" || app.Name == "" {
			continue
		}
		if index, ok := c.matchProvider(app); ok {
			c.setApplication(index, mergeApplication(c.apps[index], app))
			continue
		}
		if strings.HasPrefix(app.ID, "native:") || strings.HasPrefix(app.ID, "nixpkgs:") {
			if index, ok := c.matchName(app.Name); ok {
				c.setApplication(index, mergeProviderApplication(c.apps[index], app))
				continue
			}
		}
		if strings.HasPrefix(app.ID, "appimage:") {
			if index, ok := c.matchPortableRepo(app); ok {
				c.setApplication(index, mergeApplication(c.apps[index], app))
				continue
			}
		}
		if strings.HasPrefix(app.ID, "flathub:") || strings.HasPrefix(app.ID, "appimage:") {
			if index, ok := c.matchName(app.Name); ok && generatedAppID(c.apps[index].ID) {
				c.setApplication(index, mergeApplication(c.apps[index], app))
				continue
			}
		}
		if index, ok := c.byID[app.ID]; ok {
			c.setApplication(index, mergeApplication(c.apps[index], app))
			continue
		}
		if validateApp(app) != nil {
			continue
		}
		index := len(c.apps)
		c.apps = append(c.apps, app)
		c.searchText = append(c.searchText, app.SearchText())
		c.searchName = append(c.searchName, strings.ToLower(app.Name))
		c.providerMasks = append(c.providerMasks, applicationProviderMask(app))
		c.byID[app.ID] = index
		c.indexApplication(index, app)
		added++
	}
	return added
}

func (c *Catalog) FindProvider(d model.Distro, kind model.ProviderKind, packageID string) (model.Application, int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	index, ok := c.byProvider[providerKey(kind, packageID)]
	if !ok || index < 0 || index >= len(c.apps) {
		return model.Application{}, -1, false
	}
	app := c.apps[index]
	for i, variant := range app.Variants[d] {
		if variant.Kind == kind && variant.Package == packageID {
			return app, i, true
		}
	}
	return model.Application{}, -1, false
}

const searchResultLimit = 2500

func (c *Catalog) Search(d model.Distro, query, category string) []model.Application {
	return c.search(d, query, category, false, searchResultLimit)
}

func (c *Catalog) SearchAll(query, category string) []model.Application {
	return c.searchAll(query, category, nil, false, searchResultLimit)
}

func (c *Catalog) SearchAllByProviders(query, category string, providers []model.ProviderKind) []model.Application {
	return c.searchAll(query, category, providers, false, searchResultLimit)
}

func (c *Catalog) CuratedSearch(d model.Distro, query, category string) []model.Application {
	return c.search(d, query, category, true, 0)
}

func (c *Catalog) CuratedSearchAll(query, category string) []model.Application {
	return c.searchAll(query, category, nil, true, 0)
}

func (c *Catalog) CountAll(query, category string) int {
	return c.CountAllByProviders(query, category, nil)
}

func (c *Catalog) CountAllByProviders(query, category string, providers []model.ProviderKind) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	cat := strings.ToLower(strings.TrimSpace(category))
	wanted := providersMask(providers)
	count := 0
	for i, app := range c.apps {
		if wanted != 0 && c.providerMasks[i]&wanted == 0 {
			continue
		}
		if q != "" && !strings.Contains(c.searchText[i], q) {
			continue
		}
		if cat != "" && cat != "all" && !hasCategory(app.Categories, cat) {
			continue
		}
		count++
	}
	return count
}

func (c *Catalog) Count(d model.Distro, query, category string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	cat := strings.ToLower(strings.TrimSpace(category))
	count := 0
	for i, app := range c.apps {
		if !app.Supports(d) {
			continue
		}
		if q != "" && !strings.Contains(c.searchText[i], q) {
			continue
		}
		if cat != "" && cat != "all" && !hasCategory(app.Categories, cat) {
			continue
		}
		count++
	}
	return count
}

func (c *Catalog) search(d model.Distro, query, category string, curatedOnly bool, limit int) []model.Application {
	c.mu.RLock()
	defer c.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	cat := strings.ToLower(strings.TrimSpace(category))
	capacity := len(c.apps)
	if limit > 0 && capacity > limit {
		capacity = limit
	}
	out := make([]model.Application, 0, capacity)
	for i, app := range c.apps {
		if curatedOnly && !c.curated[app.ID] {
			continue
		}
		if !app.Supports(d) {
			continue
		}
		if q != "" && !strings.Contains(c.searchText[i], q) {
			continue
		}
		if cat != "" && cat != "all" && !hasCategory(app.Categories, cat) {
			continue
		}
		out = append(out, app)
		if q == "" && limit > 0 && len(out) >= limit {
			break
		}
	}
	if q != "" {
		sortSearchResults(out, q, c.curated)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (c *Catalog) searchAll(query, category string, providers []model.ProviderKind, curatedOnly bool, limit int) []model.Application {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.searchAllLocked(query, category, providers, curatedOnly, limit)
}

func (c *Catalog) searchAllLocked(query, category string, providers []model.ProviderKind, curatedOnly bool, limit int) []model.Application {
	q := strings.ToLower(strings.TrimSpace(query))
	cat := strings.ToLower(strings.TrimSpace(category))
	wanted := providersMask(providers)
	capacity := len(c.apps)
	if limit > 0 && capacity > limit {
		capacity = limit
	}
	out := make([]model.Application, 0, capacity)
	for i, app := range c.apps {
		if curatedOnly && !c.curated[app.ID] {
			continue
		}
		if wanted != 0 && c.providerMasks[i]&wanted == 0 {
			continue
		}
		if q != "" && !strings.Contains(c.searchText[i], q) {
			continue
		}
		if cat != "" && cat != "all" && !hasCategory(app.Categories, cat) {
			continue
		}
		out = append(out, app)
		if q == "" && limit > 0 && len(out) >= limit {
			break
		}
	}
	if q != "" {
		sortSearchResults(out, q, c.curated)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

type BrowseResult struct {
	Apps       []model.Application
	Categories []string
	Count      int
}

func (c *Catalog) BrowseAllByProviders(query, category string, providers []model.ProviderKind) BrowseResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	cat := strings.ToLower(strings.TrimSpace(category))
	wanted := providersMask(providers)
	seenCategories := map[string]struct{}{}
	apps := make([]model.Application, 0, min(len(c.apps), searchResultLimit))
	matches := make(browseHeap, 0, searchResultLimit)
	count := 0
	for i, app := range c.apps {
		if wanted != 0 && c.providerMasks[i]&wanted == 0 {
			continue
		}
		for _, name := range app.Categories {
			seenCategories[name] = struct{}{}
		}
		if q != "" && !strings.Contains(c.searchText[i], q) {
			continue
		}
		if cat != "" && cat != "all" && !hasCategory(app.Categories, cat) {
			continue
		}
		count++
		if q == "" {
			if len(apps) < searchResultLimit {
				apps = append(apps, app)
			}
			continue
		}
		candidate := browseMatch{
			app:   app,
			name:  c.searchName[i],
			score: searchScoreName(c.searchName[i], q, c.curated[app.ID]),
		}
		if len(matches) < searchResultLimit {
			matches = append(matches, candidate)
			if len(matches) == searchResultLimit {
				heap.Init(&matches)
			}
		} else if betterBrowseMatch(candidate, matches[0]) {
			matches[0] = candidate
			heap.Fix(&matches, 0)
		}
	}
	if q != "" {
		sort.Slice(matches, func(i, j int) bool { return betterBrowseMatch(matches[i], matches[j]) })
		apps = make([]model.Application, len(matches))
		for i := range matches {
			apps[i] = matches[i].app
		}
	}
	categories := make([]string, 0, len(seenCategories)+1)
	categories = append(categories, "All")
	for name := range seenCategories {
		categories = append(categories, name)
	}
	sort.Strings(categories[1:])
	return BrowseResult{Apps: apps, Categories: categories, Count: count}
}

type browseMatch struct {
	app   model.Application
	name  string
	score int
}

type browseHeap []browseMatch

func (h browseHeap) Len() int { return len(h) }
func (h browseHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score < h[j].score
	}
	return h[i].name > h[j].name
}
func (h browseHeap) Swap(i, j int)   { h[i], h[j] = h[j], h[i] }
func (h *browseHeap) Push(value any) { *h = append(*h, value.(browseMatch)) }
func (h *browseHeap) Pop() any {
	old := *h
	n := len(old)
	value := old[n-1]
	*h = old[:n-1]
	return value
}

func betterBrowseMatch(a, b browseMatch) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	return a.name < b.name
}

func (c *Catalog) ProviderCounts() (int, map[model.ProviderKind]int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	counts := map[model.ProviderKind]int{}
	for _, mask := range c.providerMasks {
		for _, kind := range providerKinds {
			if mask&providerBit(kind) != 0 {
				counts[kind]++
			}
		}
	}
	return len(c.apps), counts
}

func (c *Catalog) CategoriesAll() []string {
	return c.CategoriesAllByProviders(nil)
}

func (c *Catalog) CategoriesAllByProviders(providers []model.ProviderKind) []string {
	return c.BrowseAllByProviders("", "All", providers).Categories
}

func (c *Catalog) Categories(d model.Distro) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	seen := map[string]bool{}
	for _, app := range c.apps {
		if !app.Supports(d) {
			continue
		}
		for _, cat := range app.Categories {
			seen[cat] = true
		}
	}
	out := make([]string, 0, len(seen)+1)
	out = append(out, "All")
	for cat := range seen {
		out = append(out, cat)
	}
	sort.Strings(out[1:])
	return out
}

func (c *Catalog) indexProviders(index int, app model.Application) {
	for _, variants := range app.Variants {
		for _, variant := range variants {
			if strings.TrimSpace(variant.Package) == "" {
				continue
			}
			c.byProvider[providerKey(variant.Kind, variant.Package)] = index
		}
	}
}

func (c *Catalog) matchProvider(app model.Application) (int, bool) {
	for _, variants := range app.Variants {
		for _, variant := range variants {
			if variant.Package == "" {
				continue
			}
			if index, ok := c.byProvider[providerKey(variant.Kind, variant.Package)]; ok {
				return index, true
			}
		}
	}
	return 0, false
}

func (c *Catalog) matchName(name string) (int, bool) {
	want := identityName(name)
	if want == "" {
		return 0, false
	}
	indexes := c.byName[want]
	if len(indexes) != 1 {
		return 0, false
	}
	for index := range indexes {
		return index, true
	}
	return 0, false
}

func (c *Catalog) matchPortableRepo(app model.Application) (int, bool) {
	found := -1
	for repo := range portableRepos(app) {
		for index := range c.byPortableRepo[repo] {
			if found != -1 && found != index {
				return 0, false
			}
			found = index
		}
	}
	return found, found >= 0
}

func (c *Catalog) ensureIndexes() {
	if c.byProvider == nil {
		c.byProvider = map[string]int{}
	}
	if len(c.searchText) != len(c.apps) {
		c.searchText = make([]string, len(c.apps))
		for i, app := range c.apps {
			c.searchText[i] = app.SearchText()
		}
	}
	if len(c.searchName) != len(c.apps) {
		c.searchName = make([]string, len(c.apps))
		for i, app := range c.apps {
			c.searchName[i] = strings.ToLower(app.Name)
		}
	}
	if len(c.providerMasks) != len(c.apps) {
		c.providerMasks = make([]uint16, len(c.apps))
		for i, app := range c.apps {
			c.providerMasks[i] = applicationProviderMask(app)
		}
	}
	if c.byName != nil && c.byPortableRepo != nil {
		return
	}
	c.byName = map[string]map[int]struct{}{}
	c.byPortableRepo = map[string]map[int]struct{}{}
	for i, app := range c.apps {
		c.indexApplication(i, app)
	}
}

func (c *Catalog) indexApplication(index int, app model.Application) {
	c.indexProviders(index, app)
	addIndex(c.byName, identityName(app.Name), index)
	for repo := range portableRepos(app) {
		addIndex(c.byPortableRepo, repo, index)
	}
}

func (c *Catalog) unindexApplication(index int, app model.Application) {
	for _, variants := range app.Variants {
		for _, variant := range variants {
			key := providerKey(variant.Kind, variant.Package)
			if c.byProvider[key] == index {
				delete(c.byProvider, key)
			}
		}
	}
	removeIndex(c.byName, identityName(app.Name), index)
	for repo := range portableRepos(app) {
		removeIndex(c.byPortableRepo, repo, index)
	}
}

func (c *Catalog) setApplication(index int, app model.Application) {
	c.unindexApplication(index, c.apps[index])
	c.apps[index] = app
	c.searchText[index] = app.SearchText()
	c.searchName[index] = strings.ToLower(app.Name)
	c.providerMasks[index] = applicationProviderMask(app)
	c.indexApplication(index, app)
}

func portableRepos(app model.Application) map[string]struct{} {
	out := map[string]struct{}{}
	for _, variants := range app.Variants {
		for _, variant := range variants {
			if variant.Kind != model.ProviderAppImage {
				continue
			}
			repo := strings.ToLower(strings.TrimSpace(variant.ReleaseRepo))
			if repo != "" {
				out[repo] = struct{}{}
			}
		}
	}
	return out
}

func addIndex(index map[string]map[int]struct{}, key string, appIndex int) {
	if key == "" {
		return
	}
	if index[key] == nil {
		index[key] = map[int]struct{}{}
	}
	index[key][appIndex] = struct{}{}
}

func removeIndex(index map[string]map[int]struct{}, key string, appIndex int) {
	if key == "" || index[key] == nil {
		return
	}
	delete(index[key], appIndex)
	if len(index[key]) == 0 {
		delete(index, key)
	}
}

func generatedAppID(id string) bool {
	return strings.HasPrefix(id, "native:") || strings.HasPrefix(id, "nixpkgs:") || strings.HasPrefix(id, "flathub:") || strings.HasPrefix(id, "appimage:")
}

func identityName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func providerKey(kind model.ProviderKind, packageID string) string {
	return string(kind) + "\x00" + strings.TrimSpace(packageID)
}

func mergeProviderApplication(current, incoming model.Application) model.Application {
	incoming.Name = ""
	incoming.Summary = ""
	incoming.Description = ""
	incoming.Developer = ""
	incoming.Homepage = ""
	incoming.Icon = ""
	incoming.Screenshots = nil
	return mergeApplication(current, incoming)
}

func mergeApplication(current, incoming model.Application) model.Application {
	current = cloneApplication(current)
	if incoming.Name != "" {
		current.Name = incoming.Name
	}
	if incoming.Summary != "" {
		current.Summary = incoming.Summary
	}
	if incoming.Description != "" {
		current.Description = incoming.Description
	}
	if incoming.Developer != "" {
		current.Developer = incoming.Developer
	}
	if incoming.Homepage != "" {
		current.Homepage = incoming.Homepage
	}
	if incoming.Icon != "" {
		current.Icon = incoming.Icon
	}
	if len(incoming.Screenshots) > 0 {
		current.Screenshots = uniqueStrings(append(current.Screenshots, incoming.Screenshots...))
	}
	current.Categories = uniqueStrings(append(current.Categories, incoming.Categories...))
	current.Keywords = uniqueStrings(append(current.Keywords, incoming.Keywords...))
	if current.Variants == nil {
		current.Variants = map[model.Distro][]model.Variant{}
	}
	for distro, variants := range incoming.Variants {
		seen := map[string]bool{}
		for _, variant := range current.Variants[distro] {
			seen[providerKey(variant.Kind, variant.Package)] = true
		}
		for _, variant := range variants {
			key := providerKey(variant.Kind, variant.Package)
			if key == providerKey("", "") || seen[key] {
				continue
			}
			if index := equivalentProviderIndex(current.Variants[distro], variant); index >= 0 {
				current.Variants[distro][index] = mergeEquivalentVariant(current.Variants[distro][index], variant)
				continue
			}
			seen[key] = true
			current.Variants[distro] = append(current.Variants[distro], variant)
		}
	}
	return current
}

func equivalentProviderIndex(variants []model.Variant, incoming model.Variant) int {
	if incoming.Kind != model.ProviderNixProfile && incoming.Kind != model.ProviderNixConfig {
		return -1
	}
	for i, variant := range variants {
		if variant.Kind == incoming.Kind {
			return i
		}
	}
	return -1
}

func mergeEquivalentVariant(current, incoming model.Variant) model.Variant {
	if !current.Verified && !current.Recommended && incoming.Package != "" {
		current.Package = incoming.Package
	}
	if current.Label == "" {
		current.Label = incoming.Label
	}
	if incoming.Version != "" {
		current.Version = incoming.Version
	}
	if current.ProfileName == "" {
		current.ProfileName = incoming.ProfileName
	}
	if current.Channel == "" {
		current.Channel = incoming.Channel
	}
	if current.Notes == "" {
		current.Notes = incoming.Notes
	}
	current.Recommended = current.Recommended || incoming.Recommended
	current.Official = current.Official || incoming.Official
	current.Sandboxed = current.Sandboxed || incoming.Sandboxed
	current.RequiresRoot = current.RequiresRoot || incoming.RequiresRoot
	current.Verified = current.Verified || incoming.Verified
	return current
}

func cloneApplication(app model.Application) model.Application {
	app.Screenshots = append([]string(nil), app.Screenshots...)
	app.Categories = append([]string(nil), app.Categories...)
	app.Keywords = append([]string(nil), app.Keywords...)
	if app.Variants != nil {
		variants := make(map[model.Distro][]model.Variant, len(app.Variants))
		for distro, list := range app.Variants {
			copyList := make([]model.Variant, len(list))
			copy(copyList, list)
			for i := range copyList {
				copyList[i].Architectures = append([]string(nil), copyList[i].Architectures...)
				copyList[i].Libcs = append([]string(nil), copyList[i].Libcs...)
			}
			variants[distro] = copyList
		}
		app.Variants = variants
	}
	return app
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

var providerKinds = []model.ProviderKind{
	model.ProviderDNF,
	model.ProviderPacman,
	model.ProviderXBPS,
	model.ProviderNixProfile,
	model.ProviderNixConfig,
	model.ProviderFlatpak,
	model.ProviderAppImage,
	model.ProviderUpstream,
}

func providerBit(kind model.ProviderKind) uint16 {
	switch kind {
	case model.ProviderDNF:
		return 1 << 0
	case model.ProviderPacman:
		return 1 << 1
	case model.ProviderXBPS:
		return 1 << 2
	case model.ProviderNixProfile:
		return 1 << 3
	case model.ProviderNixConfig:
		return 1 << 4
	case model.ProviderFlatpak:
		return 1 << 5
	case model.ProviderAppImage:
		return 1 << 6
	case model.ProviderUpstream:
		return 1 << 7
	default:
		return 0
	}
}

func providersMask(providers []model.ProviderKind) uint16 {
	var mask uint16
	for _, kind := range providers {
		mask |= providerBit(kind)
	}
	return mask
}

func applicationProviderMask(app model.Application) uint16 {
	var mask uint16
	for _, variants := range app.Variants {
		for _, variant := range variants {
			mask |= providerBit(variant.Kind)
		}
	}
	return mask
}

func sortSearchResults(out []model.Application, query string, curated map[string]bool) {
	sort.SliceStable(out, func(i, j int) bool {
		si := searchScore(out[i], query, curated[out[i].ID])
		sj := searchScore(out[j], query, curated[out[j].ID])
		if si != sj {
			return si > sj
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
}

func searchScore(app model.Application, query string, curated bool) int {
	return searchScoreName(strings.ToLower(app.Name), query, curated)
}

func searchScoreName(name, query string, curated bool) int {
	score := 0
	if query != "" {
		switch {
		case name == query:
			score += 1000
		case strings.HasPrefix(name, query):
			score += 700
		case strings.Contains(name, query):
			score += 500
		default:
			score += 100
		}
	}
	if curated {
		score += 25
	}
	return score
}

func appHasAnyProvider(app model.Application, providers []model.ProviderKind) bool {
	if len(providers) == 0 {
		return true
	}
	wanted := make(map[model.ProviderKind]bool, len(providers))
	for _, provider := range providers {
		wanted[provider] = true
	}
	for _, variants := range app.Variants {
		for _, variant := range variants {
			if wanted[variant.Kind] {
				return true
			}
		}
	}
	return false
}

func hasCategory(categories []string, want string) bool {
	for _, category := range categories {
		if strings.EqualFold(category, want) {
			return true
		}
	}
	return false
}
