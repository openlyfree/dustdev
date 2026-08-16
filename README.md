# dustdev

codespaces but worse, cheaper, and running on a single droplet i pay for myself.

sign up, make a project, hit start, and a few seconds later you're in a full IDE at `<slug>.dustdev.app` — monaco editor, a real terminal, file tree, port previews, automatic TLS. containers get auto-stopped after 10 minutes of no activity because ram isn't free (your files live on a volume, nothing is lost, hit start again and you're back where you left off).

its live at [dustdev.app](https://dustdev.app)

## What does it do?

- **disposable devcontainers** — every project is its own podman container + volume. spin one up in seconds, throw it away when you're done
- **a full IDE in the browser** — monaco (the vs code editor), xterm.js terminal, file tree. no install, no sync, no setup
- **instant port previews** — start a dev server and it shows up automatically at `/preview/<port>`, proxied and ready
- **a subdomain per project** — `<slug>.dustdev.app` with a wildcard cert, so it works from your laptop, a tablet, or a borrowed machine
- **start/stop on demand** — stopped environments keep their files and cost nothing
- **nix inside** — the IDE image is built on a nix flake, so there's a nix store in your container if you want to declaratively ruin your environment
- **keeps working offline** — lose your connection (or the droplet falls over) and the IDE falls back to an in-browser runtime. details below, it's the fun part

## Offline mode

the IDE doesn't just die when the websocket drops:

- **the whole IDE is cached** — a service worker precaches the app shell, so the editor opens even when the server is unreachable. navigations are network-first and fall back to the cached shell
- **your files move to the browser** — everything gets mirrored into OPFS (the browser's origin-private file system) as you work, so your code is already local when the connection dies
- **an in-browser runtime takes over** — on disconnect the IDE boots a [webcontainer](https://webcontainers.io) (stackblitz's in-browser node runtime) and mounts your OPFS files into it. the terminal switches from the container shell to a shell running *in the tab*, and the status bar flips to `Offline (WebContainer)` so you know who's actually executing your `rm -rf`
- **the engine is cached too** — the service worker caches the webcontainer runtime assets cache-first, so the sandbox can boot even with zero network
- **edits sync back** — write files all you want offline; when the connection returns, pending changes flush back to the container volume and the terminal reattaches to the server shell
- **even the ai chat is offline** — there's a built-in chat panel powered by [web-llm](https://github.com/mlc-ai/web-llm) running on webgpu. the weights (llama 3.2 1b by default, up to 3b if your gpu is feeling brave) download once, get cached in the browser, and then work with no internet at all. it can see whatever file you have open. it runs at the speed of your laptop, not my droplet, which is the whole point

so yeah: the server can be on fire and you can still edit files, run node, and ask a small llm why your code doesn't work.

## How it works

three services, one caddy:

```
dustdev.app            → static sveltekit bundle (landing + dashboard)
dustdev.app/api/*      → frontbackend (go + gin + postgres)
<slug>.dustdev.app     → caddy forward-auth → your project's IDE container
```

when you hit start, the frontbackend talks to the podman api socket, spawns an `idehost` container with your project's volume mounted at `/workspace`, and caddy reverse-proxies your subdomain to it. an idle reaper wakes up every minute and stops containers nobody's touched in a while. auth is email + password with a session cookie, and every project subdomain forward-auths through the api first, so opening someone else's IDE logged-out just bounces you to the login page.

## Repo layout

| dir | what it is |
|-----|------------|
| `frontfrontend/` | sveltekit + shadcn-svelte landing page and dashboard (bun) |
| `frontbackend/` | go/gin api — auth, project crud, podman wrangling, quotas, the idle reaper |
| `ide/` | the per-project IDE: go backend + monaco/xterm frontend, baked into a nix-based container image |
| `deploy/` | quadlets, caddyfile, deploy script — see [`deploy/README.md`](deploy/README.md) |
| `docs/` | a little one-page landing thing |
| `.github/` | ci (svelte-check, go vet, go test -race, image builds) and cd (pushes images to ghcr) |

## Limits

because i'm broke:

| thing | default |
|-------|---------|
| projects per user | 5 |
| running at once | 2 |
| ram per container | 2 gb |
| cpus per container | 2 |
| idle timeout | 10 min |
| session ttl | 30 days |

all configurable with env vars (`MAX_PROJECTS_PER_USER`, `MAX_RUNNING_PER_USER`, `CONTAINER_MEMORY_MB`, `CONTAINER_CPUS`, `IDLE_TIMEOUT_MINUTES`, ...) if you're running your own.

## Local dev

needs go, bun, and podman:

```bash
make dev
```

spins up a throwaway postgres in podman, the api on :8081, and the frontend dev server. run `make -C ide image` once if you want to actually start projects locally.

`make build` / `make images` build the production artifacts, `make check` runs the same checks ci does, `make clean` deletes your feelings (and the dev database).

## Deploy

everything runs as quadlets on one digitalocean droplet. caddy gets a wildcard cert via porkbun dns-01, cd pushes images to ghcr, and `deploy/deploy.sh` on the server pulls whatever's on main. IDE containers use `pull=always`, so IDE updates roll out the next time each project starts — no redeploy needed.

full guide in [`deploy/README.md`](deploy/README.md).

## Notes

- the idehost runtime is a nix flake (`ide/flake.nix`), so rebuilding the image is reproducible and adding tools to every project is a one-line change
- sessions and idle reaping run as background loops in the api; expired sessions get vacuumed hourly
- the ai chat needs webgpu; if your browser doesn't have it the panel just says `unsupported` and the rest of the IDE carries on without it
- the ai chat used to be an agent that could run commands in your container. it's been reverted. we don't talk about it (it's in the git history if you want to look)

## TODO (one day... but not today)

- [ ] github oauth
- [ ] project templates
- [ ] usage graphs so the dashboard looks fancier
- [ ] the ai agent again, but good this time
- [ ] custom domains, maybe
