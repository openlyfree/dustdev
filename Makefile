BUN ?= bun
GO ?= go
PODMAN ?= podman

DEV_DB := dustdev-dev-db
DEV_NET := dustdev

export DATABASE_URL ?= postgres://dustdev:dustdev@127.0.0.1:5432/dustdev
export BASE_DOMAIN ?= localhost
export URL_SCHEME ?= http
export PODMAN_NETWORK ?= $(DEV_NET)
export PODMAN_SOCKET ?= /run/user/$(shell id -u)/podman/podman.sock
export IDE_IMAGE ?= localhost/idehost:latest

.PHONY: dev dev-front dev-back dev-db dev-network build images check clean

# ── Local development ────────────────────────────────────────────────────

dev: dev-network dev-db
	@trap 'kill 0' INT TERM; \
	(cd frontbackend && $(GO) run .) & \
	(cd frontfrontend && $(BUN) install && $(BUN) run dev) & \
	wait

dev-front:
	cd frontfrontend && $(BUN) install && $(BUN) run dev

dev-back:
	cd frontbackend && $(GO) run .

dev-network:
	@$(PODMAN) network exists $(DEV_NET) || $(PODMAN) network create $(DEV_NET)

dev-db:
	@$(PODMAN) container exists $(DEV_DB) || \
		$(PODMAN) run -d --name $(DEV_DB) \
			-e POSTGRES_USER=dustdev -e POSTGRES_PASSWORD=dustdev -e POSTGRES_DB=dustdev \
			-p 127.0.0.1:5432:5432 \
			-v dustdev-dev-pgdata:/var/lib/postgresql \
			docker.io/library/postgres:18-alpine
	@$(PODMAN) start $(DEV_DB) 2>/dev/null || true

# ── Production artifacts ─────────────────────────────────────────────────

build:
	cd frontfrontend && $(BUN) install && $(BUN) run build
	cd frontbackend && CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o ../bin/frontbackend .

images:
	$(MAKE) -C ide image
	$(PODMAN) build -t localhost/dustdev-frontbackend:latest -f frontbackend/Containerfile frontbackend
	$(PODMAN) build -t localhost/dustdev-caddy:latest -f deploy/caddy/Containerfile deploy/caddy

check:
	cd frontfrontend && $(BUN) run check
	cd frontbackend && $(GO) vet ./...

clean:
	-$(PODMAN) rm -f $(DEV_DB)
	-$(PODMAN) volume rm dustdev-dev-pgdata
	-$(PODMAN) network rm $(DEV_NET)
	rm -rf bin frontfrontend/build frontfrontend/.svelte-kit
