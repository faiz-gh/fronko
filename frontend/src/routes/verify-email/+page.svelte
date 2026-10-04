<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import MailIcon from '@lucide/svelte/icons/mail';
	import { ApiError } from '$lib/api/client';
	import { resendVerification, setEmail, verifyEmail } from '$lib/api/auth';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Alert from '$lib/components/ui/alert';
	import * as Field from '$lib/components/ui/field';
	import AuthLayout from '$lib/components/app/auth-layout.svelte';
	import CodeInput from '$lib/components/app/code-input.svelte';
	import { Cooldown, RESEND_COOLDOWN_SECONDS } from '$lib/cooldown.svelte';
	import { session } from '$lib/session.svelte';

	// Accounts from before emails were required start by adding one.
	let editingEmail = $state(false);
	let emailInput = $state('');
	let code = $state('');
	let error = $state('');
	let notice = $state('');
	let busy = $state<'verify' | 'resend' | 'email' | null>(null);
	const resend = new Cooldown();
	$effect(() => () => resend.stop());

	const addingEmail = $derived(editingEmail || (session.isAuthenticated && !session.email));

	// Only follow same-site relative paths, never "//evil.com" or absolute URLs.
	function nextPath(): string {
		const next = page.url.searchParams.get('next');
		return next && next.startsWith('/') && !next.startsWith('//') ? next : '/dashboard';
	}

	$effect(() => {
		if (session.status === 'anonymous') {
			goto('/login', { replaceState: true });
		} else if (session.isAuthenticated && session.emailVerified) {
			goto(nextPath(), { replaceState: true });
		}
	});

	function message(e: unknown, fallback: string) {
		return e instanceof Error ? e.message : fallback;
	}

	async function submitCode(value = code) {
		if (value.length !== 6 || busy) return;
		error = '';
		notice = '';
		busy = 'verify';
		try {
			// The effect above moves on once the session is verified.
			session.signIn(await verifyEmail(value));
		} catch (e) {
			error = message(e, 'Could not verify the code');
			code = '';
		} finally {
			busy = null;
		}
	}

	async function resendCode() {
		error = '';
		notice = '';
		busy = 'resend';
		try {
			await resendVerification();
			notice = `We sent a new code to ${session.email}.`;
			resend.start(RESEND_COOLDOWN_SECONDS);
		} catch (e) {
			if (e instanceof ApiError && e.status === 429 && e.retryAfter) resend.start(e.retryAfter);
			else error = message(e, 'Could not send a new code');
		} finally {
			busy = null;
		}
	}

	async function saveEmail(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		notice = '';
		busy = 'email';
		try {
			session.signIn(await setEmail(emailInput.trim()));
			editingEmail = false;
			code = '';
			notice = `We sent a code to ${session.email}.`;
			resend.start(RESEND_COOLDOWN_SECONDS);
		} catch (e) {
			error = message(e, 'Could not save your email');
		} finally {
			busy = null;
		}
	}

	function startEditing() {
		emailInput = session.email ?? '';
		error = '';
		notice = '';
		editingEmail = true;
	}
</script>

<svelte:head>
	<title>Verify your email · Fronko</title>
</svelte:head>

<AuthLayout>
	{#if !session.isAuthenticated || session.emailVerified}
		<div class="grid place-items-center py-12">
			<Spinner class="text-muted-foreground size-6" />
		</div>
	{:else}
		<div class="flex flex-col gap-3">
			<span class="bg-muted grid size-11 place-items-center rounded-full">
				<MailIcon class="size-5" />
			</span>
			<div class="flex flex-col gap-1">
				<h1 class="text-3xl font-semibold tracking-tight">
					{addingEmail && !session.email ? 'Add your email' : 'Check your email'}
				</h1>
				<p class="text-muted-foreground">
					{#if addingEmail && !session.email}
						Your account needs a verified email so you can recover it if you forget your password.
					{:else if addingEmail}
						Enter the address you want to use. We'll send a new code to it.
					{:else}
						Enter the 6-digit code we sent to <span class="text-foreground font-medium break-all">{session.email}</span>.
					{/if}
				</p>
			</div>
		</div>

		{#if error}
			<Alert.Root variant="destructive">
				<CircleAlertIcon />
				<Alert.Title>{error}</Alert.Title>
			</Alert.Root>
		{:else if notice}
			<Alert.Root>
				<CircleCheckIcon />
				<Alert.Title>{notice}</Alert.Title>
			</Alert.Root>
		{/if}

		{#if addingEmail}
			<form onsubmit={saveEmail}>
				<Field.Group>
					<Field.Field>
						<Field.Label for="email">Email</Field.Label>
						<Input
							id="email"
							type="email"
							autocomplete="email"
							autocapitalize="none"
							spellcheck={false}
							required
							bind:value={emailInput}
							disabled={!!busy}
						/>
					</Field.Field>
					<div class="flex flex-col gap-2">
						<Button type="submit" size="lg" class="w-full" disabled={!!busy || !emailInput.trim()}>
							{#if busy === 'email'}<Spinner data-icon="inline-start" />{/if}
							Send code
						</Button>
						{#if session.email}
							<Button type="button" variant="ghost" class="w-full" onclick={() => (editingEmail = false)} disabled={!!busy}>
								Cancel
							</Button>
						{/if}
					</div>
				</Field.Group>
			</form>
		{:else}
			<form
				onsubmit={(e) => {
					e.preventDefault();
					submitCode();
				}}
			>
				<Field.Group>
					<Field.Field>
						<Field.Label for="code" class="sr-only">Verification code</Field.Label>
						<CodeInput id="code" bind:value={code} disabled={!!busy} invalid={!!error} oncomplete={submitCode} />
					</Field.Field>
					<Button type="submit" size="lg" class="w-full" disabled={!!busy || code.length !== 6}>
						{#if busy === 'verify'}<Spinner data-icon="inline-start" />{/if}
						Verify email
					</Button>
				</Field.Group>
			</form>

			<div class="text-muted-foreground flex flex-col gap-1 text-sm">
				<p>
					Didn't get it? Check your spam folder, or
					<button
						type="button"
						class="text-foreground font-medium underline-offset-4 hover:underline disabled:no-underline disabled:opacity-60"
						onclick={resendCode}
						disabled={!!busy || resend.active}
					>
						{resend.active ? `resend in ${resend.remaining}s` : 'send a new code'}</button
					>.
				</p>
				<p>
					Wrong address?
					<button
						type="button"
						class="text-foreground font-medium underline-offset-4 hover:underline"
						onclick={startEditing}
						disabled={!!busy}
					>
						Change email
					</button>
				</p>
			</div>
		{/if}

		<p class="text-muted-foreground text-sm">
			Signed in as {session.username}.
			<button
				type="button"
				class="text-foreground font-medium underline-offset-4 hover:underline"
				onclick={() => session.signOut()}
			>
				Sign out
			</button>
		</p>
	{/if}
</AuthLayout>
