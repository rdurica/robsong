# Fyne needs CGO + OpenGL/ALSA. On Fedora without libXxf86vm-devel,
# we provide a local linker stub pointing at the runtime .so.
LINK_DIR := $(CURDIR)/.link
export CGO_LDFLAGS := -L$(LINK_DIR)

.PHONY: all build run clean stub deps

all: build

stub:
	@mkdir -p $(LINK_DIR)
	@if [ ! -e $(LINK_DIR)/libXxf86vm.so ]; then \
		if [ -e /usr/lib64/libXxf86vm.so ]; then \
			ln -sfn /usr/lib64/libXxf86vm.so $(LINK_DIR)/libXxf86vm.so; \
		elif [ -e /usr/lib64/libXxf86vm.so.1 ]; then \
			ln -sfn /usr/lib64/libXxf86vm.so.1 $(LINK_DIR)/libXxf86vm.so; \
		else \
			echo "Missing libXxf86vm — install: sudo dnf install -y libXxf86vm-devel"; \
			exit 1; \
		fi; \
	fi

build: stub
	@echo "Building (first time after clean can take 1–2 minutes)…"
	go build -o robsong ./cmd/robsong
	@echo "OK → ./robsong"

run: build
	./robsong

clean:
	rm -f robsong
	rm -rf $(LINK_DIR)

deps:
	sudo dnf install -y \
		golang gcc \
		libX11-devel libXcursor-devel libXrandr-devel \
		libXinerama-devel libXi-devel libXxf86vm-devel \
		libglvnd-devel alsa-lib-devel wayland-devel libxkbcommon-devel
