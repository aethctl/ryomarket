package catalog

import (
	"strings"
	"testing"

	"github.com/Ryoku-dev/ryomarket/internal/metadata"
	"github.com/Ryoku-dev/ryomarket/internal/model"
)

func TestAppImageApplicationsCreatesPortableVariant(t *testing.T) {
	apps := AppImageApplications([]metadata.AppImageItem{{
		Name: "Example App", Description: "A useful desktop utility", Categories: []any{"Utility"},
		Links: []metadata.AppImageLink{{Type: "GitHub", URL: "owner/example"}},
		Icons: []string{"Example_App/icons/128x128/example.png"}, GlibcRequired: "2.31",
	}})
	if len(apps) != 1 {
		t.Fatalf("got %d apps", len(apps))
	}
	v := apps[0].Variants[model.DistroNixOS][0]
	if v.Kind != model.ProviderAppImage || v.ReleaseRepo != "owner/example" || len(v.Libcs) != 1 || v.Libcs[0] != "glibc" {
		t.Fatalf("unexpected variant: %#v", v)
	}
	if !strings.Contains(apps[0].Icon, "raw.githubusercontent.com/AppImage/appimage.github.io") {
		t.Fatalf("unexpected icon URL %q", apps[0].Icon)
	}
}

func TestMergeAddsNativeProviderToExistingIdentity(t *testing.T) {
	c := &Catalog{
		apps: []model.Application{{
			ID: "firefox", Name: "Firefox", Categories: []string{"Internet"},
			Variants: map[model.Distro][]model.Variant{model.DistroNixOS: {{Kind: model.ProviderFlatpak, Package: "org.mozilla.firefox"}}},
		}},
		byID: map[string]int{"firefox": 0}, byProvider: map[string]int{}, curated: map[string]bool{"firefox": true},
	}
	c.indexProviders(0, c.apps[0])
	c.Merge([]model.Application{{
		ID: "native:nixos:firefox", Name: "Firefox", Categories: []string{"Internet"},
		Variants: map[model.Distro][]model.Variant{model.DistroNixOS: {{Kind: model.ProviderNixProfile, Package: "firefox"}}},
	}})
	app, _ := c.App("firefox")
	if len(app.Variants[model.DistroNixOS]) != 2 {
		t.Fatalf("expected two providers, got %#v", app.Variants[model.DistroNixOS])
	}
}
