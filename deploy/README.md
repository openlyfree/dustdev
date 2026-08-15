# Deploying dustdev.app

Everything runs as podman containers on a single DigitalOcean droplet,
managed by systemd via Quadlet files.

```
dustdev.app            → Caddy serves the static SvelteKit bundle (/opt/dustdev/frontend)
dustdev.app/api/*      → Caddy proxies to the frontbackend container
<slug>.dustdev.app     → Caddy forward-auth → reverse-proxies to the project's
                         devcontainer (network alias == slug, port 8080)
```

## 1. Porkbun DNS

1. Create a DigitalOcean droplet (Fedora 40+ or Debian 12+ recommended; 2 GB+ RAM)
   and note its IPv4 address.
2. In Porkbun → Domain Management → dustdev.app → DNS Records, create:
   - `A` record — host `@` (blank), answer `<droplet IP>`
   - `A` record — host `*`, answer `<droplet IP>`
3. In Porkbun → Account → API Access: enable API access for dustdev.app and
   create an API key. These go into `dustdev.env` as `PORKBUN_API_KEY` /
   `PORKBUN_SECRET_KEY` (used by Caddy for the wildcard certificate).

## 2. Prepare the droplet

```sh
# Fedora
sudo dnf install -y podman aardvark-dns
# Debian
sudo apt update && sudo apt install -y podman aardvark-dns golang-go

# Rootful podman API socket (frontbackend mounts it)
sudo systemctl enable --now podman.socket

# Firewall
sudo ufw allow 80/tcp && sudo ufw allow 443/tcp && sudo ufw allow 443/udp
sudo ufw enable
```

## 3. Build the images

Images are built and pushed to GHCR by CI (`.github/workflows/cd.yml`). On the
droplet you just pull them — `deploy/deploy.sh` does this. To build by hand
instead (e.g. first-time bootstrap before CI has run), see "Manual build"
below.

## 4. Configure

```sh
sudo mkdir -p /etc/dustdev /opt/dustdev/frontend

sudo cp deploy/env.example /etc/dustdev/dustdev.env
sudoedit /etc/dustdev/dustdev.env     # fill in secrets (chmod 600)

sudo cp deploy/caddy/Caddyfile /etc/dustdev/Caddyfile

# Static frontend bundle (landing + dashboard)
cd frontfrontend && bun install && bun run build
sudo cp -r build/. /opt/dustdev/frontend/
```

## 5. Install the quadlets

```sh
sudo cp deploy/dustdev.network deploy/postgres.container \
        deploy/frontbackend.container deploy/caddy.container \
        /etc/containers/systemd/

sudo systemctl daemon-reload
sudo systemctl start dustdev-network.service postgres.service \
     frontbackend.service caddy.service
```

Check status with `journalctl -u caddy.service -f` — on first request Caddy
performs the Porkbun DNS-01 challenge and caches the wildcard certificate in
the `dustdev-caddy-data` volume.

## 6. Verify

```sh
curl -I https://dustdev.app                    # landing page
curl https://dustdev.app/api/healthz           # {"ok":true}
```

Sign up at https://dustdev.app/signup, create a project, hit **Start**, then
open `https://<slug>.dustdev.app` — you should land in the IDE. Opening the
same URL logged-out (or as another user) redirects to the login page.

## CI/CD

Two GitHub Actions workflows run the pipeline:

- **CI** (`.github/workflows/ci.yml`) — on every push/PR: `svelte-check`, `go
  vet`, `go test -race`, and a no-push build of all three images.
- **CD** (`.github/workflows/cd.yml`) — on `master`: builds and pushes
  `frontbackend`, `caddy`, and `idehost` to GHCR as both `production` (mutable)
  and the commit SHA, and uploads the static frontend bundle as an artifact.

CD never touches the droplet. Deploy from the server:

### Deploying

One-time setup on the droplet — put a GitHub PAT (`read:packages`) and your
repo slug in `/etc/dustdev/github.env`:

```sh
echo 'GITHUB_REPOSITORY=ethan/code-az'        | sudo tee    /etc/dustdev/github.env
echo 'GITHUB_TOKEN=ghp_...'                   | sudo tee -a /etc/dustdev/github.env
sudo chmod 600 /etc/dustdev/github.env
```

Then deploy whatever is on `main`:

```sh
sudo deploy/deploy.sh            # latest main build
sudo deploy/deploy.sh <sha>      # pin the frontend bundle to a specific build
```

The script pulls the fresh images, installs the frontend bundle into
`/opt/dustdev/frontend`, and restarts `frontbackend` and `caddy`. Running IDE
containers are untouched; each project pulls the latest `idehost:production`
the next time it starts (`pull=always`), so IDE updates roll out on restart
without redeploying the platform.

### Manual build (bootstrap / fallback)

```sh
git clone <this repo> dustdev && cd dustdev
make images          # builds idehost, frontbackend, and caddy locally
```

Tag them to the GHCR names the Quadlets expect (or edit the `Image=` lines in
`deploy/*.container` and `IDE_IMAGE` in `dustdev.env` to your `localhost/...`
names if you want to skip the registry entirely).
