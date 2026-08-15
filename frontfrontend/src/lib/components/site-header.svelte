<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import Boxes from '@lucide/svelte/icons/boxes';

	async function handleLogout() {
		await auth.logout();
		goto('/');
	}
</script>

<header class="sticky top-0 z-40 border-b bg-background/80 backdrop-blur">
	<div class="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
		<a href="/" class="flex items-center gap-2 font-semibold tracking-tight">
			<Boxes class="size-5" />
			<span>dustdev</span>
		</a>

		<nav class="flex items-center gap-2">
			{#if auth.user}
				<Button href="/dashboard" variant="ghost" size="sm">Dashboard</Button>
				<Button variant="outline" size="sm" onclick={handleLogout}>Log out</Button>
			{:else}
				<Button href="/login" variant="ghost" size="sm">Log in</Button>
				<Button href="/signup" size="sm">Get started</Button>
			{/if}
		</nav>
	</div>
</header>
