<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import * as Alert from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { session } from '$lib/core/session.svelte';
	import { ROLE_LABEL } from '$lib/features/orgs/api';
	import { changePassword, exportMyData } from '../api';
	import ChangeEmailForm from './change-email-form.svelte';
	import DeleteAccountDialog from './delete-account-dialog.svelte';

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

	// Your data: a JSON copy of everything held about you.
	let exporting = $state(false);
	async function downloadData() {
		exporting = true;
		try {
			const data = await exportMyData();
			const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = `fronko-${session.username}-data.json`;
			a.click();
			URL.revokeObjectURL(url);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Couldn't download your data");
		} finally {
			exporting = false;
		}
	}

	let deleteOpen = $state(false);

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

<FormSection
	id="your-data"
	title="Your data"
	description="A copy of everything Fronko holds about you, in a format other tools can read."
>
	<div class="flex flex-col items-start gap-3">
		<p class="text-muted-foreground text-sm">
			Your account, teams, the cards you made or hold, the leads they collected, your files' details (not the files
			themselves), your personal integrations and any feedback you sent, as one JSON file.
		</p>
		<Button variant="outline" onclick={downloadData} disabled={exporting}>
			{#if exporting}<Spinner data-icon="inline-start" />{:else}<DownloadIcon data-icon="inline-start" />{/if}
			Download my data
		</Button>
	</div>
</FormSection>

<FormSection id="danger-zone" title="Danger zone" description="Permanent actions. Read the warnings carefully.">
	<div class="border-destructive/40 bg-destructive/5 flex flex-col gap-4 rounded-lg border p-4">
		<div class="flex items-start gap-3">
			<TriangleAlertIcon class="text-destructive mt-0.5 size-5 shrink-0" />
			<div class="flex flex-col gap-1 text-sm">
				<p class="font-medium">Delete your account</p>
				<p class="text-muted-foreground text-pretty">
					{#if session.isOwner}
						You own {session.orgName}. You'll first hand it to someone else, or delete it with everything in it: every
						card, lead, file and person.
					{:else}
						You lose access straight away. Your cards, leads and files stay with {session.orgName}.
					{/if}
					This can't be undone.
				</p>
			</div>
		</div>
		<div>
			<Button variant="destructive" onclick={() => (deleteOpen = true)}>Delete account…</Button>
		</div>
	</div>
</FormSection>

<DeleteAccountDialog bind:open={deleteOpen} onexport={downloadData} {exporting} />
