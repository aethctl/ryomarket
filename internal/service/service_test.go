package service

import (
	"context"
	"testing"

	"github.com/Ryoku-dev/ryomarket/internal/catalog"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"github.com/Ryoku-dev/ryomarket/internal/provider"
)

type fakeBackend struct {
	kind      model.ProviderKind
	installed map[string]bool
	updates   map[string]bool
}

func (b fakeBackend) Kind() model.ProviderKind { return b.kind }
func (b fakeBackend) Available() bool          { return true }
func (b fakeBackend) Installed(_ context.Context, v model.Variant) bool {
	return b.installed[v.Package]
}
func (b fakeBackend) UpdateAvailable(_ context.Context, v model.Variant) bool {
	return b.updates[v.Package]
}
func (b fakeBackend) Plan(app model.Application, distro model.Distro, v model.Variant, op provider.Operation) (provider.Plan, error) {
	return provider.Plan{AppID: app.ID, AppName: app.Name, Distro: distro, Variant: v, Operation: op}, nil
}

func TestDetectInstalledTracksProvidersIndependently(t *testing.T) {
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	obs, ok := c.App("obs-studio")
	if !ok {
		t.Fatal("missing OBS catalog entry")
	}

	s := New(c)
	s.Backends = map[model.ProviderKind]provider.Backend{
		model.ProviderDNF:     fakeBackend{kind: model.ProviderDNF, installed: map[string]bool{"obs-studio": true}},
		model.ProviderFlatpak: fakeBackend{kind: model.ProviderFlatpak, installed: map[string]bool{"com.obsproject.Studio": true}},
	}

	if _, ok := s.DetectInstalled(context.Background(), obs, model.DistroFedora); !ok {
		t.Fatal("expected OBS to be detected")
	}
	indexes := s.InstalledVariants(obs.ID)
	if len(indexes) != 2 || indexes[0] != 0 || indexes[1] != 1 {
		t.Fatalf("unexpected installed variants: %v", indexes)
	}
	if !s.VariantInstalled(obs.ID, 0) || !s.VariantInstalled(obs.ID, 1) {
		t.Fatal("expected both providers to be independently installed")
	}
}

func TestScanUpdatesKeepsProviderIdentity(t *testing.T) {
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	obs, _ := c.App("obs-studio")
	s := New(c)
	s.Backends = map[model.ProviderKind]provider.Backend{
		model.ProviderDNF: fakeBackend{kind: model.ProviderDNF, installed: map[string]bool{"obs-studio": true}},
		model.ProviderFlatpak: fakeBackend{
			kind:      model.ProviderFlatpak,
			installed: map[string]bool{"com.obsproject.Studio": true},
			updates:   map[string]bool{"com.obsproject.Studio": true},
		},
	}
	installed := []Installed{{App: obs, VariantIndex: 0}, {App: obs, VariantIndex: 1}}
	updates := s.ScanUpdates(context.Background(), model.DistroFedora, installed)
	if len(updates) != 1 || updates[0].VariantIndex != 1 {
		t.Fatalf("unexpected updates: %#v", updates)
	}
}

type countingBackend struct {
	kind  model.ProviderKind
	calls int
}

func (b *countingBackend) Kind() model.ProviderKind { return b.kind }
func (b *countingBackend) Available() bool {
	b.calls++
	return true
}
func (b *countingBackend) Installed(context.Context, model.Variant) bool { return false }
func (b *countingBackend) Plan(app model.Application, distro model.Distro, v model.Variant, op provider.Operation) (provider.Plan, error) {
	return provider.Plan{AppID: app.ID, AppName: app.Name, Distro: distro, Variant: v, Operation: op}, nil
}

func TestBackendAvailabilityIsCached(t *testing.T) {
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := &countingBackend{kind: model.ProviderNixProfile}
	s := New(c)
	s.Backends = map[model.ProviderKind]provider.Backend{model.ProviderNixProfile: backend}
	for i := 0; i < 100; i++ {
		if _, ok := s.Backend(model.ProviderNixProfile); !ok {
			t.Fatal("available backend was reported unavailable")
		}
	}
	if backend.calls != 1 {
		t.Fatalf("availability probed %d times, want 1", backend.calls)
	}
}
