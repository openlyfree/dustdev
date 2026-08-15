/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true" />
/// <reference lib="esnext" />
/// <reference lib="webworker" />

import { build, files, prerendered, version } from '$service-worker';

const sw = self as unknown as ServiceWorkerGlobalScope;

const ASSET_CACHE = `app-${version}`;
const RUNTIME_CACHE = 'webcontainer-runtime';

const APP_ASSETS = [...build, ...files, ...prerendered];

// Hosts that serve the WebContainer engine; responses are CORS-enabled and cacheable
const WEBCONTAINER_HOSTS = [
	'staticblitz.com',
	'webcontainer.io',
	'webcontainer-api.io',
	'stackblitz.com'
];

// Server-backed endpoints that must never be intercepted
const PASSTHROUGH_PATHS = ['/file', '/term', '/healthz', '/preview'];

function isWebContainerAsset(url: URL): boolean {
	return WEBCONTAINER_HOSTS.some(
		(host) => url.hostname === host || url.hostname.endsWith(`.${host}`)
	);
}

function cacheable(response: Response): boolean {
	return response.ok || response.type === 'opaque';
}

sw.addEventListener('install', (event) => {
	event.waitUntil(
		caches
			.open(ASSET_CACHE)
			.then((cache) => cache.addAll(APP_ASSETS))
			.then(() => sw.skipWaiting())
	);
});

sw.addEventListener('activate', (event) => {
	event.waitUntil(
		(async () => {
			const keys = await caches.keys();
			await Promise.all(
				keys
					.filter((key) => key.startsWith('app-') && key !== ASSET_CACHE)
					.map((key) => caches.delete(key))
			);
			await sw.clients.claim();
		})()
	);
});

sw.addEventListener('fetch', (event) => {
	const { request } = event;
	if (request.method !== 'GET') return;

	const url = new URL(request.url);

	if (url.origin === sw.location.origin) {
		if (PASSTHROUGH_PATHS.some((path) => url.pathname.startsWith(path))) return;

		// SPA navigation: network-first, fall back to the cached shell offline
		if (request.mode === 'navigate') {
			event.respondWith(
				(async () => {
					try {
						return await fetch(request);
					} catch {
						const shell = await caches.match('/');
						return shell ?? new Response('Offline', { status: 503 });
					}
				})()
			);
			return;
		}

		// Same-origin assets: cache-first, fill cache from network
		event.respondWith(
			(async () => {
				const cached = await caches.match(request);
				if (cached) return cached;
				const response = await fetch(request);
				if (cacheable(response)) {
					const cache = await caches.open(ASSET_CACHE);
					await cache.put(request, response.clone());
				}
				return response;
			})()
		);
		return;
	}

	// WebContainer engine assets: cache-first so the sandbox can boot offline
	if (isWebContainerAsset(url)) {
		event.respondWith(
			(async () => {
				const cached = await caches.match(request);
				if (cached) return cached;
				const response = await fetch(request);
				if (cacheable(response)) {
					const cache = await caches.open(RUNTIME_CACHE);
					await cache.put(request, response.clone());
				}
				return response;
			})()
		);
	}
});
