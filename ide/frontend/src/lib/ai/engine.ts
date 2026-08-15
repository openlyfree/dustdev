import { get, writable } from 'svelte/store';
import {
	WebWorkerMLCEngine,
	type ChatCompletionMessageParam,
	type InitProgressReport
} from '@mlc-ai/web-llm';
import ChatWorker from './worker?worker';

export interface AiModelOption {
	id: string;
	label: string;
	size: string;
}

export const AI_MODELS: AiModelOption[] = [
	{ id: 'Llama-3.2-1B-Instruct-q4f16_1-MLC', label: 'Llama 3.2 1B', size: '~0.9 GB' },
	{ id: 'SmolLM2-1.7B-Instruct-q4f16_1-MLC', label: 'SmolLM2 1.7B', size: '~1.1 GB' },
	{ id: 'Qwen2.5-1.5B-Instruct-q4f16_1-MLC', label: 'Qwen 2.5 1.5B', size: '~1.2 GB' },
	{ id: 'Llama-3.2-3B-Instruct-q4f16_1-MLC', label: 'Llama 3.2 3B', size: '~2 GB' }
];

export const DEFAULT_MODEL = AI_MODELS[0].id;

export type AiStatus = 'idle' | 'unsupported' | 'loading' | 'ready' | 'error';

export const aiStatus = writable<AiStatus>('idle');
export const aiProgress = writable<{ text: string; percent: number }>({ text: '', percent: 0 });
export const aiError = writable<string | null>(null);
export const loadedModel = writable<string | null>(null);

let engine: WebWorkerMLCEngine | null = null;

export function hasWebGPU(): boolean {
	return typeof navigator !== 'undefined' && 'gpu' in navigator;
}

export async function initModel(modelId: string): Promise<void> {
	if (!hasWebGPU()) {
		aiStatus.set('unsupported');
		return;
	}
	if (get(aiStatus) === 'loading') return;
	if (engine && get(loadedModel) === modelId) return;

	aiStatus.set('loading');
	aiError.set(null);
	aiProgress.set({ text: 'Starting…', percent: 0 });

	if (!engine) {
		engine = new WebWorkerMLCEngine(new ChatWorker(), {
			initProgressCallback: (report: InitProgressReport) => {
				aiProgress.set({ text: report.text, percent: report.progress });
			}
		});
	}

	try {
		await engine.reload(modelId);
		loadedModel.set(modelId);
		aiStatus.set('ready');
	} catch (error) {
		loadedModel.set(null);
		aiStatus.set('error');
		aiError.set(error instanceof Error ? error.message : 'Failed to load model');
		try {
			await engine.unload();
		} catch {
			// Nothing meaningful to release if reload itself failed
		}
	}
}

export async function* streamChat(
	messages: ChatCompletionMessageParam[],
	options: { temperature?: number } = {}
): AsyncGenerator<string> {
	if (!engine || get(aiStatus) !== 'ready') {
		throw new Error('Model is not loaded yet');
	}

	const completion = await engine.chat.completions.create({
		messages,
		stream: true,
		temperature: options.temperature ?? 0.7
	});

	for await (const chunk of completion) {
		const delta = chunk.choices[0]?.delta?.content;
		if (delta) yield delta;
	}
}

export async function resetAiChat(): Promise<void> {
	try {
		await engine?.resetChat();
	} catch {
		// A stale KV cache only affects continuity, safe to ignore
	}
}
