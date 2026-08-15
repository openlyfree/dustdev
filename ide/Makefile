BUN ?= bun
GO ?= go
IMAGE_NAME ?= idehost
IMAGE_TAG ?= latest

FRONT_DIR := frontend
BACK_DIR := backend
BIN_DIR := bin
BINARY := $(BIN_DIR)/idehost

.PHONY: all front dist back image clean dev-front dev-back check shell lock

all: back

front:
	cd $(FRONT_DIR) && $(BUN) install && $(BUN) run build

dist: front
	rm -rf $(BACK_DIR)/dist
	cp -r $(FRONT_DIR)/build $(BACK_DIR)/dist

back: dist
	mkdir -p $(BIN_DIR)
	cd $(BACK_DIR) && CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o ../$(BINARY) .

image:
	podman build -t $(IMAGE_NAME):$(IMAGE_TAG) -f Containerfile .

clean:
	rm -rf $(BIN_DIR) $(BACK_DIR)/dist $(FRONT_DIR)/build $(FRONT_DIR)/.svelte-kit
	cd $(BACK_DIR) && go mod tidy

dev-front:
	cd $(FRONT_DIR) && $(BUN) run dev

dev-back:
	cd $(BACK_DIR) && $(GO) run .

check:
	cd $(FRONT_DIR) && $(BUN) run check
	cd $(BACK_DIR) && $(GO) vet ./...

shell:
	nix develop

lock:
	nix flake lock
