package catalog

import (
	"fmt"
	"sync"
	"testing"

	"github.com/Ryoku-dev/ryomarket/internal/metadata"
	"github.com/Ryoku-dev/ryomarket/internal/model"
)

func TestCatalogLoadsAndDeduplicatesProviders(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	obs, ok := c.App("obs-studio")
	if !ok {
		t.Fatal("missing obs-studio")
	}
	if len(obs.Variants[model.DistroFedora]) < 2 {
		t.Fatal("expected Fedora OBS to expose multiple providers")
	}
}

func TestSearchRespectsDistro(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	apps := c.Search(model.DistroVoid, "hardware encoding", "")
	if len(apps) != 1 || apps[0].ID != "obs-studio" {
		t.Fatalf("unexpected search result: %#v", apps)
	}
}

func TestMergeDeduplicatesFlatpakProvider(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	before := len(c.All())
	remote := model.Application{
		ID:         "flathub:com.obsproject.Studio",
		Name:       "OBS Studio Remote",
		Summary:    "Updated summary",
		Categories: []string{"Audio & Video"},
		Variants: map[model.Distro][]model.Variant{
			model.DistroNixOS: {{Kind: model.ProviderFlatpak, Package: "com.obsproject.Studio", Recommended: true}},
		},
	}
	if added := c.Merge([]model.Application{remote}); added != 0 {
		t.Fatalf("expected merge into curated app, added %d", added)
	}
	if len(c.All()) != before {
		t.Fatal("merge created duplicate listing")
	}
	obs, ok := c.App("obs-studio")
	if !ok || obs.Summary != "Updated summary" {
		t.Fatalf("metadata was not merged: %#v", obs)
	}
}

func TestMergeAddsDynamicFlatpakAndCuratedSearchStaysSmall(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	before := len(c.CuratedSearch(model.DistroNixOS, "", ""))
	remote := model.Application{
		ID:         "flathub:org.example.New",
		Name:       "New App",
		Summary:    "Dynamic app",
		Categories: []string{"Utilities"},
		Variants: map[model.Distro][]model.Variant{
			model.DistroNixOS: {{Kind: model.ProviderFlatpak, Package: "org.example.New", Recommended: true}},
		},
	}
	if added := c.Merge([]model.Application{remote}); added != 1 {
		t.Fatalf("expected one dynamic app, added %d", added)
	}
	if len(c.CuratedSearch(model.DistroNixOS, "", "")) != before {
		t.Fatal("dynamic app leaked into curated-only scan")
	}
	apps := c.Search(model.DistroNixOS, "new app", "")
	if len(apps) != 1 || apps[0].ID != remote.ID {
		t.Fatalf("unexpected dynamic search result %#v", apps)
	}
	app, index, ok := c.FindProvider(model.DistroNixOS, model.ProviderFlatpak, "org.example.New")
	if !ok || app.ID != remote.ID || index != 0 {
		t.Fatalf("provider lookup failed app=%#v index=%d ok=%v", app, index, ok)
	}
}

func TestFlathubApplicationsSupportEveryDistro(t *testing.T) {
	apps := FlathubApplications([]metadata.FlathubSearchApp{{
		AppID: "org.example.StoreApp", Name: "Store App", Summary: "Useful",
		Categories: []string{"Utility"}, Verified: true,
	}})
	if len(apps) != 1 {
		t.Fatalf("unexpected apps %#v", apps)
	}
	app := apps[0]
	if app.ID != "flathub:org.example.StoreApp" || app.Categories[0] != "Utilities" {
		t.Fatalf("unexpected converted app %#v", app)
	}
	for _, distro := range model.SupportedDistros() {
		variants := app.Variants[distro]
		if len(variants) != 1 || variants[0].Kind != model.ProviderFlatpak || variants[0].Package != "org.example.StoreApp" {
			t.Fatalf("missing Flatpak variant for %s: %#v", distro, variants)
		}
	}
}

