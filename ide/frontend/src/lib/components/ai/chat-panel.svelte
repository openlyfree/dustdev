<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { Bot, RotateCcw, Send } from '@lucide/svelte';
	import type { ChatCompletionMessageParam } from '@mlc-ai/web-llm';
	import {
		AI_MODELS,
		aiError,
		aiProgress,
		aiStatus,
		DEFAULT_MODEL,
		hasWebGPU,
		initModel,
		loadedModel,
		resetAiChat,
		streamChat
	} from '$lib/ai/engine';
	import { activeFile, files } from '$lib/workspace/files';
	import { decodeFileContent, isBinaryPath, isTextContent } from '$lib/utils/language';

	type ChatMessage = { role: 'user' | 'assistant'; content: string };

	const MAX_FILE_CONTEXT = 3000;
	const MAX_HISTORY = 10;

	let selectedModel = $state(DEFAULT_MODEL);
	let messages = $state<ChatMessage[]>([]);
	let input = $state('');
	let generating = $state(false);
	let attachFile = $state(true);
	let scrollEl = $state<HTMLDivElement | undefined>();

	const activePath = $derived($activeFile?.path ?? null);
	const canSend = $derived($aiStatus === 'ready' && !generating && input.trim().length > 0);

	const attachedFileContext = $derived.by(() => {
		if (!attachFile || !activePath) return null;
		const stored = $files.find((f) => f.path === activePath);
		if (!stored?.data) return null;
		if (isBinaryPath(activePath) || !isTextContent(stored.data)) return null;
		const text = decodeFileContent(stored.data).slice(0, MAX_FILE_CONTEXT);
		return { path: activePath, text };
	});

	onMount(() => {
		if (!hasWebGPU()) {
			aiStatus.set('unsupported');
		}
	});

	function buildRequestMessages(): ChatCompletionMessageParam[] {
		const systemParts = [
			'You are a coding assistant running fully offline in a browser IDE. Keep answers concise and use code blocks for code.'
		];
		if (attachedFileContext) {
			systemParts.push(
				`The user has this file open (${attachedFileContext.path}):\n\n${attachedFileContext.text}`
			);
		}

		const history = messages.slice(-MAX_HISTORY).map<ChatCompletionMessageParam>((m) => ({
			role: m.role,
			content: m.content
		}));

		return [{ role: 'system', content: systemParts.join('\n\n') }, ...history];
	}

	async function scrollToBottom() {
		await tick();
		if (scrollEl) {
			scrollEl.scrollTop = scrollEl.scrollHeight;
		}
	}

	async function send() {
		const text = input.trim();
		if (!text || generating || $aiStatus !== 'ready') return;

		input = '';
		messages.push({ role: 'user', content: text });
		const request = buildRequestMessages();
		messages.push({ role: 'assistant', content: '' });
		generating = true;
		await scrollToBottom();

		try {
			for await (const delta of streamChat(request)) {
				messages[messages.length - 1].content += delta;
				void scrollToBottom();
			}
		} catch (error) {
			messages[messages.length - 1].content =
				`Error: ${error instanceof Error ? error.message : 'generation failed'}`;
		} finally {
			generating = false;
		}
	}

	async function newChat() {
		if (generating) return;
		messages = [];
		await resetAiChat();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && !event.shiftKey) {
			event.preventDefault();
			void send();
		}
	}
</script>

