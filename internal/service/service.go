package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Ryoku-dev/ryomarket/internal/catalog"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"github.com/Ryoku-dev/ryomarket/internal/portable"
	"github.com/Ryoku-dev/ryomarket/internal/provider"
)

type Installed struct {
	App          model.Application
	VariantIndex int
}

type Update struct {
	Installed
}

type availabilityState struct {
	available bool
	checkedAt time.Time
}

type Service struct {
	Catalog      *catalog.Catalog
	Backends     map[model.ProviderKind]provider.Backend
	Runner       provider.Runner
	mu           sync.RWMutex
	cache        map[string]map[int]bool
	availability map[string]availabilityState
}

func New(c *catalog.Catalog) *Service {
	return &Service{
		Catalog: c, Backends: provider.Registry(), Runner: provider.ExecRunner{},
		cache: map[string]map[int]bool{}, availability: map[string]availabilityState{},
	}
}

const availabilityTTL = 5 * time.Second

func (s *Service) cachedAvailability(key string, check func() bool) bool {
	now := time.Now()
	s.mu.RLock()
	state, ok := s.availability[key]
	s.mu.RUnlock()
	if ok && now.Sub(state.checkedAt) < availabilityTTL {
		return state.available
	}
	available := check()
	s.mu.Lock()
	s.availability[key] = availabilityState{available: available, checkedAt: now}
	s.mu.Unlock()
	return available
}

func (s *Service) Backend(kind model.ProviderKind) (provider.Backend, bool) {
	backend, ok := s.Backends[kind]
	if !ok {
		return nil, false
	}
	available := s.cachedAvailability("backend:"+string(kind), backend.Available)
	return backend, available
}

func (s *Service) VariantAvailable(distro model.Distro, variant model.Variant) bool {
	backend, ok := s.Backend(variant.Kind)
	if !ok {
		return false
	}
	if checker, ok := backend.(provider.DistroAvailability); ok {
		key := "distro:" + string(variant.Kind) + ":" + string(distro)
		if !s.cachedAvailability(key, func() bool { return checker.AvailableFor(distro) }) {
			return false
		}
	}
	if checker, ok := backend.(provider.VariantAvailability); ok {
		return checker.AvailableVariant(variant)
	}
	return true
}

func (s *Service) Plan(app model.Application, distro model.Distro, variantIndex int, op provider.Operation) (provider.Plan, error) {
	variants := app.Variants[distro]
	if variantIndex < 0 || variantIndex >= len(variants) {
		return provider.Plan{}, fmt.Errorf("invalid provider selection")
	}
	variant := variants[variantIndex]
	backend, ok := s.Backend(variant.Kind)
	if !ok || !s.VariantAvailable(distro, variant) {
		return provider.Plan{}, fmt.Errorf("%s is not available on this system", variant.Kind.DisplayName())
	}
	return backend.Plan(app, distro, variant, op)
}

func (s *Service) Execute(ctx context.Context, plan provider.Plan) (string, error) {
	return provider.Execute(ctx, s.Runner, plan)
}

func (s *Service) DetectInstalled(ctx context.Context, app model.Application, distro model.Distro) (Installed, bool) {
	variants := app.Variants[distro]
	installed := make(map[int]bool)
	first := -1
	for i, variant := range variants {
		if ctx.Err() != nil {
			break
		}
		backend, ok := s.Backend(variant.Kind)
		if !ok || !s.VariantAvailable(distro, variant) {
			continue
		}
		if backend.Installed(ctx, variant) {
			installed[i] = true
			if first == -1 {
				first = i
			}
		}
	}
	s.mu.Lock()
	if len(installed) == 0 {
		delete(s.cache, app.ID)
	} else {
		s.cache[app.ID] = installed
	}
	s.mu.Unlock()
	if first == -1 {
		return Installed{}, false
	}
	return Installed{App: app, VariantIndex: first}, true
}

