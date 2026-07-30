# Fyne needs CGO + OpenGL/ALSA. On hosts without a linkable
# libXxf86vm.so (only .so.1), provide a local linker stub.
LINK_DIR := $(CURDIR)/.link
export CGO_LDFLAGS := -L$(LINK_DIR)

VERSION := $(shell sed -n 's/^Version = "\(.*\)"/\1/p' FyneApp.toml)
ARCH := amd64
DIST := dist
RELEASE_BIN := $(DIST)/robsong
TARBALL := $(DIST)/robsong-$(VERSION)-linux-$(ARCH).tar.gz
NFPM ?= $(shell command -v nfpm 2>/dev/null || echo "$(shell go env GOPATH)/bin/nfpm")

.PHONY: all build run clean stub deps test release tarball rpm deb package

all: build

stub:
	@mkdir -p $(LINK_DIR)
	@if [ ! -e $(LINK_DIR)/libXxf86vm.so ]; then \
		for candidate in \
			/usr/lib64/libXxf86vm.so \
			/usr/lib64/libXxf86vm.so.1 \
			/usr/lib/x86_64-linux-gnu/libXxf86vm.so \
			/usr/lib/x86_64-linux-gnu/libXxf86vm.so.1; do \
			if [ -e "$$candidate" ]; then \
				ln -sfn "$$candidate" $(LINK_DIR)/libXxf86vm.so; \
				break; \
			fi; \
		done; \
		if [ ! -e $(LINK_DIR)/libXxf86vm.so ]; then \
			echo "Missing libXxf86vm — install libXxf86vm-devel (Fedora) or libxxf86vm-dev (Debian/Ubuntu)"; \
			exit 1; \
		fi; \
	fi

build: stub
	@echo "Building (first time after clean can take 1–2 minutes)…"
	go build -o robsong ./cmd/robsong
	@echo "OK → ./robsong"

run: build
	./robsong

test:
	go vet ./...
	go test ./...

# Stripped release binary for distribution.
release: stub
	@mkdir -p $(DIST)
	@echo "Building release $(VERSION)…"
	go build -ldflags="-s -w" -o $(RELEASE_BIN) ./cmd/robsong
	@echo "OK → $(RELEASE_BIN)"

# usr/ layout tarball (binary + desktop entry + icon).
tarball: release
	@rm -rf $(DIST)/stage
	@mkdir -p \
		$(DIST)/stage/usr/bin \
		$(DIST)/stage/usr/share/applications \
		$(DIST)/stage/usr/share/icons/hicolor/512x512/apps
	cp $(RELEASE_BIN) $(DIST)/stage/usr/bin/robsong
	cp packaging/robsong.desktop $(DIST)/stage/usr/share/applications/robsong.desktop
	cp assets/logo.png $(DIST)/stage/usr/share/icons/hicolor/512x512/apps/robsong.png
	tar -C $(DIST)/stage -czf $(TARBALL) usr
	@rm -rf $(DIST)/stage
	@echo "OK → $(TARBALL)"

# Fedora/RHEL RPM via nFPM (requires: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest).
rpm: release
	@if [ ! -x "$(NFPM)" ]; then \
		echo "nfpm not found — install: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest"; \
		exit 1; \
	fi
	VERSION=$(VERSION) $(NFPM) package --packager rpm --config nfpm.yaml --target $(DIST)/
	@echo "OK → $(DIST)/robsong-$(VERSION)-1.x86_64.rpm"

# Debian/Ubuntu DEB via nFPM.
deb: release
	@if [ ! -x "$(NFPM)" ]; then \
		echo "nfpm not found — install: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest"; \
		exit 1; \
	fi
	VERSION=$(VERSION) $(NFPM) package --packager deb --config nfpm.yaml --target $(DIST)/
	@echo "OK → $(DIST)/robsong_$(VERSION)-1_amd64.deb"

# Build all distribution artifacts.
package: release tarball rpm deb
	@echo "Artifacts in $(DIST)/:"
	@ls -lh $(DIST)/robsong $(TARBALL) \
		$(DIST)/robsong-$(VERSION)-1.*.rpm \
		$(DIST)/robsong_$(VERSION)-1_*.deb 2>/dev/null || true

clean:
	rm -f robsong
	rm -rf $(LINK_DIR) $(DIST)

deps:
	sudo dnf install -y \
		golang gcc \
		libX11-devel libXcursor-devel libXrandr-devel \
		libXinerama-devel libXi-devel libXxf86vm-devel \
		libglvnd-devel alsa-lib-devel wayland-devel libxkbcommon-devel
