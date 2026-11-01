package ui

import (
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"testing"
)

func TestProviderMatchesHost(t *testing.T) {
	for _, d := range model.SupportedDistros() {
		for _, kind := range []model.ProviderKind{model.ProviderDNF, model.ProviderPacman, model.ProviderXBPS, model.ProviderNixConfig} {
			want := (kind == model.ProviderDNF && d == model.DistroFedora) || (kind == model.ProviderPacman && (d == model.DistroArch || d == model.DistroCachyOS)) || (kind == model.ProviderXBPS && d == model.DistroVoid) || (kind == model.ProviderNixConfig && d == model.DistroNixOS)
			if providerMatchesHost(d, kind) != want {
				t.Errorf("provider %s on %s", kind, d)
			}
		}
	}
}
