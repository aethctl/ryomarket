GO ?= go
PREFIX ?= /usr/local
DESTDIR ?=
BINDIR := $(DESTDIR)$(PREFIX)/bin
DATADIR := $(DESTDIR)$(PREFIX)/share

.PHONY: all build test test-core vet vet-core check-core install uninstall clean

all: build

build:
	mkdir -p build
	$(GO) build -trimpath -o build/ryomarket ./cmd/ryomarket
	$(GO) build -trimpath -o build/ryomarket-nix ./cmd/ryomarket-nix
	$(GO) build -trimpath -o build/ryomarket-portable ./cmd/ryomarket-portable

test:
	$(GO) test ./...

test-core:
	$(GO) test ./internal/catalog ./internal/metadata ./internal/model ./internal/nixconfig ./internal/platform ./internal/portable ./internal/provider ./internal/service ./internal/state ./cmd/ryomarket-nix ./cmd/ryomarket-portable

vet:
	$(GO) vet ./...

vet-core:
	$(GO) vet ./internal/catalog ./internal/metadata ./internal/model ./internal/nixconfig ./internal/platform ./internal/portable ./internal/provider ./internal/service ./internal/state ./cmd/ryomarket-nix ./cmd/ryomarket-portable

check-core: test-core vet-core

install: build
	install -Dm755 build/ryomarket $(BINDIR)/ryomarket
	install -Dm755 build/ryomarket-nix $(BINDIR)/ryomarket-nix
	install -Dm755 build/ryomarket-portable $(BINDIR)/ryomarket-portable
	install -Dm644 packaging/dev.ryoku.Ryomarket.desktop $(DATADIR)/applications/dev.ryoku.Ryomarket.desktop
	install -Dm644 packaging/dev.ryoku.Ryomarket.metainfo.xml $(DATADIR)/metainfo/dev.ryoku.Ryomarket.metainfo.xml
	install -Dm644 assets/ryomarket.svg $(DATADIR)/icons/hicolor/scalable/apps/dev.ryoku.Ryomarket.svg

uninstall:
	rm -f $(BINDIR)/ryomarket $(BINDIR)/ryomarket-nix $(BINDIR)/ryomarket-portable
	rm -f $(DATADIR)/applications/dev.ryoku.Ryomarket.desktop
	rm -f $(DATADIR)/metainfo/dev.ryoku.Ryomarket.metainfo.xml
	rm -f $(DATADIR)/icons/hicolor/scalable/apps/dev.ryoku.Ryomarket.svg

clean:
	rm -rf build
