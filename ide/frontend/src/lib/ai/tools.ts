import { get } from 'svelte/store';
import { files, fileSync, isConnected } from '$lib/workspace/files';
import { decodeFileContent, isBinaryPath, isTextContent } from '$lib/utils/language';
import { runInWebContainer } from '$lib/runtime/webcontainer';

export interface ToolCall {
	name: string;
	args: Record<string, unknown>;
}

export interface ToolResult {
	ok: boolean;
	// One-line description shown in the chat UI
	summary: string;
	// Full result fed back to the model
	output: string;
}

const MAX_TOOL_OUTPUT = 6000;
const MAX_LISTED_FILES = 200;

function truncate(text: string, max = MAX_TOOL_OUTPUT): string {
	return text.length > max ? `${text.slice(0, max)}\n…[truncated]` : text;
}

export const TOOL_INSTRUCTIONS = `You can use tools by outputting a line that starts with TOOL: followed by a JSON object, for example:
TOOL: {"name": "read_file", "args": {"path": "src/index.ts"}}

Available tools:
- list_files — args: {} — list all file paths in the workspace
- read_file — args: {"path": string} — read a file's contents
- write_file — args: {"path": string, "content": string} — create or overwrite a file
- run_command — args: {"command": string} — run a shell command in the project

Rules:
- Output at most ONE TOOL: line per reply, as the last line of the reply.
- After the tool runs you receive a message starting with TOOL RESULT. Use it to decide the next step.
- When you are done using tools, reply to the user normally, without a TOOL: line.
- Read a file before overwriting it unless the user asked for a new file.`;

// Extracts the first TOOL: {...} call from a completion. Uses a balanced-brace
// scan so JSON spanning multiple lines (e.g. file contents) still parses.
export function parseToolCall(text: string): ToolCall | null {
	const marker = text.indexOf('TOOL:');
	if (marker === -1) return null;

	const rest = text.slice(marker + 'TOOL:'.length);
	const start = rest.indexOf('{');
	if (start === -1) return null;

	let depth = 0;
	let inString = false;
	let escaped = false;

	for (let i = start; i < rest.length; i++) {
		const ch = rest[i];
		if (inString) {
			if (escaped) escaped = false;
			else if (ch === '\\') escaped = true;
			else if (ch === '"') inString = false;
			continue;
		}
		if (ch === '"') inString = true;
		else if (ch === '{') depth++;
		else if (ch === '}') {
			depth--;
			if (depth !== 0) continue;
			try {
				const parsed: unknown = JSON.parse(rest.slice(start, i + 1));
				if (
					typeof parsed === 'object' &&
					parsed !== null &&
					'name' in parsed &&
					typeof parsed.name === 'string'
				) {
					const args =
						'args' in parsed && typeof parsed.args === 'object' && parsed.args !== null
							? (parsed.args as Record<string, unknown>)
							: {};
					return { name: parsed.name, args };
				}
				return null;
			} catch {
				return null;
			}
		}
	}
	return null;
}

function normalizeToolPath(raw: unknown): string {
	return String(raw ?? '')
		.trim()
		.replace(/\\/g, '/')
		.replace(/^\/+/, '');
}

async function execListFiles(): Promise<ToolResult> {
	const all = get(files).map((f) => f.path);
	// Dependency trees would flood the listing without helping the model
	const paths = all
		.filter((p) => !p.split('/').includes('node_modules'))
		.sort();
	const omitted = all.length - paths.length;
	if (paths.length === 0) {
		return { ok: true, summary: 'list files (empty)', output: 'The workspace is empty.' };
	}
	const shown = paths.slice(0, MAX_LISTED_FILES);
	const suffix =
		(paths.length > shown.length ? `\n…and ${paths.length - shown.length} more` : '') +
		(omitted > 0 ? `\n(node_modules excluded — ${omitted} files)` : '');
	return {
		ok: true,
		summary: `list files (${paths.length})`,
		output: shown.join('\n') + suffix
	};
}

async function execReadFile(args: Record<string, unknown>): Promise<ToolResult> {
	const path = normalizeToolPath(args.path);
	if (!path) {
		return { ok: false, summary: 'read file', output: 'Error: missing "path" argument.' };
	}

	const file = get(files).find((f) => f.path === path);
	if (!file) {
		return { ok: false, summary: `read ${path}`, output: `Error: file not found: ${path}` };
	}
	if (isBinaryPath(path) || (file.data.length > 0 && !isTextContent(file.data))) {
		return { ok: false, summary: `read ${path}`, output: `Error: ${path} is a binary file.` };
	}

	const text = file.data ? decodeFileContent(file.data) : '';
	return { ok: true, summary: `read ${path}`, output: truncate(text) || '(empty file)' };
}

async function execWriteFile(args: Record<string, unknown>): Promise<ToolResult> {
	const path = normalizeToolPath(args.path);
	const content = String(args.content ?? '');
	if (!path || path.includes('..') || path.endsWith('/')) {
		return { ok: false, summary: 'write file', output: `Error: invalid path: ${path || '(empty)'}` };
	}

	await fileSync.saveFile(path, content);
	return {
		ok: true,
		summary: `wrote ${path}`,
		output: `Wrote ${content.length} characters to ${path}.`
	};
}

async function execRunCommand(args: Record<string, unknown>): Promise<ToolResult> {
	const command = String(args.command ?? '').trim();
	if (!command) {
		return { ok: false, summary: 'run command', output: 'Error: missing "command" argument.' };
	}
	const summary = `$ ${command.length > 60 ? `${command.slice(0, 60)}…` : command}`;

	if (get(isConnected)) {
		const res = await fetch('/exec', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ command })
		});
		if (!res.ok) {
			const body = (await res.text()).slice(0, 500);
			return { ok: false, summary, output: `Error: command failed to run (${res.status}): ${body}` };
		}
		const data = (await res.json()) as { exit_code: number; output: string };
		return {
			ok: data.exit_code === 0,
			summary,
			output: truncate(`exit code: ${data.exit_code}\n${data.output ?? ''}`.trimEnd())
		};
	}

	const { output, exitCode } = await runInWebContainer(command);
	return {
		ok: exitCode === 0,
		summary,
		output: truncate(`exit code: ${exitCode}\n${output}`.trimEnd())
	};
}

const executors: Record<string, (args: Record<string, unknown>) => Promise<ToolResult>> = {
	list_files: execListFiles,
	read_file: execReadFile,
	write_file: execWriteFile,
	run_command: execRunCommand
};

export async function executeTool(call: ToolCall): Promise<ToolResult> {
	const executor = executors[call.name];
	if (!executor) {
		return {
			ok: false,
			summary: `unknown tool ${call.name}`,
			output: `Error: unknown tool "${call.name}". Available tools: ${Object.keys(executors).join(', ')}.`
		};
	}
	try {
		return await executor(call.args);
	} catch (error) {
		const message = error instanceof Error ? error.message : String(error);
		return { ok: false, summary: `${call.name} failed`, output: `Error: ${message}` };
	}
}
