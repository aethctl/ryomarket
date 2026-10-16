package platform

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/Ryoku-dev/ryomarket/internal/model"
)

type Info struct {
	Distro  model.Distro
	ID      string
	IDLike  []string
	Name    string
	Version string
}

func Detect() Info {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return Info{Distro: model.DistroUnknown}
	}
	defer f.Close()
	return DetectFrom(f)
}

func DetectFrom(r io.Reader) Info {
	values := map[string]string{}
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), "\"")
	}

	id := strings.ToLower(values["ID"])
	like := strings.Fields(strings.ToLower(values["ID_LIKE"]))
	return Info{
		Distro:  mapDistro(id, like),
		ID:      id,
		IDLike:  like,
		Name:    values["PRETTY_NAME"],
		Version: values["VERSION_ID"],
	}
}

func mapDistro(id string, like []string) model.Distro {
	switch id {
	case "fedora":
		return model.DistroFedora
	case "arch":
		return model.DistroArch
	case "cachyos":
		return model.DistroCachyOS
	case "void":
		return model.DistroVoid
	case "nixos":
		return model.DistroNixOS
	}

	for _, candidate := range like {
		switch candidate {
		case "fedora":
			return model.DistroFedora
		case "arch":
			return model.DistroArch
		case "void":
			return model.DistroVoid
		case "nixos":
			return model.DistroNixOS
		}
	}
	return model.DistroUnknown
}
