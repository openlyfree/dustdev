<script lang="ts">
	import { ExternalLink, RefreshCw } from '@lucide/svelte';
	import { openPorts, previewUrl, selectedPreviewPort } from '$lib/runtime/ports';

	let iframeKey = $state(0);

	const activePort = $derived($selectedPreviewPort);
	const previewSrc = $derived(activePort !== null ? previewUrl(activePort) : null);

	function selectPort(port: number) {
		selectedPreviewPort.set(port);
	}

	function reloadPreview() {
		iframeKey += 1;
	}

	function openExternal() {
		if (activePort === null) return;
		window.open(previewUrl(activePort), '_blank', 'noopener,noreferrer');
	}
</script>

<div class="flex h-full flex-col">
	<div
		class="flex items-center justify-between gap-2 border-b border-border bg-muted/40 px-3 py-1.5"
	>
		<div class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
			{#if $openPorts.length === 0}
				<span class="text-[10px] text-muted-foreground">
					No open ports — start a dev server in the terminal
				</span>
			{:else}
				{#each $openPorts as port (port)}
					<button
						type="button"
						class="shrink-0 rounded-md px-2 py-0.5 font-mono text-[10px] transition-colors {activePort ===
						port
							? 'bg-primary text-primary-foreground'
							: 'bg-muted text-muted-foreground hover:text-foreground'}"
						onclick={() => selectPort(port)}
					>
						:{port}
					</button>
				{/each}
			{/if}
		</div>
		{#if previewSrc}
			<div class="flex shrink-0 items-center gap-1">
				<button
					type="button"
					class="rounded p-1 text-muted-foreground hover:text-foreground"
					title="Reload preview"
					aria-label="Reload preview"
					onclick={reloadPreview}
				>
					<RefreshCw class="size-3.5" />
				</button>
				<button
					type="button"
					class="rounded p-1 text-muted-foreground hover:text-foreground"
					title="Open in new tab"
					aria-label="Open in new tab"
					onclick={openExternal}
				>
					<ExternalLink class="size-3.5" />
				</button>
			</div>
		{/if}
	</div>

	<div class="relative min-h-0 flex-1 bg-background">
		{#if previewSrc}
			{#key `${previewSrc}-${iframeKey}`}
				<iframe
					title="App preview"
					src={previewSrc}
					class="h-full w-full border-0 bg-white"
					sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals"
				></iframe>
			{/key}
		{:else}
			<div
				class="flex h-full items-center justify-center p-4 text-center text-xs text-muted-foreground"
			>
				<p>
					Run a server in the terminal, e.g.<br />
					<code class="font-mono">python3 -m http.server 3000</code><br />
					or <code class="font-mono">npx serve . -l 3000</code>
				</p>
			</div>
		{/if}
	</div>
</div>
