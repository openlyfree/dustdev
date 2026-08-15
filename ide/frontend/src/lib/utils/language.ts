const EXTENSION_LANGUAGE: Record<string, string> = {
	ts: 'typescript',
	tsx: 'typescript',
	js: 'javascript',
	jsx: 'javascript',
	mjs: 'javascript',
	cjs: 'javascript',
	svelte: 'html',
	vue: 'html',
	html: 'html',
	htm: 'html',
	css: 'css',
	scss: 'scss',
	less: 'less',
	json: 'json',
	md: 'markdown',
	markdown: 'markdown',
	yml: 'yaml',
	yaml: 'yaml',
	toml: 'ini',
	ini: 'ini',
	xml: 'xml',
	svg: 'xml',
	go: 'go',
	rs: 'rust',
	py: 'python',
	rb: 'ruby',
	java: 'java',
	kt: 'kotlin',
	c: 'c',
	h: 'c',
	cpp: 'cpp',
	hpp: 'cpp',
	cs: 'csharp',
	sh: 'shell',
	bash: 'shell',
	zsh: 'shell',
	sql: 'sql',
	dockerfile: 'dockerfile',
	makefile: 'makefile',
	env: 'ini',
	gitignore: 'plaintext',
	txt: 'plaintext'
};

const BINARY_EXTENSIONS = new Set([
	'png',
	'jpg',
	'jpeg',
	'gif',
	'webp',
	'ico',
	'bmp',
	'pdf',
	'zip',
	'gz',
	'tar',
	'7z',
	'rar',
	'exe',
	'dll',
	'so',
	'dylib',
	'woff',
	'woff2',
	'ttf',
	'eot',
	'mp3',
	'mp4',
	'webm',
	'avi',
	'mov',
	'wasm'
]);

export function languageForPath(path: string): string {
	const base = path.split('/').pop() ?? path;
	const lower = base.toLowerCase();

	if (lower === 'dockerfile' || lower.startsWith('dockerfile.')) {
		return 'dockerfile';
	}
	if (lower === 'makefile') {
		return 'makefile';
	}

	const dot = lower.lastIndexOf('.');
	if (dot === -1) {
		return 'plaintext';
	}

	const ext = lower.slice(dot + 1);
	return EXTENSION_LANGUAGE[ext] ?? 'plaintext';
}

export function isBinaryPath(path: string): boolean {
	const base = path.split('/').pop() ?? path;
	const dot = base.lastIndexOf('.');
	if (dot === -1) {
		return false;
	}
	return BINARY_EXTENSIONS.has(base.slice(dot + 1).toLowerCase());
}

export function isTextContent(base64: string): boolean {
	try {
		const binary = atob(base64);
		const sample = binary.slice(0, 8192);
		for (let i = 0; i < sample.length; i++) {
			const code = sample.charCodeAt(i);
			if (code === 0) {
				return false;
			}
		}
		return true;
	} catch {
		return false;
	}
}

export function decodeFileContent(base64: string): string {
	const binary = atob(base64);
	const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
	return new TextDecoder('utf-8', { fatal: false }).decode(bytes);
}

export function encodeFileContent(content: string): string {
	const bytes = new TextEncoder().encode(content);
	let binary = '';
	for (const byte of bytes) {
		binary += String.fromCharCode(byte);
	}
	return btoa(binary);
}
