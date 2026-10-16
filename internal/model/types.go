package model

import "strings"

type Distro string

const (
	DistroUnknown Distro = "unknown"
	DistroFedora  Distro = "fedora"
	DistroArch    Distro = "arch"
	DistroCachyOS Distro = "cachyos"
	DistroVoid    Distro = "void"
	DistroNixOS   Distro = "nixos"
)

func (d Distro) DisplayName() string {
	switch d {
	case DistroFedora:
		return "Fedora"
	case DistroArch:
		return "Arch Linux"
	case DistroCachyOS:
		return "CachyOS"
	case DistroVoid:
		return "Void Linux"
	case DistroNixOS:
		return "NixOS"
	default:
		return "Unknown"
	}
}

func SupportedDistros() []Distro {
	return []Distro{DistroFedora, DistroArch, DistroCachyOS, DistroVoid, DistroNixOS}
}

type ProviderKind string

const (
	ProviderDNF        ProviderKind = "dnf"
	ProviderPacman     ProviderKind = "pacman"
	ProviderXBPS       ProviderKind = "xbps"
	ProviderNixProfile ProviderKind = "nix-profile"
	ProviderNixConfig  ProviderKind = "nix-config"
	ProviderFlatpak    ProviderKind = "flatpak"
	ProviderAppImage   ProviderKind = "appimage"
	ProviderUpstream   ProviderKind = "upstream"
)

func (p ProviderKind) DisplayName() string {
	switch p {
	case ProviderDNF:
		return "Fedora repository"
	case ProviderPacman:
		return "Native repository"
	case ProviderXBPS:
		return "Void repository"
	case ProviderNixProfile:
		return "Nix profile"
	case ProviderNixConfig:
		return "NixOS configuration"
	case ProviderFlatpak:
		return "Flatpak"
	case ProviderAppImage:
		return "AppImage"
	case ProviderUpstream:
		return "Upstream"
	default:
		return string(p)
	}
}

type Variant struct {
	Kind          ProviderKind `json:"kind"`
	Package       string       `json:"package"`
	Label         string       `json:"label,omitempty"`
	Version       string       `json:"version,omitempty"`
	Channel       string       `json:"channel,omitempty"`
	SourceURL     string       `json:"source_url,omitempty"`
	ReleaseRepo   string       `json:"release_repo,omitempty"`
	AssetPattern  string       `json:"asset_pattern,omitempty"`
	Entrypoint    string       `json:"entrypoint,omitempty"`
	ProfileName   string       `json:"profile_name,omitempty"`
	Architectures []string     `json:"architectures,omitempty"`
	Libcs         []string     `json:"libcs,omitempty"`
	Recommended   bool         `json:"recommended,omitempty"`
	Official      bool         `json:"official,omitempty"`
	Sandboxed     bool         `json:"sandboxed,omitempty"`
	RequiresRoot  bool         `json:"requires_root,omitempty"`
	Verified      bool         `json:"verified,omitempty"`
	Notes         string       `json:"notes,omitempty"`
}

type Application struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Summary     string               `json:"summary"`
	Description string               `json:"description"`
	Developer   string               `json:"developer,omitempty"`
	Homepage    string               `json:"homepage,omitempty"`
	Icon        string               `json:"icon,omitempty"`
	Screenshots []string             `json:"screenshots,omitempty"`
	Categories  []string             `json:"categories"`
	Keywords    []string             `json:"keywords,omitempty"`
	Variants    map[Distro][]Variant `json:"variants"`
}

func (a Application) Supports(d Distro) bool {
	return len(a.Variants[d]) > 0
}

func (a Application) SearchText() string {
	parts := []string{a.Name, a.Summary, a.Description, a.Developer}
	parts = append(parts, a.Categories...)
	parts = append(parts, a.Keywords...)
	return strings.ToLower(strings.Join(parts, " "))
}

func (a Application) RecommendedVariant(d Distro) int {
	variants := a.Variants[d]
	for i, v := range variants {
		if v.Recommended {
			return i
		}
	}
	if len(variants) > 0 {
		return 0
	}
	return -1
}
