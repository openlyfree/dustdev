<script lang="ts">
	import type { TreeNode } from '$lib/utils/file-tree';
	import { ChevronRight, File, FilePlus, Folder } from '@lucide/svelte';
	import { get } from 'svelte/store';
	import { activeFile, files } from '$lib/workspace/files';
	import Self from './file-tree.svelte';

	interface Props {
		nodes: TreeNode[];
		depth?: number;
		onNewFile?: (parentDir: string) => void;
	}

	let { nodes, depth = 0, onNewFile }: Props = $props();

	const expanded = $state<Record<string, boolean>>({});

	function toggle(path: string) {
		expanded[path] = !expanded[path];
	}

	function selectFile(path: string) {
		const file = get(files).find((f) => f.path === path);
		if (file) {
			activeFile.set(file);
		}
	}

	function startNewFileInDir(event: MouseEvent, dir: string) {
		event.stopPropagation();
		expanded[dir] = true;
		onNewFile?.(dir);
	}
</script>

<ul class="space-y-0.5">
	{#each nodes as node (node.path)}
		<li>
			{#if node.isDir}
				<div
					class="group flex w-full items-center rounded hover:bg-muted"
					style={`padding-left: ${depth * 12 + 8}px`}
				>
					<button
						type="button"
						class="flex min-w-0 flex-1 items-center gap-1 py-1 pr-1 text-left text-sm text-muted-foreground"
						onclick={() => toggle(node.path)}
					>
						<ChevronRight
							class="size-3.5 shrink-0 transition-transform {expanded[node.path] ? 'rotate-90' : ''}"
						/>
						<Folder class="size-3.5 shrink-0" />
						<span class="truncate font-mono">{node.name}</span>
					</button>
					{#if onNewFile}
						<button
							type="button"
							class="mr-1 rounded p-0.5 text-muted-foreground hover:bg-background hover:text-foreground"
							title="New file in {node.name}"
							aria-label="New file in {node.name}"
							onclick={(event) => startNewFileInDir(event, node.path)}
						>
							<FilePlus class="size-3.5" />
						</button>
					{/if}
				</div>
				{#if expanded[node.path]}
					<Self nodes={node.children} depth={depth + 1} {onNewFile} />
				{/if}
			{:else}
				<button
					type="button"
					class="flex w-full items-center gap-1.5 rounded px-2 py-1 text-left text-sm font-mono transition-colors hover:bg-muted {$activeFile?.path ===
					node.path
						? 'bg-muted text-foreground font-medium'
						: 'text-muted-foreground'}"
					style={`padding-left: ${depth * 12 + 24}px`}
					onclick={() => selectFile(node.path)}
				>
					<File class="size-3.5 shrink-0" />
					<span class="truncate">{node.name}</span>
				</button>
			{/if}
		</li>
	{/each}
</ul>
