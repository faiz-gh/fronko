<script lang="ts">
	import { goto } from '$app/navigation';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import KeyRoundIcon from '@lucide/svelte/icons/key-round';
	import { ApiError } from '$lib/api/client';
	import { forgotPassword, resetPassword } from '$lib/api/auth';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Alert from '$lib/components/ui/alert';
	import * as Field from '$lib/components/ui/field';
	import AuthLayout from '$lib/components/app/auth-layout.svelte';
	import CodeInput from '$lib/components/app/code-input.svelte';
	import { Cooldown, RESEND_COOLDOWN_SECONDS } from '$lib/cooldown.svelte';

	let step = $state<'email' | 'reset'>('email');
	let email = $state('');
	let code = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');
	let notice = $state('');
	let busy = $state<'send' | 'reset' | null>(null);
	const resend = new Cooldown();
	$effect(() => () => resend.stop());

	const passwordError = $derived(password.length > 0 && password.length < 8 ? 'At least 8 characters.' : '');
	const confirmError = $derived(confirm.length > 0 && confirm !== password ? "Passwords don't match." : '');
	const canReset = $derived(code.length === 6 && password.length >= 8 && confirm === password && !busy);

	async function sendCode(event?: SubmitEvent) {
		event?.preventDefault();
		error = '';
		notice = '';
		busy = 'send';
		try {
			await forgotPassword(email.trim());
			step = 'reset';
			notice = `If ${email.trim()} belongs to a verified Fronko account, we've sent it a code.`;
			resend.start(RESEND_COOLDOWN_SECONDS);
		} catch (e) {
			if (e instanceof ApiError && e.status === 429 && e.retryAfter) resend.start(e.retryAfter);
			error = e instanceof Error ? e.message : 'Something went wrong';
		} finally {
			busy = null;
		}
	}

	async function reset(event: SubmitEvent) {
		event.preventDefault();
		if (!canReset) return;
		error = '';
		notice = '';
		busy = 'reset';
		try {
			await resetPassword(email.trim(), code, password);
			await goto('/login?reset=1', { replaceState: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Could not reset your password';
		} finally {
			busy = null;
		}
	}
</script>

<svelte:head>
	<title>Reset password · Fronko</title>
</svelte:head>

<AuthLayout>
	<div class="flex flex-col gap-3">
		<span class="bg-muted grid size-11 place-items-center rounded-full">
			<KeyRoundIcon class="size-5" />
		</span>
		<div class="flex flex-col gap-1">
			<h1 class="text-3xl font-semibold tracking-tight">
				{step === 'email' ? 'Forgot your password?' : 'Choose a new password'}
			</h1>
			<p class="text-muted-foreground">
				{step === 'email'
					? "Enter your account's email and we'll send you a code to reset it."
					: 'Enter the 6-digit code from the email and your new password.'}
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
			<Alert.Description>{notice}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if step === 'email'}
		<form onsubmit={sendCode}>
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
						bind:value={email}
						disabled={!!busy}
					/>
				</Field.Field>
				<Button type="submit" size="lg" class="w-full" disabled={!!busy || !email.trim() || resend.active}>
					{#if busy === 'send'}<Spinner data-icon="inline-start" />{/if}
					{resend.active ? `Try again in ${resend.remaining}s` : 'Send code'}
				</Button>
			</Field.Group>
		</form>
	{:else}
		<form onsubmit={reset}>
			<Field.Group>
				<Field.Field>
					<Field.Label for="code">Code</Field.Label>
					<CodeInput id="code" bind:value={code} disabled={!!busy} />
				</Field.Field>
				<Field.Field data-invalid={!!passwordError || undefined}>
					<Field.Label for="new-password">New password</Field.Label>
					<Input
						id="new-password"
						type="password"
						autocomplete="new-password"
						required
						bind:value={password}
						disabled={!!busy}
						aria-invalid={!!passwordError || undefined}
					/>
					{#if passwordError}
						<Field.Error>{passwordError}</Field.Error>
					{:else}
						<Field.Description>At least 8 characters. You'll be signed out everywhere else.</Field.Description>
					{/if}
				</Field.Field>
				<Field.Field data-invalid={!!confirmError || undefined}>
					<Field.Label for="confirm-password">Confirm new password</Field.Label>
					<Input
						id="confirm-password"
						type="password"
						autocomplete="new-password"
						required
						bind:value={confirm}
						disabled={!!busy}
						aria-invalid={!!confirmError || undefined}
					/>
					{#if confirmError}
						<Field.Error>{confirmError}</Field.Error>
					{/if}
				</Field.Field>
				<Button type="submit" size="lg" class="w-full" disabled={!canReset}>
					{#if busy === 'reset'}<Spinner data-icon="inline-start" />{/if}
					Reset password
				</Button>
			</Field.Group>
		</form>

		<p class="text-muted-foreground text-sm">
			Didn't get it? Check your spam folder, or
			<button
				type="button"
				class="text-foreground font-medium underline-offset-4 hover:underline disabled:no-underline disabled:opacity-60"
				onclick={() => sendCode()}
				disabled={!!busy || resend.active}
			>
				{resend.active ? `resend in ${resend.remaining}s` : 'send a new code'}</button
			>. Wrong address?
			<button
				type="button"
				class="text-foreground font-medium underline-offset-4 hover:underline"
				onclick={() => {
					step = 'email';
					code = '';
					resend.stop();
					error = '';
					notice = '';
				}}
				disabled={!!busy}
			>
				Start over
			</button>
		</p>
	{/if}

	<a href="/login" class="text-muted-foreground hover:text-foreground text-sm underline-offset-4 hover:underline">
		Back to sign in
	</a>
</AuthLayout>