<div class="flex h-full flex-col">
	<div
		class="flex items-center justify-between gap-2 border-b border-border bg-muted/40 px-3 py-1.5"
	>
		<div class="flex min-w-0 flex-1 items-center gap-2">
			<Bot class="size-3.5 shrink-0 text-muted-foreground" />
			{#if $aiStatus === 'ready'}
				<span class="flex items-center gap-1.5 text-[10px] text-muted-foreground">
					<span class="size-1.5 rounded-full bg-green-500"></span>
					{$loadedModel} — runs locally
				</span>
			{:else if $aiStatus === 'unsupported'}
				<span class="text-[10px] text-red-400">WebGPU is not available in this browser</span>
			{:else if $aiStatus === 'loading'}
				<span class="min-w-0 flex-1 truncate text-[10px] text-muted-foreground">
					{Math.round($aiProgress.percent * 100)}% — {$aiProgress.text}
				</span>
			{:else if $aiStatus === 'error'}
				<span class="min-w-0 flex-1 truncate text-[10px] text-red-400">{$aiError}</span>
			{:else}
				<select
					bind:value={selectedModel}
					class="max-w-44 rounded-md border border-border bg-background px-1.5 py-0.5 text-[10px] text-foreground"
				>
					{#each AI_MODELS as model (model.id)}
						<option value={model.id}>{model.label} ({model.size})</option>
					{/each}
				</select>
				<button
					type="button"
					class="rounded-md bg-primary px-2 py-0.5 text-[10px] font-medium text-primary-foreground transition-opacity hover:opacity-90"
					onclick={() => void initModel(selectedModel)}
				>
					Load model
				</button>
				<span class="hidden text-[10px] text-muted-foreground sm:inline">
					Downloads once, then works offline
				</span>
			{/if}
		</div>
		{#if messages.length > 0}
			<button
				type="button"
				class="shrink-0 rounded p-1 text-muted-foreground hover:text-foreground"
				title="New chat"
				aria-label="New chat"
				onclick={() => void newChat()}
			>
				<RotateCcw class="size-3.5" />
			</button>
		{/if}
	</div>

	{#if $aiStatus === 'loading'}
		<div class="h-0.5 w-full bg-muted">
			<div
				class="h-full bg-primary transition-all duration-300"
				style:width={`${$aiProgress.percent * 100}%`}
			></div>
		</div>
	{/if}

	<div bind:this={scrollEl} class="min-h-0 flex-1 space-y-3 overflow-y-auto p-3">
		{#if messages.length === 0}
			<div
				class="flex h-full items-center justify-center p-4 text-center text-xs text-muted-foreground"
			>
				<p>
					{#if $aiStatus === 'ready'}
						Ask anything — the model runs on your GPU, no network needed.
					{:else}
						Load a model above to chat with a local AI.<br />
						Weights are cached in the browser, so it keeps working offline.
					{/if}
				</p>
			</div>
		{:else}
			{#each messages as message, i (i)}
				<div class="flex {message.role === 'user' ? 'justify-end' : 'justify-start'}">
					<div
						class="max-w-[85%] rounded-lg px-3 py-1.5 text-xs leading-relaxed break-words whitespace-pre-wrap {message.role ===
						'user'
							? 'bg-primary text-primary-foreground'
							: 'bg-muted text-foreground'}"
					>
						{message.content}{#if generating && i === messages.length - 1 && message.role === 'assistant'}<span
								class="inline-block h-3 w-1 animate-pulse bg-current align-text-bottom"
							></span>{/if}
					</div>
				</div>
			{/each}
		{/if}
	</div>

	<div class="border-t border-border p-2">
		{#if attachedFileContext}
			<p class="mb-1 truncate px-1 text-[10px] text-muted-foreground">
				Context: {attachedFileContext.path}
			</p>
		{/if}
		<div class="flex items-end gap-2">
			<textarea
				bind:value={input}
				onkeydown={handleKeydown}
				placeholder={$aiStatus === 'ready' ? 'Ask the local model…' : 'Load a model first…'}
				disabled={$aiStatus !== 'ready'}
				rows="2"
				class="min-h-9 flex-1 resize-none rounded-md border border-border bg-background px-2 py-1.5 text-xs text-foreground placeholder:text-muted-foreground focus:ring-1 focus:ring-ring focus:outline-none disabled:opacity-50"
			></textarea>
			<button
				type="button"
				class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-40"
				disabled={!canSend}
				title="Send"
				aria-label="Send message"
				onclick={() => void send()}
			>
				<Send class="size-3.5" />
			</button>
		</div>
		{#if activePath && $aiStatus === 'ready'}
			<label
				class="mt-1 flex w-fit cursor-pointer items-center gap-1.5 px-1 text-[10px] text-muted-foreground"
			>
				<input type="checkbox" bind:checked={attachFile} class="size-3 accent-primary" />
				Attach active file as context
			</label>
		{/if}
	</div>
</div>
