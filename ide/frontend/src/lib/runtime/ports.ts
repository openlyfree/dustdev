import { derived, get, writable } from 'svelte/store';
import { webContainerPorts } from '$lib/runtime/stores';
import { isConnected, runtimeMode } from '$lib/workspace/files';

const serverPorts = writable<number[]>([]);

export const selectedPreviewPort = writable<number | null>(null);

// Server-reported ports when online plus WebContainer ports from the in-browser sandbox
export const openPorts = derived(
	[serverPorts, webContainerPorts],
	([$serverPorts, $webContainerPorts]) => {
		const sandboxPorts = Object.keys($webContainerPorts).map(Number);
		return [...new Set([...$serverPorts, ...sandboxPorts])].sort((a, b) => a - b);
	}
);

// Keep the selection pointed at a live port
openPorts.subscribe(($openPorts) => {
	const selected = get(selectedPreviewPort);
	if (selected !== null && !$openPorts.includes(selected)) {
		selectedPreviewPort.set($openPorts[0] ?? null);
	} else if (selected === null && $openPorts.length > 0) {
		selectedPreviewPort.set($openPorts[0]);
	}
});

let pollTimer: ReturnType<typeof setInterval> | null = null;

async function fetchPorts(): Promise<number[]> {
	const res = await fetch('/ports');
	if (!res.ok) {
		return [];
	}
	const data = (await res.json()) as { ports?: number[] };
	return data.ports ?? [];
}

export function startPortPolling() {
	stopPortPolling();

	const poll = async () => {
		if (!get(isConnected)) {
			serverPorts.set([]);
			return;
		}

		try {
			serverPorts.set(await fetchPorts());
		} catch {
			serverPorts.set([]);
		}
	};

	void poll();
	pollTimer = setInterval(() => void poll(), 2000);
}

export function stopPortPolling() {
	if (pollTimer) {
		clearInterval(pollTimer);
		pollTimer = null;
	}
	serverPorts.set([]);
}

export function previewUrl(port: number): string {
	if (get(runtimeMode) === 'opfs-only') {
		const sandboxUrl = get(webContainerPorts)[port];
		if (sandboxUrl) return sandboxUrl;
	}
	return `/preview/${port}/`;
}
