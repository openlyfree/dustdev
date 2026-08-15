<script lang="ts">
	import { onMount, tick } from 'svelte';
	import {
		Bot,
		FilePen,
		FileText,
		List,
		RotateCcw,
		Send,
		SquareTerminal,
		Wrench
	} from '@lucide/svelte';
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
		resetAiChat
	} from '$lib/ai/engine';
	import { runAgent } from '$lib/ai/agent';
	import { activeFile, files } from '$lib/workspace/files';
	import { decodeFileContent, isBinaryPath, isTextContent } from '$lib/utils/language';

	type ToolActivity = { name: string; summary: string; ok: boolean; detail: string };
	type ChatMessage =
		| { role: 'user' | 'assistant'; content: string }
		| { role: 'tool'; tool: ToolActivity };

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

	function buildSystemPrompt(): string {
		const parts = [
			'You are a coding assistant in a web IDE. You can read and edit files and run commands in the workspace using tools. Keep answers concise and use code blocks for code.'
		];
		if (attachedFileContext) {
			parts.push(
				`The user has this file open (${attachedFileContext.path}):\n\n${attachedFileContext.text}`
			);
		}
		return parts.join('\n\n');
	}

	function buildHistory(): ChatCompletionMessageParam[] {
		return messages
			.filter(
				(m): m is { role: 'user' | 'assistant'; content: string } =>
					(m.role === 'user' || m.role === 'assistant') && m.content.trim().length > 0
			)
			.slice(-MAX_HISTORY)
			.map((m) => ({ role: m.role, content: m.content }));
	}

	async function scrollToBottom() {
		await tick();
		if (scrollEl) {
			scrollEl.scrollTop = scrollEl.scrollHeight;
		}
	}

	function lastAssistant(): (ChatMessage & { role: 'assistant' }) | null {
		const last = messages[messages.length - 1];
		return last?.role === 'assistant' ? (last as ChatMessage & { role: 'assistant' }) : null;
	}

	async function send() {
		const text = input.trim();
		if (!text || generating || $aiStatus !== 'ready') return;

		input = '';
		messages.push({ role: 'user', content: text });
		const systemPrompt = buildSystemPrompt();
		const history = buildHistory();
		generating = true;
		await scrollToBottom();

		try {
			for await (const event of runAgent(text, history, systemPrompt)) {
				if (event.type === 'step_start') {
					messages.push({ role: 'assistant', content: '' });
				} else if (event.type === 'delta') {
					const assistant = lastAssistant();
					if (assistant && event.visible) assistant.content += event.text;
				} else if (event.type === 'step_end') {
					if (event.toolCall || messages.at(-1)?.role === 'assistant') {
						const assistant = lastAssistant();
						if (assistant && event.toolCall) {
							const marker = assistant.content.indexOf('TOOL:');
							if (marker >= 0) assistant.content = assistant.content.slice(0, marker).trimEnd();
							// Drop bubbles that only contained the tool call
							if (!assistant.content.trim()) messages.pop();
						}
					}
				} else if (event.type === 'tool') {
					messages.push({
						role: 'tool',
						tool: {
							name: event.call.name,
							summary: event.result.summary,
							ok: event.result.ok,
							detail: event.result.output
						}
					});
				}
				void scrollToBottom();
			}
		} catch (error) {
			const message = `Error: ${error instanceof Error ? error.message : 'generation failed'}`;
			const assistant = lastAssistant();
			if (assistant && !assistant.content.trim()) {
				assistant.content = message;
			} else {
				messages.push({ role: 'assistant', content: message });
			}
		}

		const tail = lastAssistant();
		if (tail && !tail.content.trim()) messages.pop();
		generating = false;
		void scrollToBottom();
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
					{$loadedModel}
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
						Ask anything — it can read and edit files and run commands.
					{:else}
						Load a model above to start chatting.
					{/if}
				</p>
			</div>
		{:else}
			{#each messages as message, i (i)}
				{#if message.role === 'tool'}
					<div class="flex justify-start">
						<div
							class="flex max-w-[85%] items-center gap-1.5 rounded-md border border-border bg-muted/40 px-2 py-1 font-mono text-[10px] text-muted-foreground"
							title={message.tool.detail}
						>
							{#if message.tool.name === 'run_command'}
								<SquareTerminal class="size-3 shrink-0" />
							{:else if message.tool.name === 'write_file'}
								<FilePen class="size-3 shrink-0" />
							{:else if message.tool.name === 'read_file'}
								<FileText class="size-3 shrink-0" />
							{:else if message.tool.name === 'list_files'}
								<List class="size-3 shrink-0" />
							{:else}
								<Wrench class="size-3 shrink-0" />
							{/if}
							<span class="truncate">{message.tool.summary}</span>
							<span class={message.tool.ok ? 'text-green-500' : 'text-red-400'}>
								{message.tool.ok ? '✓' : '✗'}
							</span>
						</div>
					</div>
				{:else}
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
				{/if}
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
				placeholder={$aiStatus === 'ready' ? 'Ask anything…' : 'Load a model first…'}
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
