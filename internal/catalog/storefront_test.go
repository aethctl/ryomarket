package catalog

import (
	"fmt"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"runtime"
	"testing"
)

func TestHostBrowseCountsAndCategories(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c.Merge([]model.Application{{ID: "native:fedora-only", Name: "Fedora only", Categories: []string{"FedoraOnly"}, Variants: map[model.Distro][]model.Variant{model.DistroFedora: {{Kind: model.ProviderDNF, Package: "fedora-only"}}}}})
	providers := []model.ProviderKind{model.ProviderNixProfile, model.ProviderFlatpak, model.ProviderAppImage}
	result := c.BrowseFor(model.DistroNixOS, "", "All", providers, false)
	total, counts := c.ProviderCountsFor(model.DistroNixOS, providers)
	if result.Count != total {
		t.Fatalf("browse=%d counts=%d", result.Count, total)
	}
	if counts[model.ProviderDNF] != 0 {
		t.Fatal("DNF counted on NixOS")
	}
	for _, app := range result.Apps {
		if !app.Supports(model.DistroNixOS) {
			t.Fatalf("foreign app %s", app.ID)
		}
	}
	for _, category := range result.Categories {
		if category == "FedoraOnly" {
			t.Fatal("foreign category leaked")
		}
	}
	if got := c.BrowseFor(model.DistroNixOS, "", "All", []model.ProviderKind{"unavailable"}, false); got.Count != 0 {
		t.Fatal("empty capabilities exposed the catalogue")
	}
}

func TestCuratedLandingKeepsFullSearch(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	before := c.BrowseFor(model.DistroNixOS, "", "All", nil, true).Count
	var apps []model.Application
	for i := 0; i < 3000; i++ {
		apps = append(apps, model.Application{ID: fmt.Sprintf("nixpkgs:example-%d", i), Name: fmt.Sprintf("Example %d", i), Variants: map[model.Distro][]model.Variant{model.DistroNixOS: {{Kind: model.ProviderNixProfile, Package: fmt.Sprintf("example-%d", i)}}}})
	}
	c.Merge(apps)
	if got := c.BrowseFor(model.DistroNixOS, "", "All", nil, true).Count; got != before {
		t.Fatalf("curation grew %d -> %d", before, got)
	}
	result := c.BrowseFor(model.DistroNixOS, "Example", "All", nil, false)
	if result.Count != 3000 || len(result.Apps) != searchResultLimit {
		t.Fatalf("search count=%d rendered=%d", result.Count, len(result.Apps))
	}
}

func TestOfficialEntriesCannotOfferUnverifiedInstallation(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, app := range RyokuApplications() {
		if app.Ecosystem == nil || app.Ecosystem.Reason == "" || app.Homepage == "" {
			t.Fatalf("incomplete metadata %s", app.ID)
		}
		if len(app.Variants) != 0 {
			t.Fatalf("unverified install enabled for %s", app.ID)
		}
		if _, ok := c.App(app.ID); !ok {
			t.Fatalf("missing detail page %s", app.ID)
		}
	}
}

func BenchmarkHostBrowse(b *testing.B) {
	c, _ := Load()
	for i := 0; i < b.N; i++ {
		c.BrowseFor(model.DistroNixOS, "", "All", []model.ProviderKind{model.ProviderNixProfile}, true)
	}
}

func TestNameCollisionCannotEnableOfficialInstallation(t *testing.T) {
	c, _ := Load()
	c.Merge([]model.Application{{ID: "nixpkgs:untrusted-ryotunes", Name: "Ryotunes", Variants: map[model.Distro][]model.Variant{model.DistroNixOS: {{Kind: model.ProviderNixProfile, Package: "untrusted-ryotunes"}}}}})
	app, _ := c.App("ryoku:ryotunes")
	if len(app.Variants) != 0 {
		t.Fatal("name collision added an unverified provider")
	}
}

func TestPackagedRyokuCollectionHasIndependentSources(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("optional application packages currently support x86_64-linux")
	}
	t.Setenv("RYOMARKET_APPS_FLAKE", "path:/nix/store/packaged-source?narHash=sha256-test")
	apps := RyokuApplications()
	if len(apps) != 6 {
		t.Fatalf("expected six optional applications, got %d", len(apps))
	}
	for _, app := range apps {
		if app.ID == "ryoku:ryogami" {
			t.Fatal("essential Ryogami must not become optional")
		}
		if app.Description == app.Summary {
			t.Fatalf("missing description: %s", app.ID)
		}
		if err := validateApp(app); err != nil {
			t.Fatalf("%s: %v", app.ID, err)
		}
		if len(app.Variants[model.DistroNixOS]) != 3 {
			t.Fatalf("missing installation choices: %s", app.ID)
		}
		for _, v := range app.Variants[model.DistroNixOS] {
			if v.FlakeRef == "" || !v.Official {
				t.Fatalf("untrusted source: %#v", v)
			}
		}
	}
}