func (s *Service) ScanInstalled(ctx context.Context, distro model.Distro) []Installed {
	apps := s.Catalog.CuratedSearch(distro, "", "")
	out := make([]Installed, 0)
	seen := map[string]bool{}
	for _, app := range apps {
		if ctx.Err() != nil {
			break
		}
		s.DetectInstalled(ctx, app, distro)
		for _, index := range s.InstalledVariants(app.ID) {
			key := fmt.Sprintf("%s:%d", app.ID, index)
			seen[key] = true
			out = append(out, Installed{App: app, VariantIndex: index})
		}
	}
	for _, item := range s.scanNixProfileInstalled(ctx, distro) {
		key := fmt.Sprintf("%s:%d", item.App.ID, item.VariantIndex)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	for _, item := range s.scanFlatpakInstalled(ctx, distro) {
		key := fmt.Sprintf("%s:%d", item.App.ID, item.VariantIndex)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	for _, item := range s.scanPortableInstalled(distro) {
		key := fmt.Sprintf("%s:%d", item.App.ID, item.VariantIndex)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func (s *Service) scanPortableInstalled(distro model.Distro) []Installed {
	manifests, err := portable.ListManifests()
	if err != nil {
		return nil
	}
	out := make([]Installed, 0, len(manifests))
	for _, manifest := range manifests {
		kind := model.ProviderAppImage
		if manifest.Kind == "archive" {
			kind = model.ProviderUpstream
		}
		app, index, ok := s.Catalog.FindProvider(distro, kind, manifest.ID)
		if !ok {
			continue
		}
		s.mu.Lock()
		if s.cache[app.ID] == nil {
			s.cache[app.ID] = map[int]bool{}
		}
		s.cache[app.ID][index] = true
		s.mu.Unlock()
		out = append(out, Installed{App: app, VariantIndex: index})
	}
	return out
}

func (s *Service) scanNixProfileInstalled(ctx context.Context, distro model.Distro) []Installed {
	out, err := exec.CommandContext(ctx, "nix", "--extra-experimental-features", "nix-command flakes", "profile", "list", "--json").Output()
	if err != nil {
		return nil
	}
	var document struct {
		Elements map[string]struct {
			Active   bool   `json:"active"`
			AttrPath string `json:"attrPath"`
		} `json:"elements"`
	}
	if json.Unmarshal(out, &document) != nil || document.Elements == nil {
		return nil
	}
	installed := make([]Installed, 0, len(document.Elements))
	for name, element := range document.Elements {
		if !element.Active {
			continue
		}
		packageID := normalizeNixAttr(element.AttrPath)
		app, index, ok := s.Catalog.FindProvider(distro, model.ProviderNixProfile, packageID)
		if !ok {
			app, index, ok = s.Catalog.FindProvider(distro, model.ProviderNixProfile, name)
		}
		if !ok {
			continue
		}
		s.mu.Lock()
		if s.cache[app.ID] == nil {
			s.cache[app.ID] = map[int]bool{}
		}
		s.cache[app.ID][index] = true
		s.mu.Unlock()
		installed = append(installed, Installed{App: app, VariantIndex: index})
	}
	return installed
}

func normalizeNixAttr(value string) string {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) >= 3 && (parts[0] == "legacyPackages" || parts[0] == "packages") {
		return strings.Join(parts[2:], ".")
	}
	return value
}

func (s *Service) scanFlatpakInstalled(ctx context.Context, distro model.Distro) []Installed {
	out, err := exec.CommandContext(ctx, "flatpak", "list", "--app", "--columns=application").Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	installed := make([]Installed, 0)
	for _, line := range strings.Split(string(out), "\n") {
		packageID := strings.TrimSpace(line)
		if packageID == "" || seen[packageID] {
			continue
		}
		seen[packageID] = true
		app, index, ok := s.Catalog.FindProvider(distro, model.ProviderFlatpak, packageID)
		if !ok {
			continue
		}
		s.mu.Lock()
		if s.cache[app.ID] == nil {
			s.cache[app.ID] = map[int]bool{}
		}
		s.cache[app.ID][index] = true
		s.mu.Unlock()
		installed = append(installed, Installed{App: app, VariantIndex: index})
	}
	return installed
}

func (s *Service) ScanUpdates(ctx context.Context, distro model.Distro, installed []Installed) []Update {
	out := make([]Update, 0)
	for _, item := range installed {
		if ctx.Err() != nil {
			break
		}
		variants := item.App.Variants[distro]
		if item.VariantIndex < 0 || item.VariantIndex >= len(variants) {
			continue
		}
		variant := variants[item.VariantIndex]
		backend, ok := s.Backend(variant.Kind)
		if !ok {
			continue
		}
		checker, ok := backend.(provider.UpdateChecker)
		if ok && checker.UpdateAvailable(ctx, variant) {
			out = append(out, Update{Installed: item})
		}
	}
	return out
}

func (s *Service) VariantInstalled(appID string, variantIndex int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[appID][variantIndex]
}

func (s *Service) InstalledVariants(appID string) []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	variants := s.cache[appID]
	out := make([]int, 0, len(variants))
	for index := range variants {
		out = append(out, index)
	}
	sort.Ints(out)
	return out
}

func (s *Service) CachedInstalled(appID string) (Installed, bool) {
	app, ok := s.Catalog.App(appID)
	if !ok {
		return Installed{}, false
	}
	indexes := s.InstalledVariants(appID)
	if len(indexes) == 0 {
		return Installed{}, false
	}
	return Installed{App: app, VariantIndex: indexes[0]}, true
}
