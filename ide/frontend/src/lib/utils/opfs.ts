import type { FileMessage } from '$lib/workspace/files';

export async function initOPFS(): Promise<FileSystemDirectoryHandle> {
	if (!navigator.storage?.getDirectory) {
		throw new Error('Origin Private File System is not supported in this browser.');
	}
	try {
		await navigator.storage.persist();
	} catch {
		// Persistence is best-effort; OPFS still works without it
	}
	return navigator.storage.getDirectory();
}

export function bytesToBase64(bytes: Uint8Array): string {
	let binary = '';
	for (const byte of bytes) {
		binary += String.fromCharCode(byte);
	}
	return btoa(binary);
}

export async function writeToOPFS(path: string, content: string): Promise<void> {
	const rootDir = await initOPFS();
	const segments = path.split('/').filter(Boolean);

	if (segments.length === 0) {
		throw new Error('Invalid path provided for writing.');
	}

	const fileName = segments.pop()!;
	let currentDir = rootDir;

	for (const segment of segments) {
		currentDir = await currentDir.getDirectoryHandle(segment, { create: true });
	}

	const fileHandle = await currentDir.getFileHandle(fileName, { create: true });
	const writable = await fileHandle.createWritable();
	await writable.write(content);
	await writable.close();
}

export async function readFromOPFS(path: string): Promise<string> {
	const rootDir = await initOPFS();
	const segments = path.split('/').filter(Boolean);

	if (segments.length === 0) {
		throw new Error('Invalid path provided for reading.');
	}

	const fileName = segments.pop()!;
	let currentDir = rootDir;

	for (const segment of segments) {
		currentDir = await currentDir.getDirectoryHandle(segment, { create: false });
	}

	const fileHandle = await currentDir.getFileHandle(fileName, { create: false });
	const file = await fileHandle.getFile();
	return file.text();
}

export async function deleteFromOPFS(path: string): Promise<void> {
	const rootDir = await initOPFS();
	const segments = path.split('/').filter(Boolean);

	if (segments.length === 0) {
		throw new Error('Invalid path provided for deletion.');
	}

	const targetName = segments.pop()!;
	let currentDir = rootDir;

	for (const segment of segments) {
		currentDir = await currentDir.getDirectoryHandle(segment, { create: false });
	}

	await currentDir.removeEntry(targetName, { recursive: true });
}

export async function listOPFSFiles(
	dirHandle?: FileSystemDirectoryHandle,
	prefix = ''
): Promise<FileMessage[]> {
	const root = dirHandle ?? (await initOPFS());
	const result: FileMessage[] = [];

	for await (const [name, handle] of root.entries()) {
		const path = prefix ? `${prefix}/${name}` : name;

		if (handle.kind === 'file') {
			const file = await handle.getFile();
			const bytes = new Uint8Array(await file.arrayBuffer());
			result.push({ path, data: bytesToBase64(bytes) });
		} else if (handle.kind === 'directory') {
			result.push(...(await listOPFSFiles(handle, path)));
		}
	}

	return result;
}
