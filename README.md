# dustdev

dustdev is a web IDE platform built at **Code AZ** in a 9-hour hackathon.

## Pitch

This project was built for **rural Arizonans** and anyone in Arizona dealing with unreliable or low-bandwidth internet.  
The core idea: keep coding productive even when connectivity is weak or drops.

## What it does

- Hosts projects in browser-accessible dev environments
- Serves each project at its own subdomain (`<slug>.dustdev.app`)
- Provides an online-first workflow with offline-friendly IDE behavior:
  - cached app assets
  - offline runtime fallback
  - local model/runtime caching in the browser

## Repository layout

- `frontfrontend/` — main app frontend
- `frontbackend/` — Go API/backend services
- `ide/` — IDE service (frontend + backend)
- `deploy/` — deployment config and runbooks

## Deployment

Production deployment notes live in `deploy/README.md`.
