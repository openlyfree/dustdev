<script lang="ts">
	import { onMount } from 'svelte';
	import Editor from '$lib/components/editor/editor.svelte';
	import Terminal from '$lib/components/terminal/terminal.svelte';
	import PreviewPanel from '$lib/components/preview/preview-panel.svelte';
	import ChatPanel from '$lib/components/ai/chat-panel.svelte';
	import FileTree from '$lib/components/explorer/file-tree.svelte';
	import NewFilePrompt from '$lib/components/explorer/new-file-prompt.svelte';
	import * as Resizable from '$lib/components/ui/resizable/index.js';
	import { FilePlus } from '@lucide/svelte';
	import { runtime } from '$lib/runtime';
	import { webContainerError, webContainerReady } from '$lib/runtime/stores';
	import { buildFileTree } from '$lib/utils/file-tree';
	import {
		decodeFileContent,
		isBinaryPath,
		isTextContent,
		languageForPath
	} from '$lib/utils/language';
	import { activeFile, fileSync, files, isConnected, runtimeMode } from '$lib/workspace/files';

	let saveTimer: ReturnType<typeof setTimeout> | null = null;
	let showNewFileDialog = $state(false);
	let newFileParentDir = $state('');
	let bottomPanel = $state<'terminal' | 'preview' | 'ai'>('terminal');
	let aiVisited = $state(false);

	function switchBottomPanel(panel: 'terminal' | 'preview' | 'ai') {
		bottomPanel = panel;
		if (panel === 'ai') aiVisited = true;
	}

	// node_modules syncs for offline persistence but stays out of the explorer
	const fileTree = $derived(
		buildFileTree($files.map((f) => f.path).filter((p) => !p.split('/').includes('node_modules')))
	);

	const statusLabel = $derived(
		$isConnected ? 'Online' : $webContainerReady ? 'Offline (WebContainer)' : 'Offline'
	);

	const editorState = $derived.by(() => {
		const path = $activeFile?.path;
		if (!path) {
			return {
				path: null,
				value: '',
				language: 'plaintext',
				readOnly: false
			};
		}

		const stored = $files.find((f) => f.path === path);
		if (!stored) {
			return {
				path,
				value: '',
				language: languageForPath(path),
				readOnly: true
			};
		}

		const fileData = stored.data ?? '';
		const isBinary = isBinaryPath(path) || (fileData.length > 0 && !isTextContent(fileData));

		if (isBinary) {
			return {
				path,
				value: `Binary file — not editable\n\n${path}`,
				language: 'plaintext',
				readOnly: true
			};
		}

		return {
			path,
			value: fileData ? decodeFileContent(fileData) : '',
			language: languageForPath(path),
			readOnly: false
		};
	});

	function scheduleSave(path: string, value: string) {
		if (editorState.readOnly) return;

		if (saveTimer) clearTimeout(saveTimer);
		saveTimer = setTimeout(() => {
			void fileSync.saveFile(path, value);
		}, 400);
	}

	function openNewFile(parentDir = '') {
		newFileParentDir = parentDir;
		showNewFileDialog = true;
	}

	function closeNewFile() {
		showNewFileDialog = false;
		newFileParentDir = '';
	}

	onMount(() => {
		runtime.init();

		return () => {
			if (saveTimer) clearTimeout(saveTimer);
			runtime.close();
		};
	});
</script>

