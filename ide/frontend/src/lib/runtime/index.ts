import { get } from 'svelte/store';
import {
	deleteFromWebContainer,
	mountOPFSToWebContainer,
	writeToWebContainer
} from '$lib/runtime/webcontainer';
import { terminalBackend, webContainerError, webContainerReady } from '$lib/runtime/stores';
import { startPortPolling, stopPortPolling } from '$lib/runtime/ports';
import { listOPFSFiles } from '$lib/utils/opfs';
import { fileSync, files, isConnected, runtimeMode } from '$lib/workspace/files';

class RuntimeManager {
	private offlineActivated = false;

	init() {
		fileSync.init({
			onOnline: () => void this.activateOnline(),
			onOffline: () => void this.activateOffline(),
			onBootstrapComplete: () => void this.syncWebContainerFromOPFS(),
			onFileSaved: (path, content) => void this.onFileSaved(path, content),
			onFileDeleted: (path) => void this.onFileDeleted(path)
		});
	}

	async activateOnline() {
		this.offlineActivated = false;
		terminalBackend.set('server');
		webContainerReady.set(false);
		startPortPolling();

		await fileSync.flushPendingSync();
	}

	async activateOffline() {
		stopPortPolling();

		if (this.offlineActivated && get(runtimeMode) === 'opfs-only') {
			terminalBackend.set('webcontainer');
			return;
		}

		this.offlineActivated = true;
		runtimeMode.set('opfs-only');
		terminalBackend.set('webcontainer');

		try {
			const localFiles = await listOPFSFiles();
			if (localFiles.length > 0 && get(files).length === 0) {
				files.set(localFiles);
			}

			await mountOPFSToWebContainer();
			webContainerReady.set(true);
			webContainerError.set(null);
		} catch (error) {
			const message = error instanceof Error ? error.message : 'WebContainer failed to start';
			webContainerError.set(message);
			webContainerReady.set(false);
			console.warn('Offline runtime unavailable:', error);
		}
	}

	async syncWebContainerFromOPFS() {
		if (!get(isConnected)) return;

		try {
			await mountOPFSToWebContainer();
			webContainerReady.set(true);
			webContainerError.set(null);
		} catch (error) {
			console.warn('WebContainer sync skipped:', error);
		}
	}

	async onFileSaved(path: string, content: string) {
		if (!get(isConnected)) {
			try {
				await writeToWebContainer(path, content);
				webContainerReady.set(true);
				webContainerError.set(null);
			} catch (error) {
				console.warn('Failed to write to WebContainer:', error);
			}
		}
	}

	async onFileDeleted(path: string) {
		if (!get(isConnected)) {
			await deleteFromWebContainer(path);
		}
	}

	close() {
		stopPortPolling();
		fileSync.close();
	}
}

export const runtime = new RuntimeManager();
