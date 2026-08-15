<script lang="ts">
	import { onMount } from 'svelte';
	import loader from '@monaco-editor/loader';
	import * as monacoEditor from 'monaco-editor';
	import type * as Monaco from 'monaco-editor';

	loader.config({ monaco: monacoEditor });

	interface Props {
		value?: string;
		language?: string;
		theme?: string;
		readOnly?: boolean;
		options?: Monaco.editor.IStandaloneEditorConstructionOptions;
		onchange?: (value: string) => void;
	}

	let {
		value = '',
		language = 'plaintext',
		theme = 'catppuccin-mocha',
		readOnly = false,
		options = {},
		onchange
	}: Props = $props();

	let container: HTMLDivElement;
	let editor: Monaco.editor.IStandaloneCodeEditor | undefined;
	let monaco: typeof Monaco | undefined;
	let resizeObserver: ResizeObserver | undefined;
	let ready = false;

	onMount(() => {
		let isMounted = true;

		loader.init().then((monacoInstance) => {
			if (!isMounted || !container) return;
			monaco = monacoInstance;

			monacoInstance.editor.defineTheme('catppuccin-mocha', {
				base: 'vs-dark',
				inherit: true,
				rules: [
					{ token: '', foreground: 'cdd6f4', background: '181825' },
					{ token: 'comment', foreground: '6c7086', fontStyle: 'italic' },
					{ token: 'keyword', foreground: 'cba6f7', fontStyle: 'bold' },
					{ token: 'string', foreground: 'a6e3a1' },
					{ token: 'number', foreground: 'fab387' },
					{ token: 'type', foreground: 'f9e2af' },
					{ token: 'function', foreground: '89b4fa' },
					{ token: 'variable', foreground: 'cdd6f4' }
				],
				colors: {
					'editor.background': '#181825',
					'editor.foreground': '#cdd6f4',
					'editorCursor.foreground': '#cba6f7',
					'editor.lineHighlightBackground': '#31324450',
					'editorLineNumber.foreground': '#45475a',
					'editorLineNumber.activeForeground': '#cba6f7',
					'editor.selectionBackground': '#585b7050',
					'editor.inactiveSelectionBackground': '#45475a30'
				}
			});

			const createdEditor = monacoInstance.editor.create(container, {
				value,
				language,
				theme,
				readOnly,
				automaticLayout: true,
				minimap: { enabled: true },
				fontSize: 13,
				scrollBeyondLastLine: false,
				smoothScrolling: true,
				cursorBlinking: 'smooth',
				padding: { top: 12, bottom: 12 },
				...options
			});
			editor = createdEditor;
			ready = true;

			createdEditor.onDidChangeModelContent(() => {
				if (readOnly) return;
				const currentVal = createdEditor.getValue() ?? '';
				onchange?.(currentVal);
			});

			resizeObserver = new ResizeObserver(() => {
				editor?.layout();
			});
			resizeObserver.observe(container);
		});

		return () => {
			isMounted = false;
			ready = false;
			resizeObserver?.disconnect();
			editor?.dispose();
		};
	});

	$effect(() => {
		if (!ready || !editor) return;
		const current = editor.getValue();
		if (current !== value) {
			editor.setValue(value);
		}
	});

	$effect(() => {
		if (editor && monaco && language) {
			const model = editor.getModel();
			if (model) {
				monaco.editor.setModelLanguage(model, language);
			}
		}
	});

	$effect(() => {
		if (editor && monaco && theme) {
			monaco.editor.setTheme(theme);
		}
	});

	$effect(() => {
		if (editor) {
			editor.updateOptions({ readOnly });
		}
	});
</script>

<div bind:this={container} class="h-full w-full min-h-0 min-w-0 overflow-hidden"></div>
