import { get, writable } from 'svelte/store';
import { deleteFromOPFS, writeToOPFS } from '$lib/utils/opfs';
import { encodeFileContent } from '$lib/utils/language';

export interface FileMessage {
	path: string;
	data: string;
}

export type RuntimeMode = 'container' | 'opfs-only';

export const isConnected = writable(false);
export const runtimeMode = writable<RuntimeMode>('container');
export const files = writable<FileMessage[]>([]);
export const activeFile = writable<FileMessage | null>(null);

const recentLocalWrites = new Map<string, number>();
const pendingServerSync = new Set<string>();

type FileSyncCallbacks = {
	onOnline?: () => void;
	onOffline?: () => void;
	onBootstrapComplete?: () => void;
	onFileSaved?: (path: string, content: string) => void;
	onFileDeleted?: (path: string) => void;
};

function shouldIgnoreRemoteUpdate(path: string): boolean {
	const wroteAt = recentLocalWrites.get(path);
	if (wroteAt && Date.now() - wroteAt < 800) {
		return true;
	}
	return false;
}

function decodeToText(base64: string): string | null {
	try {
		return new TextDecoder().decode(Uint8Array.from(atob(base64), (c) => c.charCodeAt(0)));
	} catch {
		return null;
	}
}

function normalizePath(path: string): string {
	return path.replace(/\\/g, '/');
}

class FileSyncManager {
	private ws: WebSocket | null = null;
	private callbacks: FileSyncCallbacks = {};
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private bootstrapReceived = false;

	init(callbacks: FileSyncCallbacks = {}) {
		this.callbacks = callbacks;
		this.connect();
	}

	private connect() {
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}

		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		this.ws = new WebSocket(`${protocol}//${window.location.host}/file`);

		this.ws.onopen = () => {
			isConnected.set(true);
			runtimeMode.set('container');
			this.bootstrapReceived = false;
			this.callbacks.onOnline?.();
		};

		const goOffline = () => {
			if (!get(isConnected) && get(runtimeMode) === 'opfs-only') {
				return;
			}
			isConnected.set(false);
			runtimeMode.set('opfs-only');
			this.callbacks.onOffline?.();
			this.scheduleReconnect();
		};

		this.ws.onclose = goOffline;
		this.ws.onerror = goOffline;

		this.ws.onmessage = async (event) => {
			const msg = JSON.parse(event.data);
			const incoming: FileMessage[] = Array.isArray(msg) ? msg : [msg];
			const isBootstrap = Array.isArray(msg);

			for (const item of incoming) {
				const path = normalizePath(item.path);
				const normalized = { ...item, path };

				if (shouldIgnoreRemoteUpdate(path)) {
					continue;
				}

				if (normalized.data === 'delete') {
					try {
						await deleteFromOPFS(path);
					} catch {
						// OPFS may not have the file yet
					}
					this.callbacks.onFileDeleted?.(path);
				} else {
					const text = decodeToText(normalized.data);
					if (text !== null) {
						try {
							await writeToOPFS(path, text);
						} catch {
							// Binary or OPFS unavailable
						}
					}
				}
			}

			files.update((current) => {
				let updated = [...current];
				for (const item of incoming) {
					const path = normalizePath(item.path);
					const normalized = { ...item, path };

					if (shouldIgnoreRemoteUpdate(path)) {
						continue;
					}

					const index = updated.findIndex((f) => f.path === path);
					if (normalized.data === 'delete') {
						updated = updated.filter((f) => f.path !== path);
						if (get(activeFile)?.path === path) {
							activeFile.set(null);
						}
					} else if (index >= 0) {
						updated[index] = normalized;
					} else {
						updated.push(normalized);
					}
				}
				return updated;
			});

			activeFile.update((current) => {
				if (!current) return current;
				const path = normalizePath(current.path);
				const refreshed = get(files).find((f) => f.path === path);
				return refreshed ?? current;
			});

			if (isBootstrap && !this.bootstrapReceived) {
				this.bootstrapReceived = true;
				this.callbacks.onBootstrapComplete?.();
			}
		};
	}

	private scheduleReconnect() {
		if (this.reconnectTimer) return;
		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = null;
			this.connect();
		}, 3000);
	}

	async saveFile(path: string, content: string) {
		const normalizedPath = normalizePath(path);
		recentLocalWrites.set(normalizedPath, Date.now());

		try {
			await writeToOPFS(normalizedPath, content);
		} catch {
			// OPFS optional when unavailable
		}

		const encoded = encodeFileContent(content);
		files.update((current) => {
			const index = current.findIndex((f) => f.path === normalizedPath);
			const entry = { path: normalizedPath, data: encoded };
			if (index >= 0) {
				const updated = [...current];
				updated[index] = entry;
				return updated;
			}
			return [...current, entry];
		});

		activeFile.update((current) =>
			current?.path === normalizedPath ? { path: normalizedPath, data: encoded } : current
		);

		if (this.ws?.readyState === WebSocket.OPEN) {
			this.ws.send(JSON.stringify({ path: normalizedPath, data: encoded }));
		} else {
			pendingServerSync.add(normalizedPath);
		}

		await this.callbacks.onFileSaved?.(normalizedPath, content);
	}

	async createFile(path: string): Promise<{ ok: true } | { ok: false; error: string }> {
		const normalizedPath = normalizePath(path.trim()).replace(/^\/+/, '');
		if (!normalizedPath) {
			return { ok: false, error: 'Enter a file name' };
		}
		if (normalizedPath.includes('..') || normalizedPath.endsWith('/')) {
			return { ok: false, error: 'Invalid path' };
		}

		if (get(files).some((f) => f.path === normalizedPath)) {
			return { ok: false, error: 'File already exists' };
		}

		await this.saveFile(normalizedPath, '');
		const created = get(files).find((f) => f.path === normalizedPath);
		if (created) {
			activeFile.set(created);
		}
		return { ok: true };
	}

	async flushPendingSync() {
		if (!this.ws || this.ws.readyState !== WebSocket.OPEN || pendingServerSync.size === 0) {
			return;
		}

		for (const path of pendingServerSync) {
			const file = get(files).find((f) => f.path === path);
			if (file) {
				this.ws.send(JSON.stringify({ path, data: file.data }));
			}
		}
		pendingServerSync.clear();
	}

	close() {
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		this.ws?.close();
	}
}

export const fileSync = new FileSyncManager();
