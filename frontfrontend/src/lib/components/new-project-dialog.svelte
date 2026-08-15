<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { api, ApiError, type Project } from '$lib/api';
	import { slugify } from '$lib/utils.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';

	let {
		open = $bindable(false),
		oncreated
	}: {
		open: boolean;
		oncreated: (project: Project) => void;
	} = $props();

	let name = $state('');
	let error = $state('');
	let submitting = $state(false);

	const slug = $derived(slugify(name));

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;
		try {
			const project = await api.createProject(name);
			oncreated(project);
			open = false;
			name = '';
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Something went wrong, try again';
		} finally {
			submitting = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay class="fixed inset-0 z-50 bg-black/60" />
		<Dialog.Content
			class="fixed top-1/2 left-1/2 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-xl border bg-card p-6 shadow-lg outline-none"
		>
			<Dialog.Title class="text-lg font-semibold">New project</Dialog.Title>
			<Dialog.Description class="mt-1 text-sm text-muted-foreground">
				Each project is its own devcontainer with a persistent volume and a subdomain.
			</Dialog.Description>

			<form class="mt-5 flex flex-col gap-4" onsubmit={handleSubmit}>
				<div class="flex flex-col gap-2">
					<Label for="project-name">Project name</Label>
					<Input
						id="project-name"
						placeholder="my-api"
						bind:value={name}
						required
						maxlength={64}
						autocomplete="off"
					/>
					{#if slug}
						<p class="font-mono text-xs text-muted-foreground">
							{slug}.dustdev.app
						</p>
					{/if}
				</div>

				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}

				<div class="flex justify-end gap-2">
					<Dialog.Close>
						{#snippet child({ props })}
							<Button {...props} variant="outline">Cancel</Button>
						{/snippet}
					</Dialog.Close>
					<Button type="submit" disabled={submitting || !slug}>
						{submitting ? 'Creating…' : 'Create project'}
					</Button>
				</div>
			</form>
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
