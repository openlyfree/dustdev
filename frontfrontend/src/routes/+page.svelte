<script lang="ts">
	import { onMount } from 'svelte';
	import { auth } from '$lib/auth.svelte';
	import SiteHeader from '$lib/components/site-header.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import Boxes from '@lucide/svelte/icons/boxes';
	import Code2 from '@lucide/svelte/icons/code-2';
	import Container from '@lucide/svelte/icons/container';
	import Globe from '@lucide/svelte/icons/globe';
	import Play from '@lucide/svelte/icons/play';
	import SquareTerminal from '@lucide/svelte/icons/square-terminal';
	import Zap from '@lucide/svelte/icons/zap';

	onMount(() => {
		if (!auth.loaded) void auth.load();
	});

	const features = [
		{
			icon: Code2,
			title: 'Full editor in the browser',
			description:
				'Monaco-powered editing with the keybindings and IntelliSense you already know. No install, no sync, no setup.'
		},
		{
			icon: SquareTerminal,
			title: 'A real terminal',
			description:
				'A persistent shell inside your container with git, curl and a Nix toolchain ready to go. Install anything, break anything.'
		},
		{
			icon: Zap,
			title: 'Instant port previews',
			description:
				'Start a dev server and it shows up automatically. Every listening port gets a live preview URL, proxied and ready to share.'
		},
		{
			icon: Container,
			title: 'Disposable devcontainers',
			description:
				'Every project is its own isolated container. Spin one up in seconds, throw it away when you are done.'
		},
		{
			icon: Globe,
			title: 'A subdomain per project',
			description:
				'Each environment lives on its own subdomain with automatic TLS. Open it from your laptop, tablet, or a borrowed machine.'
		},
		{
			icon: Play,
			title: 'Start and stop on demand',
			description:
				'Stopped environments keep their files and cost nothing. Hit start and pick up exactly where you left off.'
		}
	];

	const steps = [
		{
			title: 'Create a project',
			description: 'Pick a name and we carve out a fresh container and volume for it.'
		},
		{
			title: 'Hit start',
			description: 'Your devcontainer boots and gets its own TLS-secured subdomain.'
		},
		{
			title: 'Write code',
			description: 'Editor, terminal and live previews — all in one tab, from anywhere.'
		}
	];
</script>

<SiteHeader />

<main>
	<!-- Hero -->
	<section class="mx-auto max-w-6xl px-4 pt-24 pb-20 text-center sm:pt-32">
		<div
			class="mx-auto mb-6 inline-flex items-center gap-2 rounded-full border bg-card px-3 py-1 text-xs text-muted-foreground"
		>
			<span class="size-1.5 rounded-full bg-emerald-500"></span>
			Dev environments, on tap
		</div>
		<h1 class="mx-auto max-w-3xl text-4xl font-bold tracking-tight text-balance sm:text-6xl">
			Your dev environment, one click away
		</h1>
		<p class="mx-auto mt-6 max-w-2xl text-lg text-pretty text-muted-foreground">
			dustdev spins up disposable devcontainers in the cloud and hands you a full IDE in the
			browser — editor, terminal, and live previews on your own subdomain.
		</p>
		<div class="mt-10 flex items-center justify-center gap-3">
			<Button href="/signup" size="lg" class="px-6">
				Start building
				<ArrowRight />
			</Button>
			<Button href="/login" variant="outline" size="lg" class="px-6">Log in</Button>
		</div>
	</section>

	<!-- Features -->
	<section class="border-t bg-card/40">
		<div class="mx-auto max-w-6xl px-4 py-20">
			<h2 class="text-center text-3xl font-bold tracking-tight">Everything you need to ship</h2>
			<p class="mx-auto mt-3 max-w-xl text-center text-muted-foreground">
				A complete development box that lives in the cloud and follows you anywhere.
			</p>
			<div class="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
				{#each features as feature (feature.title)}
					<Card.Root class="bg-background">
						<Card.Header>
							<div
								class="mb-2 flex size-9 items-center justify-center rounded-lg border bg-muted"
							>
								<feature.icon class="size-4.5" />
							</div>
							<Card.Title>{feature.title}</Card.Title>
							<Card.Description>{feature.description}</Card.Description>
						</Card.Header>
					</Card.Root>
				{/each}
			</div>
		</div>
	</section>

	<!-- How it works -->
	<section class="border-t">
		<div class="mx-auto max-w-6xl px-4 py-20">
			<h2 class="text-center text-3xl font-bold tracking-tight">Up and running in seconds</h2>
			<div class="mt-12 grid gap-8 sm:grid-cols-3">
				{#each steps as step, i (step.title)}
					<div class="flex flex-col items-start gap-3">
						<div
							class="flex size-9 items-center justify-center rounded-lg bg-primary text-sm font-semibold text-primary-foreground"
						>
							{i + 1}
						</div>
						<h3 class="text-lg font-semibold">{step.title}</h3>
						<p class="text-sm text-muted-foreground">{step.description}</p>
					</div>
				{/each}
			</div>
			<div class="mt-16 text-center">
				<Button href="/signup" size="lg" class="px-6">
					Create your first environment
					<ArrowRight />
				</Button>
			</div>
		</div>
	</section>
</main>

<footer class="border-t">
	<div
		class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-3 px-4 py-8 text-sm text-muted-foreground sm:flex-row"
	>
		<div class="flex items-center gap-2">
			<Boxes class="size-4" />
			<span>dustdev</span>
		</div>
		<p>Disposable dev environments in your browser.</p>
	</div>
</footer>
