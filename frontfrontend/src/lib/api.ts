export type User = {
	id: string;
	email: string;
};

export type ProjectStatus = 'stopped' | 'starting' | 'running' | 'stopping' | 'error';

export type Project = {
	id: string;
	name: string;
	slug: string;
	status: ProjectStatus;
	url: string;
	created_at: string;
};

export class ApiError extends Error {
	status: number;

	constructor(status: number, message: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
	}
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(`/api${path}`, {
		credentials: 'include',
		headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
		...init
	});

	if (!res.ok) {
		let message = res.statusText;
		try {
			const body = await res.json();
			if (body && typeof body.error === 'string') message = body.error;
		} catch {
			// non-JSON error body; keep statusText
		}
		throw new ApiError(res.status, message);
	}

	if (res.status === 204) return undefined as T;
	return (await res.json()) as T;
}

export const api = {
	signup: (email: string, password: string) =>
		request<User>('/signup', { method: 'POST', body: JSON.stringify({ email, password }) }),
	login: (email: string, password: string) =>
		request<User>('/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
	logout: () => request<void>('/logout', { method: 'POST' }),
	me: () => request<User>('/me'),
	listProjects: () => request<{ projects: Project[] }>('/projects'),
	createProject: (name: string) =>
		request<Project>('/projects', { method: 'POST', body: JSON.stringify({ name }) }),
	startProject: (id: string) => request<Project>(`/projects/${id}/start`, { method: 'POST' }),
	stopProject: (id: string) => request<Project>(`/projects/${id}/stop`, { method: 'POST' }),
	deleteProject: (id: string) => request<void>(`/projects/${id}`, { method: 'DELETE' })
};
