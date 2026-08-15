import type { ChatCompletionMessageParam } from '@mlc-ai/web-llm';
import { streamChat } from '$lib/ai/engine';
import {
	executeTool,
	parseToolCall,
	TOOL_INSTRUCTIONS,
	type ToolCall,
	type ToolResult
} from '$lib/ai/tools';

export type AgentEvent =
	// A new assistant reply started streaming
	| { type: 'step_start' }
	// A streamed token; visible is false once a TOOL: marker appears in the reply
	| { type: 'delta'; text: string; visible: boolean }
	// The reply finished; toolCall is set when the model asked for a tool
	| { type: 'step_end'; toolCall: ToolCall | null }
	// A tool finished executing
	| { type: 'tool'; call: ToolCall; result: ToolResult }
	| { type: 'done' };

const MAX_STEPS = 8;

export async function* runAgent(
	userText: string,
	history: ChatCompletionMessageParam[],
	systemPrompt: string
): AsyncGenerator<AgentEvent> {
	const conversation: ChatCompletionMessageParam[] = [
		{ role: 'system', content: `${systemPrompt}\n\n${TOOL_INSTRUCTIONS}` },
		...history,
		{ role: 'user', content: userText }
	];

	for (let step = 0; step < MAX_STEPS; step++) {
		if (step === MAX_STEPS - 1) {
			conversation.push({
				role: 'user',
				content: '(Tool limit reached — do not call more tools. Summarize for the user.)'
			});
		}

		yield { type: 'step_start' };

		let text = '';
		let toolDetected = false;
		for await (const delta of streamChat(conversation, { temperature: 0.2 })) {
			text += delta;
			if (!toolDetected && text.includes('TOOL:')) toolDetected = true;
			yield { type: 'delta', text: delta, visible: !toolDetected };
		}

		const attempted = text.includes('TOOL:');
		const toolCall = parseToolCall(text);
		yield { type: 'step_end', toolCall };
		if (!attempted) break;

		conversation.push({ role: 'assistant', content: text });

		if (!toolCall) {
			// The model tried to call a tool but the JSON was unreadable — let it retry
			const result: ToolResult = {
				ok: false,
				summary: 'malformed tool call',
				output:
					'Error: could not parse the tool call. Reply with a single line: TOOL: {"name": "...", "args": {...}}'
			};
			yield { type: 'tool', call: { name: 'unknown', args: {} }, result };
			conversation.push({ role: 'user', content: `TOOL RESULT:\n${result.output}` });
			continue;
		}

		const result = await executeTool(toolCall);
		yield { type: 'tool', call: toolCall, result };
		conversation.push({
			role: 'user',
			content: `TOOL RESULT (${toolCall.name}):\n${result.output}`
		});
	}

	yield { type: 'done' };
}
