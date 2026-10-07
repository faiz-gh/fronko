<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import * as Alert from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { session } from '$lib/core/session.svelte';
	import { ROLE_LABEL } from '$lib/features/orgs/api';
	import { changePassword } from '../api';
	import ChangeEmailForm from './change-email-form.svelte';

	let changingEmail = $state(false);

	// Change password
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let changingPassword = $state(false);
	let passwordMessage = $state('');

	const newPasswordError = $derived(newPassword.length > 0 && newPassword.length < 8 ? 'At least 8 characters.' : '');
	const confirmPasswordError = $derived(
		confirmPassword.length > 0 && confirmPassword !== newPassword ? "Passwords don't match." : ''
	);
	const canChangePassword = $derived(
		!!currentPassword && newPassword.length >= 8 && confirmPassword === newPassword && !changingPassword
	);

	async function submitPassword(event: SubmitEvent) {
		event.preventDefault();
		if (!canChangePassword) return;
		changingPassword = true;
		passwordMessage = '';
		try {
			await changePassword(currentPassword, newPassword);
			currentPassword = newPassword = confirmPassword = '';
			toast.success('Password changed. Other sessions were signed out.');
		} catch (e) {
			passwordMessage = e instanceof Error ? e.message : 'Failed to change password';
		} finally {
			changingPassword = false;
		}
	}
</script>

<FormSection id="account" title="Account" description="How you sign in. Either works on the sign-in page.">
	<dl class="grid gap-5 text-sm sm:grid-cols-2">
		<div class="flex flex-col gap-1.5">
			<dt class="text-muted-foreground">Username</dt>
			<dd class="font-medium">{session.username}</dd>
		</div>
		{#if !session.isOwner}
			<div class="flex flex-col gap-1.5">
				<dt class="text-muted-foreground">Organisation</dt>
				<dd class="font-medium">{session.orgName} · {session.role ? ROLE_LABEL[session.role] : ''}</dd>
			</div>
		{/if}
		<div class="flex flex-col gap-1.5">
			<dt class="text-muted-foreground">Email</dt>
			<dd class="flex flex-wrap items-center gap-2 font-medium">
				<span class="break-all">{session.email}</span>
				{#if session.emailVerified}
					<Badge variant="secondary" class="gap-1">
						<CircleCheckIcon class="size-3" />
						Verified
					</Badge>
				{/if}
				{#if session.isOwner && !changingEmail}
					<Button variant="link" size="sm" class="h-auto px-0" onclick={() => (changingEmail = true)}>Change</Button>
				{/if}
			</dd>
			{#if !session.isOwner}
				<dd class="text-muted-foreground text-xs">Managed by {session.orgName}. Ask them if it needs to change.</dd>
			{/if}
		</div>
	</dl>
	{#if changingEmail}
		<div class="mt-5">
			<ChangeEmailForm onclose={() => (changingEmail = false)} />
		</div>
	{/if}
</FormSection>

{#if session.hasPassword}
	<FormSection
		id="password"
		title="Password"
		description="Changing it signs you out on every other device. You stay signed in here."
	>
		<form onsubmit={submitPassword} class="flex flex-col gap-6">
			<Field.Group class="grid gap-5 sm:grid-cols-2">
				<Field.Field class="sm:col-span-2 sm:max-w-[calc(50%-0.625rem)]">
					<Field.Label for="current-password">Current password</Field.Label>
					<Input
						id="current-password"
						type="password"
						autocomplete="current-password"
						bind:value={currentPassword}
						disabled={changingPassword}
					/>
				</Field.Field>
				<Field.Field data-invalid={!!newPasswordError || undefined}>
					<Field.Label for="new-password">New password</Field.Label>
					<Input
						id="new-password"
						type="password"
						autocomplete="new-password"
						bind:value={newPassword}
						disabled={changingPassword}
						aria-invalid={!!newPasswordError || undefined}
					/>
					{#if newPasswordError}
						<Field.Error>{newPasswordError}</Field.Error>
					{:else}
						<Field.Description>At least 8 characters.</Field.Description>
					{/if}
				</Field.Field>
				<Field.Field data-invalid={!!confirmPasswordError || undefined}>
					<Field.Label for="confirm-password">Confirm new password</Field.Label>
					<Input
						id="confirm-password"
						type="password"
						autocomplete="new-password"
						bind:value={confirmPassword}
						disabled={changingPassword}
						aria-invalid={!!confirmPasswordError || undefined}
					/>
					{#if confirmPasswordError}
						<Field.Error>{confirmPasswordError}</Field.Error>
					{/if}
				</Field.Field>
			</Field.Group>

			{#if passwordMessage}
				<Alert.Root variant="destructive">
					<CircleAlertIcon />
					<Alert.Title>{passwordMessage}</Alert.Title>
				</Alert.Root>
			{/if}

			<div>
				<Button type="submit" disabled={!canChangePassword}>
					{#if changingPassword}<Spinner data-icon="inline-start" />{/if}
					Change password
				</Button>
			</div>
		</form>
	</FormSection>
{:else}
	<FormSection id="password" title="Password" description="How you sign in.">
		<p class="text-muted-foreground text-sm">
			You sign in with your organisation's single sign-on, so your Fronko account has no password.
		</p>
	</FormSection>
{/if}
