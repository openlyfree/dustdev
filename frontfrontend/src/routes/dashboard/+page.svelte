<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import { api, type Project, type ProjectStatus } from '$lib/api';
	import SiteHeader from '$lib/components/site-header.svelte';
	import NewProjectDialog from '$lib/components/new-project-dialog.svelte';
	import { Badge, type BadgeVariant } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import Loader2 from '@lucide/svelte/icons/loader-2';
	import Plus from '@lucide/svelte/icons/plus';
	import RefreshCw from '@lucide/svelte/icons/refresh-cw';
	import Square from '@lucide/svelte/icons/square';
	import Play from '@lucide/svelte/icons/play';
	import Trash2 from '@lucide/svelte/icons/trash-2';

	let projects = $state<Project[]>([]);
	let loading = $state(true);
	let error = $state('');
	let dialogOpen = $state(false);
	let busy = $state<Record<string, boolean>>({});

	const statusVariant: Record<ProjectStatus, BadgeVariant> = {
		running: 'success',
		starting: 'warning',
		stopping: 'warning',
		stopped: 'secondary',
		error: 'destructive'
	};

	async function refresh() {
		try {
			const res = await api.listProjects();
			projects = res.projects;
			error = '';
		} catch {
			error = 'Failed to load projects';
		} finally {
			loading = false;
		}
	}

	async function act(id: string, action: (id: string) => Promise<unknown>) {
		busy[id] = true;
		try {
			await action(id);
			await refresh();
		} catch {
			error = 'Action failed, try again';
		} finally {
			busy[id] = false;
		}
	}

	async function remove(id: string, name: string) {
		if (!confirm(`Delete "${name}"? The container and its files will be removed.`)) return;
		await act(id, api.deleteProject);
	}

	onMount(() => {
		let interval: ReturnType<typeof setInterval> | undefined;

		void (async () => {
			if (!auth.loaded) await auth.load();
			if (!auth.user) {
				goto('/login');
				return;
			}
			await refresh();
			interval = setInterval(() => {
				if (projects.some((p) => p.status === 'starting' || p.status === 'stopping')) {
					void refresh();
				}
			}, 3000);
		})();

		return () => clearInterval(interval);
	});
</script>

<SiteHeader />

<main class="mx-auto max-w-6xl px-4 py-10">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">Projects</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				Your devcontainers. Start one and it appears on its own subdomain.
			</p>
		</div>
		<div class="flex items-center gap-2">
			<Button variant="outline" size="icon" onclick={refresh} aria-label="Refresh">
				<RefreshCw />
			</Button>
			<Button onclick={() => (dialogOpen = true)}>
				<Plus />
				New project
			</Button>
		</div>
	</div>

	{#if error}
		<p class="mt-6 text-sm text-destructive">{error}</p>
	{/if}

	{#if loading}
		<div class="mt-20 flex justify-center text-muted-foreground">
			<Loader2 class="size-6 animate-spin" />
		</div>
	{:else if projects.length === 0}
		<Card.Root class="mt-10 border-dashed">
			<Card.Content class="flex flex-col items-center gap-3 py-16 text-center">
				<p class="text-lg font-medium">No projects yet</p>
				<p class="max-w-sm text-sm text-muted-foreground">
					Create your first project to get a browser IDE with a terminal, live previews and
					its own subdomain.
				</p>
				<Button class="mt-2" onclick={() => (dialogOpen = true)}>
					<Plus />
					Create a project
				</Button>
			</Card.Content>
		</Card.Root>
	{:else}
		<div class="mt-8 flex flex-col gap-3">
			{#each projects as project (project.id)}
				<Card.Root>
					<Card.Content class="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between">
						<div class="flex min-w-0 flex-col gap-1">
							<div class="flex items-center gap-2">
								<span class="truncate font-medium">{project.name}</span>
								<Badge variant={statusVariant[project.status] ?? 'secondary'}>
									{project.status}
								</Badge>
							</div>
							<a
								href={project.url}
								target="_blank"
								rel="noopener"
								class="w-fit truncate font-mono text-xs text-muted-foreground hover:text-foreground hover:underline"
							>
								{project.slug}.dustdev.app
							</a>
						</div>

						<div class="flex shrink-0 items-center gap-2">
							{#if project.status === 'running'}
								<Button href={project.url} target="_blank" size="sm">
									<ExternalLink />
									Open IDE
								</Button>
								<Button
									variant="outline"
									size="sm"
									disabled={busy[project.id]}
									onclick={() => act(project.id, api.stopProject)}
								>
									<Square />
									Stop
								</Button>
							{:else}
								<Button
									variant="outline"
									size="sm"
									disabled={busy[project.id] ||
										project.status === 'starting' ||
										project.status === 'stopping'}
									onclick={() => act(project.id, api.startProject)}
								>
									{#if project.status === 'starting' || project.status === 'stopping'}
										<Loader2 class="animate-spin" />
										{project.status === 'starting' ? 'Starting…' : 'Stopping…'}
									{:else}
										<Play />
										Start
									{/if}
								</Button>
							{/if}
							<Button
								variant="ghost"
								size="icon-sm"
								disabled={busy[project.id]}
								aria-label="Delete project"
								onclick={() => remove(project.id, project.name)}
							>
								<Trash2 class="text-destructive" />
							</Button>
						</div>
					</Card.Content>
				</Card.Root>
			{/each}
		</div>
	{/if}
</main>

<NewProjectDialog
	bind:open={dialogOpen}
	oncreated={(project) => {
		projects = [...projects, project];
	}}
/>
