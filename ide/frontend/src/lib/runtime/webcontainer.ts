import type { WebContainer, WebContainerProcess } from '@webcontainer/api';
import { WebContainer as WebContainerApi } from '@webcontainer/api';
import { webContainerPorts } from '$lib/runtime/stores';
import { initOPFS } from '$lib/utils/opfs';

type FileTree = Record<string, { file: { contents: string } } | { directory: FileTree }>;

let webcontainerInstance: WebContainer | null = null;
let bootPromise: Promise<WebContainer> | null = null;
let mounted = false;

export async function getWebContainer(): Promise<WebContainer> {
	if (webcontainerInstance) {
		return webcontainerInstance;
	}

	if (!bootPromise) {
		bootPromise = WebContainerApi.boot().then((instance) => {
			webcontainerInstance = instance;
			registerPortListeners(instance);
			return instance;
		});
	}

	return bootPromise;
}

function registerPortListeners(instance: WebContainer): void {
	const add = (port: number, url: string) => {
		webContainerPorts.update((current) => ({ ...current, [port]: url }));
	};
	const remove = (port: number) => {
		webContainerPorts.update((current) => {
			const next = { ...current };
			delete next[port];
			return next;
		});
	};

	instance.on('server-ready', (port, url) => add(port, url));
	instance.on('port', (port, type, url) => {
		if (type === 'open') {
			add(port, url);
		} else {
			remove(port);
		}
	});
}

async function buildTreeFromOPFS(
	dirHandle: FileSystemDirectoryHandle,
	path = ''
): Promise<FileTree> {
	const tree: FileTree = {};

	for await (const [name, handle] of dirHandle.entries()) {
		const currentPath = path ? `${path}/${name}` : name;

		if (handle.kind === 'file') {
			const file = await handle.getFile();
			const contents = await file.text();
			tree[name] = { file: { contents } };
		} else if (handle.kind === 'directory') {
			tree[name] = {
				directory: await buildTreeFromOPFS(handle, currentPath)
			};
		}
	}

	return tree;
}

export async function mountOPFSToWebContainer(): Promise<void> {
	const wc = await getWebContainer();
	const root = await initOPFS();
	const tree = await buildTreeFromOPFS(root);

	if (Object.keys(tree).length === 0) {
		await wc.mount({
			'README.md': {
				file: {
					contents:
						'# Offline workspace\n\nFiles you edit here are stored locally until you reconnect.\n'
				}
			}
		});
	} else {
		await wc.mount(tree);
	}

	mounted = true;
}

export async function writeToWebContainer(path: string, content: string): Promise<void> {
	const wc = await getWebContainer();
	if (!mounted) {
		await mountOPFSToWebContainer();
	}

	const dir = path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : '';
	if (dir) {
		await wc.fs.mkdir(dir, { recursive: true });
	}
	await wc.fs.writeFile(path, content);
}

export async function deleteFromWebContainer(path: string): Promise<void> {
	const wc = await getWebContainer();
	if (!mounted) return;

	try {
		await wc.fs.rm(path, { recursive: true });
	} catch {
		// File may not exist in the sandbox yet
	}
}

export async function spawnWebContainerShell(
	cols: number,
	rows: number
): Promise<WebContainerProcess> {
	const wc = await getWebContainer();
	if (!mounted) {
		await mountOPFSToWebContainer();
	}

	return wc.spawn('jsh', {
		terminal: { cols, rows }
	});
}

export function isWebContainerMounted(): boolean {
	return mounted;
}

export async function teardownWebContainer(): Promise<void> {
	if (webcontainerInstance) {
		await webcontainerInstance.teardown();
	}
	webcontainerInstance = null;
	bootPromise = null;
	mounted = false;
	webContainerPorts.set({});
}
