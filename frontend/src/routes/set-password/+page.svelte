<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import KeyRoundIcon from '@lucide/svelte/icons/key-round';
	import { changePassword } from '$lib/features/auth/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Alert from '$lib/components/ui/alert';
	import * as Field from '$lib/components/ui/field';
	import AuthLayout from '$lib/features/auth/components/auth-layout.svelte';
	import { session } from '$lib/core/session.svelte';

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let busy = $state(false);

	// Only follow same-site relative paths, never "//evil.com" or absolute URLs.
	function nextPath(): string {
		const next = page.url.searchParams.get('next');
		return next && next.startsWith('/') && !next.startsWith('//') ? next : '/dashboard';
	}

	$effect(() => {
		if (session.status === 'anonymous') {
			goto('/login', { replaceState: true });
		} else if (session.isAuthenticated && (!session.emailVerified || !session.mustChangePassword)) {
			goto(session.nextStep(nextPath()), { replaceState: true });
		}
	});

	const tooShort = $derived(newPassword.length > 0 && newPassword.length < 8);
	const mismatch = $derived(confirmPassword.length > 0 && confirmPassword !== newPassword);
	const sameAsTemporary = $derived(newPassword.length > 0 && newPassword === currentPassword);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (tooShort || mismatch || sameAsTemporary || busy) return;
		error = '';
		busy = true;
		try {
			await changePassword(currentPassword, newPassword);
			// The effect above moves on once the flag clears.
			session.mustChangePassword = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Could not set your password';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Choose your password · Fronko</title>
</svelte:head>

<AuthLayout>
	{#if !session.isAuthenticated || !session.mustChangePassword || !session.emailVerified}
		<div class="grid place-items-center py-12">
			<Spinner class="text-muted-foreground size-6" />
		</div>
	{:else}
		<div class="flex flex-col gap-3">
			<span class="bg-muted grid size-11 place-items-center rounded-full">
				<KeyRoundIcon class="size-5" />
			</span>
			<div class="flex flex-col gap-1">
				<h1 class="text-3xl font-semibold tracking-tight">Choose your password</h1>
				<p class="text-muted-foreground">
					{session.orgName} set a temporary password for your account. Replace it with one only you know.
				</p>
			</div>
		</div>

		{#if error}
			<Alert.Root variant="destructive">
				<CircleAlertIcon />
				<Alert.Title>{error}</Alert.Title>
			</Alert.Root>
		{/if}

		<form onsubmit={handleSubmit}>
			<Field.Group>
				<!-- Lets password managers tie the new password to this account. -->
				<input type="text" class="hidden" autocomplete="username" value={session.username ?? ''} readonly />
				<Field.Field>
					<Field.Label for="current-password">Temporary password</Field.Label>
					<Input
						id="current-password"
						type="password"
						autocomplete="current-password"
						required
						bind:value={currentPassword}
						disabled={busy}
					/>
				</Field.Field>
				<Field.Field data-invalid={tooShort || sameAsTemporary || undefined}>
					<Field.Label for="new-password">New password</Field.Label>
					<Input
						id="new-password"
						type="password"
						autocomplete="new-password"
						required
						bind:value={newPassword}
						disabled={busy}
						aria-invalid={tooShort || sameAsTemporary || undefined}
					/>
					{#if tooShort}
						<Field.Error>At least 8 characters.</Field.Error>
					{:else if sameAsTemporary}
						<Field.Error>Choose something different from the temporary password.</Field.Error>
					{:else}
						<Field.Description>At least 8 characters.</Field.Description>
					{/if}
				</Field.Field>
				<Field.Field data-invalid={mismatch || undefined}>
					<Field.Label for="confirm-password">Confirm new password</Field.Label>
					<Input
						id="confirm-password"
						type="password"
						autocomplete="new-password"
						required
						bind:value={confirmPassword}
						disabled={busy}
						aria-invalid={mismatch || undefined}
					/>
					{#if mismatch}
						<Field.Error>Passwords don't match.</Field.Error>
					{/if}
				</Field.Field>
				<Button
					type="submit"
					size="lg"
					class="w-full"
					disabled={busy ||
						!currentPassword ||
						!newPassword ||
						!confirmPassword ||
						tooShort ||
						mismatch ||
						sameAsTemporary}
				>
					{#if busy}<Spinner data-icon="inline-start" />{/if}
					Set password and continue
				</Button>
			</Field.Group>
		</form>

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
