import { writable } from 'svelte/store';

export type TerminalBackend = 'server' | 'webcontainer';

export const terminalBackend = writable<TerminalBackend>('server');
export const webContainerReady = writable(false);
export const webContainerError = writable<string | null>(null);

// Ports listening inside the WebContainer sandbox, keyed by port with their preview URL
export const webContainerPorts = writable<Record<number, string>>({});