func TestFlathubCategoryMappingUsesRyomarketVocabulary(t *testing.T) {
	apps := FlathubApplications([]metadata.FlathubSearchApp{
		{
			AppID:         "org.example.Chat",
			Name:          "Chat App",
			Categories:    []string{"Network"},
			SubCategories: []string{"Chat", "InstantMessaging"},
		},
		{
			AppID:         "org.example.Music",
			Name:          "Music App",
			Categories:    []string{"AudioVideo"},
			SubCategories: []string{"Music"},
		},
		{
			AppID:      "org.example.Code",
			Name:       "Code App",
			Categories: []string{"Development"},
		},
		{
			AppID:      "org.example.Graphics",
			Name:       "Graphics App",
			Categories: []string{"Graphics"},
		},
	})
	if len(apps) != 4 {
		t.Fatalf("unexpected apps %#v", apps)
	}
	assertCategories := func(app model.Application, want ...string) {
		t.Helper()
		for _, category := range want {
			if !hasCategory(app.Categories, category) {
				t.Fatalf("%s missing category %q from %#v", app.Name, category, app.Categories)
			}
		}
	}
	assertCategories(apps[0], "Internet", "Communication", "Social")
	assertCategories(apps[1], "Audio", "Audio & Music", "Media")
	assertCategories(apps[2], "Development")
	assertCategories(apps[3], "Graphics", "Creator")
}

func TestSearchCapsRenderedResultsButCountKeepsFullCatalogue(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	apps := make([]model.Application, 0, searchResultLimit+25)
	for i := 0; i < searchResultLimit+25; i++ {
		name := fmt.Sprintf("Package %04d", i)
		apps = append(apps, model.Application{
			ID: name, Name: name, Summary: "Utility tool", Categories: []string{"Utilities"},
			Variants: map[model.Distro][]model.Variant{
				model.DistroFedora: {{Kind: model.ProviderNixProfile, Package: fmt.Sprintf("pkg-%04d", i)}},
			},
		})
	}
	c.Merge(apps)
	if got := len(c.Search(model.DistroFedora, "", "")); got != searchResultLimit {
		t.Fatalf("expected rendered search limit %d, got %d", searchResultLimit, got)
	}
	if got := c.Count(model.DistroFedora, "", ""); got < searchResultLimit+25 {
		t.Fatalf("expected full catalogue count, got %d", got)
	}
}

func TestMergeUsesIdentityIndexForGeneratedProviders(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	base := make([]model.Application, 0, 2000)
	for i := 0; i < 2000; i++ {
		base = append(base, model.Application{
			ID: fmt.Sprintf("flathub:org.example.App%d", i), Name: fmt.Sprintf("Example App %d", i), Categories: []string{"Utilities"},
			Variants: map[model.Distro][]model.Variant{model.DistroFedora: {{Kind: model.ProviderFlatpak, Package: fmt.Sprintf("org.example.App%d", i)}}},
		})
	}
	c.Merge(base)
	providers := make([]model.Application, 0, len(base))
	for i := range base {
		providers = append(providers, model.Application{
			ID: fmt.Sprintf("nixpkgs:example-app-%d", i), Name: fmt.Sprintf("Example App %d", i), Categories: []string{"Utilities"},
			Variants: map[model.Distro][]model.Variant{model.DistroFedora: {{Kind: model.ProviderNixProfile, Package: fmt.Sprintf("example-app-%d", i)}}},
		})
	}
	c.Merge(providers)
	if len(c.byName) < 2000 {
		t.Fatalf("identity index was not populated: %d", len(c.byName))
	}
	app, ok := c.App("flathub:org.example.App1999")
	if !ok || len(app.Variants[model.DistroFedora]) != 2 {
		t.Fatalf("generated provider did not merge through identity index: %#v", app)
	}
}

func TestUnifiedSearchIgnoresDistroBoundaries(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c.Merge([]model.Application{
		{
			ID:         "native:fedora-only",
			Name:       "Fedora Only Tool",
			Summary:    "Fedora tool",
			Categories: []string{"System"},
			Variants: map[model.Distro][]model.Variant{
				model.DistroFedora: {{Kind: model.ProviderDNF, Package: "fedora-only"}},
			},
		},
		{
			ID:         "native:void-only",
			Name:       "Void Only Tool",
			Summary:    "Void tool",
			Categories: []string{"System"},
			Variants: map[model.Distro][]model.Variant{
				model.DistroVoid: {{Kind: model.ProviderXBPS, Package: "void-only"}},
			},
		},
	})

	if got := c.SearchAll("Fedora Only Tool", ""); len(got) != 1 || got[0].ID != "native:fedora-only" {
		t.Fatalf("unified search missed Fedora-only app: %#v", got)
	}
	if got := c.SearchAll("Void Only Tool", ""); len(got) != 1 || got[0].ID != "native:void-only" {
		t.Fatalf("unified search missed Void-only app: %#v", got)
	}
	if c.CountAll("Only Tool", "System") < 2 {
		t.Fatal("unified count should include apps from multiple distro providers")
	}
}

