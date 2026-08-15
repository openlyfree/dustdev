<script lang="ts">
	import { onMount } from 'svelte';
	import '@xterm/xterm/css/xterm.css';
	import { spawnWebContainerShell } from '$lib/runtime/webcontainer';
	import { terminalBackend } from '$lib/runtime/stores';

	let terminalContainer: HTMLDivElement;
	let term: import('@xterm/xterm').Terminal | undefined;
	let fitAddon: import('@xterm/addon-fit').FitAddon | undefined;
	let socket: WebSocket | undefined;
	let wcCleanup: (() => void) | undefined;
	let resizeObserver: ResizeObserver | undefined;
	let resizeAnimationFrame: number | null = null;
	let activeBackend: 'server' | 'webcontainer' | null = null;

	const catppuccinMochaTheme = {
		background: '#1e1e2e',
		foreground: '#cdd6f4',
		cursor: '#cba6f7',
		selectionBackground: '#585b70',
		black: '#45475a',
		red: '#f38ba8',
		green: '#a6e3a1',
		yellow: '#f9e2af',
		blue: '#89b4fa',
		magenta: '#cba6f7',
		cyan: '#94e2d5',
		white: '#bac2de',
		brightBlack: '#585b70',
		brightRed: '#f38ba8',
		brightGreen: '#a6e3a1',
		brightYellow: '#f9e2af',
		brightBlue: '#89b4fa',
		brightMagenta: '#cba6f7',
		brightCyan: '#94e2d5',
		brightWhite: '#a6adc8'
	};

	function disconnect() {
		socket?.close();
		socket = undefined;
		wcCleanup?.();
		wcCleanup = undefined;
		activeBackend = null;
	}

	async function connectServerTerminal() {
		if (!term || !fitAddon || activeBackend === 'server') return;

		disconnect();
		activeBackend = 'server';
		term.clear();
		term.writeln('\x1b[32m[Connected to server shell]\x1b[0m\r\n');

		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		socket = new WebSocket(`${protocol}//${window.location.host}/term`);

		const sendResize = (cols: number, rows: number) => {
			if (socket?.readyState === WebSocket.OPEN && cols > 0 && rows > 0) {
				socket.send(JSON.stringify({ type: 'resize', cols, rows }));
			}
		};

		socket.onopen = () => {
			if (!term || !fitAddon) return;
			fitAddon.fit();
			sendResize(term.cols, term.rows);
		};

		socket.onmessage = (event: MessageEvent) => {
			if (!term) return;
			if (event.data instanceof Blob) {
				event.data.text().then((text) => term?.write(text));
			} else {
				term.write(event.data);
			}
		};

		socket.onclose = () => {
			if (!term || activeBackend !== 'server') return;
			term.writeln('\r\n\x1b[33m[Server shell disconnected — switching to offline shell]\x1b[0m');
		};

		socket.onerror = () => {
			if (!term || activeBackend !== 'server') return;
			term.writeln('\r\n\x1b[31m[Failed to connect to server shell]\x1b[0m');
		};

		const dataDisposable = term.onData((data: string) => {
			if (socket?.readyState === WebSocket.OPEN) {
				socket.send(data);
			}
		});

		const resizeDisposable = term.onResize(({ cols, rows }) => {
			sendResize(cols, rows);
		});

		const previousCleanup = wcCleanup;
		wcCleanup = () => {
			dataDisposable.dispose();
			resizeDisposable.dispose();
			previousCleanup?.();
		};
	}

	async function connectWebContainerTerminal() {
		if (!term || !fitAddon || activeBackend === 'webcontainer') return;

		disconnect();
		activeBackend = 'webcontainer';
		term.clear();
		term.writeln('\x1b[33m[Offline — starting WebContainer shell]\x1b[0m\r\n');

		try {
			fitAddon.fit();
			const process = await spawnWebContainerShell(term.cols, term.rows);

			const outputReader = process.output.getReader();
			let reading = true;

			const readOutput = async () => {
				while (reading) {
					const { value, done } = await outputReader.read();
					if (done) break;
					term?.write(value);
				}
			};
			void readOutput();

			const inputWriter = process.input.getWriter();
			const dataDisposable = term.onData((data: string) => {
				void inputWriter.write(data);
			});

			const resizeDisposable = term.onResize(({ cols, rows }) => {
				process.resize({ cols, rows });
			});

			wcCleanup = () => {
				reading = false;
				void outputReader.cancel();
				void inputWriter.close();
				dataDisposable.dispose();
				resizeDisposable.dispose();
				process.kill();
			};
		} catch (error) {
			activeBackend = null;
			const message = error instanceof Error ? error.message : 'unknown error';
			term.writeln(`\r\n\x1b[31m[WebContainer shell failed: ${message}]\x1b[0m`);
		}
	}

	async function switchBackend(backend: 'server' | 'webcontainer') {
		if (backend === 'server') {
			await connectServerTerminal();
		} else {
			await connectWebContainerTerminal();
		}
	}

	onMount(() => {
		let isMounted = true;
		let unsubscribe: (() => void) | undefined;

		Promise.all([import('@xterm/xterm'), import('@xterm/addon-fit')]).then(
			([{ Terminal }, { FitAddon }]) => {
				if (!isMounted || !terminalContainer) return;

				term = new Terminal({ cursorBlink: true, theme: catppuccinMochaTheme, fontSize: 13 });
				fitAddon = new FitAddon();
				term.loadAddon(fitAddon);
				term.open(terminalContainer);
				fitAddon.fit();

				unsubscribe = terminalBackend.subscribe((backend) => {
					void switchBackend(backend);
				});

				resizeObserver = new ResizeObserver(() => {
					if (resizeAnimationFrame) cancelAnimationFrame(resizeAnimationFrame);
					resizeAnimationFrame = requestAnimationFrame(() => {
						if (
							!terminalContainer ||
							terminalContainer.clientWidth === 0 ||
							terminalContainer.clientHeight === 0
						) {
							return;
						}
						try {
							fitAddon?.fit();
						} catch (e) {
							console.error('Failed to fit terminal:', e);
						}
					});
				});
				resizeObserver.observe(terminalContainer);
			}
		);

		return () => {
			isMounted = false;
			unsubscribe?.();
			if (resizeAnimationFrame) cancelAnimationFrame(resizeAnimationFrame);
			resizeObserver?.disconnect();
			disconnect();
			term?.dispose();
		};
	});
</script>

<div
	bind:this={terminalContainer}
	class="h-full w-full overflow-hidden rounded-md border border-input bg-background p-0"
></div>
