<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import { ApiError } from '$lib/api';
	import SiteHeader from '$lib/components/site-header.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';

	let email = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');
	let submitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		if (password !== confirm) {
			error = 'Passwords do not match';
			return;
		}
		submitting = true;
		try {
			await auth.signup(email, password);
			goto('/dashboard');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Something went wrong, try again';
		} finally {
			submitting = false;
		}
	}
</script>

<SiteHeader />

<main class="mx-auto flex max-w-6xl justify-center px-4 pt-20">
	<Card.Root class="w-full max-w-sm">
		<Card.Header>
			<Card.Title class="text-xl">Create your account</Card.Title>
			<Card.Description>Spin up your first dev environment in seconds.</Card.Description>
		</Card.Header>
		<Card.Content>
			<form class="flex flex-col gap-4" onsubmit={handleSubmit}>
				<div class="flex flex-col gap-2">
					<Label for="email">Email</Label>
					<Input
						id="email"
						type="email"
						placeholder="you@example.com"
						bind:value={email}
						required
						autocomplete="email"
					/>
				</div>
				<div class="flex flex-col gap-2">
					<Label for="password">Password</Label>
					<Input
						id="password"
						type="password"
						placeholder="At least 8 characters"
						bind:value={password}
						required
						minlength={8}
						autocomplete="new-password"
					/>
				</div>
				<div class="flex flex-col gap-2">
					<Label for="confirm">Confirm password</Label>
					<Input
						id="confirm"
						type="password"
						placeholder="••••••••"
						bind:value={confirm}
						required
						autocomplete="new-password"
					/>
				</div>

				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}

				<Button type="submit" disabled={submitting} class="w-full">
					{submitting ? 'Creating account…' : 'Create account'}
				</Button>

				<p class="text-center text-sm text-muted-foreground">
					Already have an account?
					<a href="/login" class="text-foreground underline underline-offset-4">Log in</a>
				</p>
			</form>
		</Card.Content>
	</Card.Root>
</main>
