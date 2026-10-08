<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import KeyRoundIcon from '@lucide/svelte/icons/key-round';
	import { discoverSSO, login, register, ssoStartUrl } from '$lib/features/auth/api';
	import { ApiError } from '$lib/core/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Alert from '$lib/components/ui/alert';
	import * as Field from '$lib/components/ui/field';
	import * as Tabs from '$lib/components/ui/tabs';
	import AuthLayout from '$lib/features/auth/components/auth-layout.svelte';
	import { session } from '$lib/core/session.svelte';

	type Mode = 'login' | 'register';

	let mode = $state<Mode>(page.url.searchParams.get('mode') === 'register' ? 'register' : 'login');
	let username = $state('');
	let email = $state('');
	let organization = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	const expired = page.url.searchParams.has('expired');
	const passwordReset = page.url.searchParams.has('reset');
	const accountDeleted = page.url.searchParams.has('deleted');
	// Single sign-on sends people back here when it fails, saying why.
	const ssoError = page.url.searchParams.get('sso_error') ?? '';

	// Signing in with single sign-on instead of a password.
	let ssoMode = $state(false);
	let ssoIdentifier = $state('');
	let ssoLoading = $state(false);

	function startSSO(path: string) {
		ssoLoading = true;
		window.location.assign(ssoStartUrl(path, nextPath()));
	}

	async function handleSSO(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		ssoLoading = true;
		try {
			const { url } = await discoverSSO(ssoIdentifier.trim());
			startSSO(url);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Something went wrong';
			ssoLoading = false;
		}
	}

	// Only follow same-site relative paths, never "//evil.com" or absolute URLs.
	function nextPath(): string {
		const next = page.url.searchParams.get('next');
		return next && next.startsWith('/') && !next.startsWith('//') ? next : '/dashboard';
	}

	// Signed in already, or just now. Unfinished accounts verify their email and
	// replace a temporary password first.
	$effect(() => {
		if (!session.isAuthenticated || loading) return;
		goto(session.nextStep(nextPath()), { replaceState: true });
	});

	const usernameError = $derived(
		mode === 'register' && username.length > 0 && !/^[a-zA-Z0-9_.-]{3,32}$/.test(username)
			? '3–32 characters: letters, numbers, ".", "_" or "-".'
			: ''
	);
	const emailError = $derived(
		mode === 'register' && email.length > 0 && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())
			? 'Enter a valid email address.'
			: ''
	);
	const passwordError = $derived(
		mode === 'register' && password.length > 0 && password.length < 8 ? 'At least 8 characters.' : ''
	);
	const canSubmit = $derived(!!username && !!password && (mode === 'login' || !!email));

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (usernameError || emailError || passwordError) return;
		error = '';
		session.suspendedReason = null;
		loading = true;
		try {
			const user =
				mode === 'login'
					? await login(username, password)
					: await register(username, email.trim(), password, organization.trim());
			session.signIn(user);
		} catch (e) {
			if (e instanceof ApiError && e.code === 'org_suspended') session.suspendedReason = e.reason ?? '';
			else if (e instanceof ApiError && e.code === 'sso_required' && e.ssoUrl) startSSO(e.ssoUrl);
			else error = e instanceof Error ? e.message : 'Something went wrong';
		} finally {
			// The effect above navigates once loading drops.
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>{mode === 'login' ? 'Sign in' : 'Create account'} · Fronko</title>
</svelte:head>

<AuthLayout>
	<div class="flex flex-col gap-1">
		<h1 class="text-3xl font-semibold tracking-tight">
			{mode === 'login' ? 'Welcome back' : 'Create your account'}
		</h1>
		<p class="text-muted-foreground">
			{mode === 'login'
				? 'Sign in to manage your cards and leads.'
				: 'Set up digital business cards for you and your team.'}
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

	{#if session.suspendedReason !== null && !error}
		<Alert.Root variant="destructive">
			<CircleAlertIcon />
			<Alert.Title>Your organisation has been suspended</Alert.Title>
			<Alert.Description>
				{#if session.suspendedReason}<p>Reason: {session.suspendedReason}</p>{/if}
				<p>Nobody in it can sign in until it's reinstated. Your organisation's owner was emailed the details.</p>
			</Alert.Description>
		</Alert.Root>
	{:else if error}
		<Alert.Root variant="destructive">
			<CircleAlertIcon />
			<Alert.Title>{error}</Alert.Title>
		</Alert.Root>
	{:else if ssoError && mode === 'login'}
		<Alert.Root variant="destructive">
			<CircleAlertIcon />
			<Alert.Title>Single sign-on didn't work</Alert.Title>
			<Alert.Description>{ssoError}</Alert.Description>
		</Alert.Root>
	{:else if accountDeleted && mode === 'login'}
		<Alert.Root>
			<CircleCheckIcon />
			<Alert.Title>Your account was deleted</Alert.Title>
			<Alert.Description>We've emailed you a confirmation. Thanks for using Fronko.</Alert.Description>
		</Alert.Root>
	{:else if passwordReset && mode === 'login'}
		<Alert.Root>
			<CircleCheckIcon />
			<Alert.Title>Password updated</Alert.Title>
			<Alert.Description>Sign in with your new password.</Alert.Description>
		</Alert.Root>
	{:else if expired}
		<Alert.Root>
			<CircleAlertIcon />
			<Alert.Title>Your session expired</Alert.Title>
			<Alert.Description>Please sign in again to continue.</Alert.Description>
		</Alert.Root>
	{/if}

	{#if mode === 'login' && ssoMode}
		<form onsubmit={handleSSO}>
			<Field.Group>
				<Field.Field>
					<Field.Label for="sso-identifier">Work email or organisation</Field.Label>
					<Input
						id="sso-identifier"
						autocomplete="email"
						autocapitalize="none"
						spellcheck={false}
						required
						placeholder="you@company.com"
						bind:value={ssoIdentifier}
						disabled={ssoLoading}
					/>
					<Field.Description>
						We'll send you to your company's sign-in page. If your email isn't recognised, enter your organisation's
						Fronko handle (the part after /p/ in your card links).
					</Field.Description>
				</Field.Field>
				<Button type="submit" size="lg" class="w-full" disabled={ssoLoading || !ssoIdentifier.trim()}>
					{#if ssoLoading}<Spinner data-icon="inline-start" />{/if}
					Continue
				</Button>
				<Button type="button" variant="ghost" class="w-full" onclick={() => (ssoMode = false)} disabled={ssoLoading}>
					Sign in with a password instead
				</Button>
			</Field.Group>
		</form>
	{:else}
		<form onsubmit={handleSubmit}>
			<Field.Group>
				<Field.Field data-invalid={!!usernameError || undefined}>
					<Field.Label for="username">{mode === 'login' ? 'Username or email' : 'Username'}</Field.Label>
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
				{#if mode === 'register'}
					<Field.Field data-invalid={!!emailError || undefined}>
						<Field.Label for="email">Email</Field.Label>
						<Input
							id="email"
							type="email"
							autocomplete="email"
							autocapitalize="none"
							spellcheck={false}
							required
							bind:value={email}
							disabled={loading}
							aria-invalid={!!emailError || undefined}
						/>
						{#if emailError}
							<Field.Error>{emailError}</Field.Error>
						{:else}
							<Field.Description>We'll send a code to confirm it.</Field.Description>
						{/if}
					</Field.Field>
					<Field.Field>
						<Field.Label for="organization">
							Company or team <span class="text-muted-foreground font-normal">(optional)</span>
						</Field.Label>
						<Input
							id="organization"
							autocomplete="organization"
							maxlength={80}
							bind:value={organization}
							disabled={loading}
							placeholder="Acme Inc."
						/>
						<Field.Description>
							Leave it blank to use your username. You can add your team's accounts once you're in.
						</Field.Description>
					</Field.Field>
				{/if}
				<Field.Field data-invalid={!!passwordError || undefined}>
					<div class="flex items-center justify-between gap-2">
						<Field.Label for="password">Password</Field.Label>
						{#if mode === 'login'}
							<a
								href="/forgot-password"
								class="text-muted-foreground hover:text-foreground text-sm underline-offset-4 hover:underline"
							>
								Forgot password?
							</a>
						{/if}
					</div>
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
				<Button type="submit" size="lg" class="w-full" disabled={loading || !canSubmit}>
					{#if loading}
						<Spinner data-icon="inline-start" />
					{/if}
					{mode === 'login' ? 'Sign in' : 'Create account'}
				</Button>
				{#if mode === 'login'}
					<Field.Separator>or</Field.Separator>
					<Button
						type="button"
						variant="outline"
						size="lg"
						class="w-full"
						disabled={loading || ssoLoading}
						onclick={() => {
							ssoMode = true;
							error = '';
							if (username.includes('@')) ssoIdentifier = username.trim();
						}}
					>
						<KeyRoundIcon data-icon="inline-start" />
						Sign in with SSO
					</Button>
				{/if}
			</Field.Group>
		</form>
	{/if}
</AuthLayout>