func TestProviderFilteredSearchRunsBeforeRenderCap(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	apps := make([]model.Application, 0, searchResultLimit+40)
	for i := 0; i < searchResultLimit+20; i++ {
		apps = append(apps, model.Application{
			ID: fmt.Sprintf("flatpak:cap-%d", i), Name: fmt.Sprintf("Flatpak Cap %d", i), Categories: []string{"Utilities"},
			Variants: map[model.Distro][]model.Variant{model.DistroFedora: {{Kind: model.ProviderFlatpak, Package: fmt.Sprintf("org.example.Cap%d", i)}}},
		})
	}
	for i := 0; i < 20; i++ {
		apps = append(apps, model.Application{
			ID: fmt.Sprintf("dnf:late-%d", i), Name: fmt.Sprintf("DNF Late %d", i), Categories: []string{"Utilities"},
			Variants: map[model.Distro][]model.Variant{model.DistroFedora: {{Kind: model.ProviderDNF, Package: fmt.Sprintf("dnf-late-%d", i)}}},
		})
	}
	c.Merge(apps)
	got := c.SearchAllByProviders("", "", []model.ProviderKind{model.ProviderDNF})
	if len(got) < 20 {
		t.Fatalf("provider filter ran after render cap: got %d DNF apps", len(got))
	}
}

func TestMergeCollapsesGeneratedNixVariantsIntoCuratedProviders(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	incoming := model.Application{
		ID: "nixpkgs:kdePackages.kdenlive", Name: "Kdenlive", Categories: []string{"Media"},
		Variants: map[model.Distro][]model.Variant{
			model.DistroNixOS: {
				{Kind: model.ProviderNixConfig, Package: "kdePackages.kdenlive", Label: "NixOS configuration", Official: true},
				{Kind: model.ProviderNixProfile, Package: "kdePackages.kdenlive", ProfileName: "kdenlive", Label: "Nix profile", Official: true},
			},
		},
	}
	c.Merge([]model.Application{incoming})
	app, ok := c.App("kdenlive")
	if !ok {
		t.Fatal("missing curated kdenlive")
	}
	counts := map[model.ProviderKind]int{}
	for _, variant := range app.Variants[model.DistroNixOS] {
		counts[variant.Kind]++
	}
	if counts[model.ProviderNixConfig] != 1 || counts[model.ProviderNixProfile] != 1 {
		t.Fatalf("duplicate Nix providers survived merge: %#v", app.Variants[model.DistroNixOS])
	}
}

func TestProviderCountsAndBrowseSnapshotStayConsistent(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c.Merge([]model.Application{{
		ID: "native:perf-count", Name: "Performance Count Tool", Categories: []string{"Utilities"},
		Variants: map[model.Distro][]model.Variant{
			model.DistroFedora: {{Kind: model.ProviderDNF, Package: "perf-count"}},
			model.DistroNixOS:  {{Kind: model.ProviderNixProfile, Package: "perf-count"}},
		},
	}})
	total, counts := c.ProviderCounts()
	if total != c.CountAll("", "") {
		t.Fatalf("provider total %d does not match catalogue total %d", total, c.CountAll("", ""))
	}
	if counts[model.ProviderDNF] == 0 || counts[model.ProviderNixProfile] == 0 {
		t.Fatalf("provider counts missing merged providers: %#v", counts)
	}
	browse := c.BrowseAllByProviders("Performance Count Tool", "Utilities", []model.ProviderKind{model.ProviderDNF})
	if browse.Count != 1 || len(browse.Apps) != 1 || browse.Apps[0].ID != "native:perf-count" {
		t.Fatalf("unexpected browse snapshot: %#v", browse)
	}
}

func TestBrowseSnapshotIsSafeWhileCatalogueMerges(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	const rounds = 100
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			_ = c.BrowseAllByProviders("obs", "All", nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			c.Merge([]model.Application{{
				ID: fmt.Sprintf("nixpkgs:race-%d", i), Name: fmt.Sprintf("Race Tool %d", i), Categories: []string{"Utilities"},
				Variants: map[model.Distro][]model.Variant{
					model.DistroNixOS: {{Kind: model.ProviderNixProfile, Package: fmt.Sprintf("race-%d", i)}},
				},
			}})
		}
	}()
	wg.Wait()
}