<main class="h-screen w-screen overflow-hidden bg-background">
	<Resizable.PaneGroup direction="horizontal" class="h-full w-full">
		<Resizable.Pane defaultSize={20} minSize={15}>
			<div class="flex h-full flex-col border-r border-border p-4">
				<div class="mb-3 flex flex-col gap-1">
					<div class="flex items-center justify-between">
						<span class="text-xs font-semibold tracking-wider text-muted-foreground uppercase"
							>Explorer</span
						>
						<div class="flex items-center gap-2">
							<button
								type="button"
								class="flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
								title="New file"
								onclick={() => openNewFile('')}
							>
								<FilePlus class="size-3.5" />
								New file
							</button>
							<span class="flex items-center gap-1.5 text-[10px] text-muted-foreground">
								<span
									class="size-1.5 rounded-full {$isConnected
										? 'bg-green-400'
										: $webContainerReady
											? 'bg-amber-400'
											: 'bg-red-400'}"
								></span>
								{statusLabel}
							</span>
						</div>
					</div>
					{#if $runtimeMode === 'opfs-only'}
						<p class="text-[10px] text-muted-foreground">
							{$webContainerReady
								? 'Editing locally. Changes sync when back online.'
								: 'Local sandbox unavailable in this browser.'}
						</p>
					{/if}
					{#if $webContainerError}
						<p class="text-[10px] text-red-400">{$webContainerError}</p>
					{/if}
				</div>

				<div class="flex-1 overflow-y-auto">
					{#if showNewFileDialog}
						<NewFilePrompt
							parentDir={newFileParentDir}
							oncreated={closeNewFile}
							oncancel={closeNewFile}
						/>
					{/if}

					{#if fileTree.length === 0 && !showNewFileDialog}
						<p class="px-2 text-xs text-muted-foreground">
							{$isConnected
								? 'No files yet. Click "New file" to create one.'
								: $webContainerReady
									? 'No local files yet. Click "New file" to create one.'
									: 'Waiting for connection…'}
						</p>
					{:else if fileTree.length > 0}
						<FileTree nodes={fileTree} onNewFile={openNewFile} />
					{/if}
				</div>
			</div>
		</Resizable.Pane>

		<Resizable.Handle />

		<Resizable.Pane defaultSize={80}>
			<Resizable.PaneGroup direction="vertical" class="h-full w-full">
				<Resizable.Pane
					defaultSize={70}
					minSize={20}
					class="relative flex h-full w-full flex-col overflow-hidden"
				>
					{#if editorState.path}
						<div
							class="flex items-center justify-between border-b border-border bg-muted/40 px-4 py-1.5 font-mono text-xs text-muted-foreground"
						>
							<span>{editorState.path}</span>
							{#if !editorState.readOnly}
								<span class="text-[10px]">auto-save</span>
							{/if}
						</div>
					{/if}
					<div class="relative min-h-0 flex-1 overflow-hidden">
						{#key editorState.path}
							<Editor
								value={editorState.value}
								language={editorState.language}
								readOnly={editorState.readOnly}
								onchange={(value) => {
									if (editorState.path) {
										scheduleSave(editorState.path, value);
									}
								}}
							/>
						{/key}
					</div>
				</Resizable.Pane>

				<Resizable.Handle />

				<Resizable.Pane
					defaultSize={30}
					minSize={15}
					class="relative flex h-full w-full flex-col overflow-hidden"
				>
					<div
						class="flex items-center gap-1 border-b border-border bg-muted/40 px-3 py-1.5 text-xs text-muted-foreground"
					>
						<button
							type="button"
							class="rounded-md px-2 py-0.5 font-semibold tracking-wider uppercase transition-colors {bottomPanel ===
							'terminal'
								? 'bg-muted text-foreground'
								: 'hover:bg-muted/50 hover:text-foreground'}"
							onclick={() => switchBottomPanel('terminal')}
						>
							Terminal
							<span class="ml-1 font-normal normal-case">
								{$isConnected ? '(server)' : '(webcontainer)'}
							</span>
						</button>
						<button
							type="button"
							class="rounded-md px-2 py-0.5 font-semibold tracking-wider uppercase transition-colors {bottomPanel ===
							'preview'
								? 'bg-muted text-foreground'
								: 'hover:bg-muted/50 hover:text-foreground'}"
							onclick={() => switchBottomPanel('preview')}
						>
							Preview
							{#if $isConnected}
								<span class="ml-1 font-normal normal-case">(online)</span>
							{/if}
						</button>
						<button
							type="button"
							class="rounded-md px-2 py-0.5 font-semibold tracking-wider uppercase transition-colors {bottomPanel ===
							'ai'
								? 'bg-muted text-foreground'
								: 'hover:bg-muted/50 hover:text-foreground'}"
							onclick={() => switchBottomPanel('ai')}
						>
						AI
					</button>
					</div>
					<div class="min-h-0 flex-1">
						{#if bottomPanel === 'terminal'}
							<Terminal />
						{:else if bottomPanel === 'preview'}
							<PreviewPanel />
						{/if}
						{#if aiVisited}
							<div class="h-full" class:hidden={bottomPanel !== 'ai'}>
								<ChatPanel />
							</div>
						{/if}
					</div>
				</Resizable.Pane>
			</Resizable.PaneGroup>
		</Resizable.Pane>
	</Resizable.PaneGroup>
</main>
