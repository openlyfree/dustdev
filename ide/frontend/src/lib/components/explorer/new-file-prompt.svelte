<script lang="ts">
	import { tick } from 'svelte';
	import { fileSync } from '$lib/workspace/files';

	interface Props {
		parentDir?: string;
		oncreated?: () => void;
		oncancel?: () => void;
	}

	let { parentDir = '', oncreated, oncancel }: Props = $props();

	let name = $state('');
	let error = $state('');
	let submitting = $state(false);
	let inputEl = $state<HTMLInputElement | null>(null);

	const placeholder = $derived(parentDir ? 'new-file.ts' : 'path/to/file.ts');

	$effect(() => {
		void tick().then(() => inputEl?.focus());
	});

	async function submit() {
		if (submitting) return;
		error = '';
		const trimmed = name.trim();
		if (!trimmed) {
			error = 'Enter a file name';
			return;
		}

		submitting = true;
		try {
			const path = parentDir ? `${parentDir}/${trimmed}` : trimmed;
			const result = await fileSync.createFile(path);
			if (!result.ok) {
				error = result.error;
				return;
			}
			oncreated?.();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create file';
		} finally {
			submitting = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			void submit();
		} else if (event.key === 'Escape') {
			event.preventDefault();
			oncancel?.();
		}
	}
</script>

<div class="mb-3 space-y-2 rounded-md border border-primary/30 bg-muted/50 p-3">
	<p class="text-xs font-medium text-foreground">
		{#if parentDir}
			New file in <span class="font-mono">{parentDir}/</span>
		{:else}
			Create new file
		{/if}
	</p>
	<input
		bind:this={inputEl}
		bind:value={name}
		type="text"
		class="border-input bg-background focus-visible:border-ring focus-visible:ring-ring/50 h-8 w-full rounded-lg border px-2.5 font-mono text-xs outline-none focus-visible:ring-2"
		{placeholder}
		disabled={submitting}
		onkeydown={handleKeydown}
	/>
	<div class="flex gap-2">
		<button
			type="button"
			class="bg-primary text-primary-foreground hover:bg-primary/90 h-7 rounded-md px-3 text-xs font-medium disabled:opacity-50"
			disabled={submitting}
			onclick={() => void submit()}
		>
			{submitting ? 'Creating…' : 'Create'}
		</button>
		<button
			type="button"
			class="text-muted-foreground hover:bg-muted h-7 rounded-md px-3 text-xs"
			disabled={submitting}
			onclick={() => oncancel?.()}
		>
			Cancel
		</button>
	</div>
	{#if error}
		<p class="text-xs text-red-400">{error}</p>
	{/if}
</div>
