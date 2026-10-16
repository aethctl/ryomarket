package platform

import (
	"strings"
	"testing"

	"github.com/Ryoku-dev/ryomarket/internal/model"
)

func TestDetectFrom(t *testing.T) {
	tests := []struct {
		name string
		data string
		want model.Distro
	}{
		{"fedora", "ID=fedora\nPRETTY_NAME=Fedora Linux\n", model.DistroFedora},
		{"cachyos", "ID=cachyos\nID_LIKE=arch\n", model.DistroCachyOS},
		{"arch-like", "ID=endeavouros\nID_LIKE=arch\n", model.DistroArch},
		{"nixos", "ID=nixos\nVERSION_ID=26.11\n", model.DistroNixOS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFrom(strings.NewReader(tt.data))
			if got.Distro != tt.want {
				t.Fatalf("got %q want %q", got.Distro, tt.want)
			}
		})
	}
}
