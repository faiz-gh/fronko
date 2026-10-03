<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import { login, register } from '$lib/api/auth';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Alert from '$lib/components/ui/alert';
	import * as Field from '$lib/components/ui/field';
	import * as Tabs from '$lib/components/ui/tabs';
	import Logo from '$lib/components/app/logo.svelte';
	import ProfileCard from '$lib/components/app/profile-card.svelte';
	import { emptyCard, type CardData } from '$lib/card/card';
	import { session } from '$lib/session.svelte';

	type Mode = 'login' | 'register';

	// Illustrative sample for the side panel.
	const sample: CardData = {
		...emptyCard('Daniel Reyes'),
		title: 'Founder',
		company: 'Tidewater Coffee',
		location: 'Austin, TX',
		email: 'daniel@example.com',
		phone: '+1 512 555 0199',
		website: 'tidewater.example',
		accent: 'orange',
		links: [{ id: '1', label: '', url: 'https://instagram.com/tidewater' }]
	};

	let mode = $state<Mode>(page.url.searchParams.get('mode') === 'register' ? 'register' : 'login');
	let username = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	const expired = page.url.searchParams.has('expired');

	// Only follow same-site relative paths, never "//evil.com" or absolute URLs.
	function nextPath(): string {
		const next = page.url.searchParams.get('next');
		return next && next.startsWith('/') && !next.startsWith('//') ? next : '/dashboard';
	}

	// Already signed in (the session check resolves asynchronously).
	$effect(() => {
		if (session.isAuthenticated && !loading) goto(nextPath(), { replaceState: true });
	});

	const usernameError = $derived(
		mode === 'register' && username.length > 0 && !/^[a-zA-Z0-9_.-]{3,32}$/.test(username)
			? '3–32 characters: letters, numbers, ".", "_" or "-".'
			: ''
	);
	const passwordError = $derived(
		mode === 'register' && password.length > 0 && password.length < 8 ? 'At least 8 characters.' : ''
	);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (usernameError || passwordError) return;
		error = '';
		loading = true;
		try {
			const res = mode === 'login' ? await login(username, password) : await register(username, password);
			session.signIn(res.username);
			await goto(nextPath(), { replaceState: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Something went wrong';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>{mode === 'login' ? 'Sign in' : 'Create account'} · Fronko</title>
</svelte:head>

<div class="grid min-h-svh lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
	<div class="flex flex-col gap-4 p-6 md:p-10 lg:p-12">
		<Logo />
		<div class="flex flex-1 items-center justify-center">
			<div class="flex w-full max-w-sm flex-col gap-7">
				<div class="flex flex-col gap-1">
					<h1 class="text-3xl font-semibold tracking-tight">
						{mode === 'login' ? 'Welcome back' : 'Create your account'}
					</h1>
					<p class="text-muted-foreground">
						{mode === 'login'
							? 'Sign in to manage your cards and leads.'
							: 'Set up your first digital business card in a minute.'}
					</p>
				</div>

				<Tabs.Root
					value={mode}
					onValueChange={(v) => {
						mode = v as Mode;
						error = '';
					}}
				>
					<Tabs.List class="w-full">
						<Tabs.Trigger value="login">Sign in</Tabs.Trigger>
						<Tabs.Trigger value="register">Create account</Tabs.Trigger>
					</Tabs.List>
				</Tabs.Root>

				{#if expired && !error}
					<Alert.Root>
						<CircleAlertIcon />
						<Alert.Title>Your session expired</Alert.Title>
						<Alert.Description>Please sign in again to continue.</Alert.Description>
					</Alert.Root>
				{/if}

				{#if error}
					<Alert.Root variant="destructive">
						<CircleAlertIcon />
						<Alert.Title>{error}</Alert.Title>
					</Alert.Root>
				{/if}

				<form onsubmit={handleSubmit}>
					<Field.Group>
						<Field.Field data-invalid={!!usernameError || undefined}>
							<Field.Label for="username">Username</Field.Label>
							<Input
								id="username"
								autocomplete="username"
								autocapitalize="none"
								spellcheck={false}
								required
								bind:value={username}
								disabled={loading}
								aria-invalid={!!usernameError || undefined}
							/>
							{#if usernameError}
								<Field.Error>{usernameError}</Field.Error>
							{/if}
						</Field.Field>
						<Field.Field data-invalid={!!passwordError || undefined}>
							<Field.Label for="password">Password</Field.Label>
							<Input
								id="password"
								type="password"
								autocomplete={mode === 'login' ? 'current-password' : 'new-password'}
								required
								bind:value={password}
								disabled={loading}
								aria-invalid={!!passwordError || undefined}
							/>
							{#if passwordError}
								<Field.Error>{passwordError}</Field.Error>
							{:else if mode === 'register'}
								<Field.Description>At least 8 characters.</Field.Description>
							{/if}
						</Field.Field>
						<Button type="submit" size="lg" class="w-full" disabled={loading || !username || !password}>
							{#if loading}
								<Spinner data-icon="inline-start" />
							{/if}
							{mode === 'login' ? 'Sign in' : 'Create account'}
						</Button>
					</Field.Group>
				</form>
			</div>
		</div>
	</div>

	<div class="bg-primary text-primary-foreground relative hidden overflow-hidden lg:flex lg:flex-col lg:justify-between lg:p-12">
		<div
			class="absolute inset-0 opacity-60"
			style="background-image: radial-gradient(oklch(1 0 0 / 0.12) 1px, transparent 1px); background-size: 18px 18px; mask-image: radial-gradient(70% 60% at 60% 45%, black, transparent)"
			aria-hidden="true"
		></div>
		<div
			class="absolute top-1/4 left-1/2 size-[520px] -translate-x-1/2 rounded-full opacity-50 blur-3xl"
			style="background: radial-gradient(closest-side, oklch(0.64 0.19 45 / 0.5), transparent)"
			aria-hidden="true"
		></div>

		<div class="relative grid flex-1 place-items-center py-8">
			<div class="pointer-events-none w-full max-w-[340px] rotate-[-2deg] select-none" aria-hidden="true">
				<ProfileCard card={sample} slug="daniel" />
			</div>
		</div>

		<div class="relative flex max-w-md flex-col gap-2">
			<p class="text-xl font-medium tracking-tight text-balance">
				One link for your contact details, your socials, and a way for people to reach you back.
			</p>
			<p class="text-primary-foreground/60 text-sm">Make as many cards as you need. Every lead lands in one place.</p>
		</div>
	</div>
</div>
