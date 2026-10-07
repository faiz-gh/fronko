<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import { createOrgUser, getOrganization, type OrgUser } from '$lib/features/orgs/api';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Switch } from '$lib/components/ui/switch';
	import QuotaInput from '$lib/features/files/components/quota-input.svelte';
	import TeamMembershipsInput from '$lib/features/teams/components/team-memberships-input.svelte';
	import type { TeamRole } from '$lib/features/teams/api';
	import { teams } from '$lib/features/teams/store.svelte';
	import { orgUsers } from '$lib/features/orgs/users.svelte';
	import { generatePassword, signInDetails } from '$lib/core/password';
	import { session } from '$lib/core/session.svelte';

	let { open = $bindable(false), oncreated }: { open?: boolean; oncreated?: (user: OrgUser) => void } = $props();

	let username = $state('');
	let email = $state('');
	let password = $state(generatePassword());
	let admin = $state(false);
	let quota = $state<number | null>(null);
	let memberships = $state<{ team_id: number; role: TeamRole }[]>([]);
	let creating = $state(false);
	let error = $state('');
	let created = $state<{ username: string; email: string; password: string } | null>(null);

	// Start each new user at the organisation's default limit.
	$effect(() => {
		if (!open) return;
		getOrganization()
			.then((org) => (quota = org.default_quota_bytes))
			.catch(() => {});
	});

	const usernameError = $derived(
		username.length > 0 && !/^[a-zA-Z0-9_.-]{3,32}$/.test(username)
			? '3–32 characters: letters, numbers, ".", "_" or "-".'
			: ''
	);
	const emailError = $derived(
		email.length > 0 && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()) ? 'Enter a valid email address.' : ''
	);
	const passwordError = $derived(password.length > 0 && password.length < 8 ? 'At least 8 characters.' : '');

	function reset() {
		username = '';
		email = '';
		password = generatePassword();
		admin = false;
		memberships = [];
		error = '';
		created = null;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (usernameError || emailError || passwordError) return;
		creating = true;
		error = '';
		try {
			const user = await createOrgUser({
				username: username.trim(),
				email: email.trim(),
				password,
				role: admin ? 'admin' : 'member',
				quota_bytes: quota,
				teams: memberships
			});
			orgUsers.upsert(user);
			if (memberships.length) teams.refresh();
			oncreated?.(user);
			created = { username: user.username, email: user.email ?? email.trim(), password };
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create user';
		} finally {
			creating = false;
		}
	}

	async function copyDetails() {
		if (!created) return;
		try {
			await navigator.clipboard.writeText(signInDetails(created.username, created.password));
			toast.success('Sign-in details copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}
</script>

<Dialog.Root bind:open onOpenChange={(o) => !o && reset()}>
	<Dialog.Content class="max-h-[92svh] overflow-y-auto sm:max-w-lg">
		{#if created}
			<Dialog.Header>
				<span class="bg-brand-soft text-brand mb-2 grid size-10 place-items-center rounded-full">
					<CircleCheckIcon class="size-5" />
				</span>
				<Dialog.Title class="text-lg">{created.username} can sign in now</Dialog.Title>
				<Dialog.Description>
					We emailed their username and temporary password to {created.email}. They'll confirm their email and choose
					their own password the first time they sign in. You can also copy the details below.
				</Dialog.Description>
			</Dialog.Header>
			<dl class="bg-muted/50 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 rounded-lg border p-4 text-sm">
				<dt class="text-muted-foreground">Username</dt>
				<dd class="font-medium break-all">{created.username}</dd>
				<dt class="text-muted-foreground">Temporary password</dt>
				<dd class="font-mono font-medium break-all">{created.password}</dd>
			</dl>
			<Dialog.Footer>
				<Button variant="outline" onclick={copyDetails}>
					<CopyIcon data-icon="inline-start" />
					Copy sign-in details
				</Button>
				<Button onclick={() => (open = false)}>Done</Button>
			</Dialog.Footer>
		{:else}
			<form onsubmit={handleSubmit} class="flex flex-col gap-6">
				<Dialog.Header>
					<Dialog.Title class="text-lg">New user</Dialog.Title>
					<Dialog.Description>
						They'll use {session.orgName}'s storage and work on the cards you assign them.
					</Dialog.Description>
				</Dialog.Header>
				<Field.Group>
					<div class="grid gap-4 sm:grid-cols-2">
						<Field.Field data-invalid={!!usernameError || undefined}>
							<Field.Label for="new-user-username">Username</Field.Label>
							<Input
								id="new-user-username"
								autocomplete="off"
								autocapitalize="none"
								spellcheck={false}
								bind:value={username}
								required
								aria-invalid={!!usernameError || undefined}
							/>
							{#if usernameError}<Field.Error>{usernameError}</Field.Error>{/if}
						</Field.Field>
						<Field.Field data-invalid={!!emailError || undefined}>
							<Field.Label for="new-user-email">Email</Field.Label>
							<Input
								id="new-user-email"
								type="email"
								autocomplete="off"
								autocapitalize="none"
								spellcheck={false}
								bind:value={email}
								required
								aria-invalid={!!emailError || undefined}
							/>
							{#if emailError}<Field.Error>{emailError}</Field.Error>{/if}
						</Field.Field>
					</div>
					<Field.Field data-invalid={!!passwordError || undefined}>
						<Field.Label for="new-user-password">Temporary password</Field.Label>
						<div class="flex gap-2">
							<Input
								id="new-user-password"
								class="font-mono"
								autocomplete="off"
								spellcheck={false}
								bind:value={password}
								required
								aria-invalid={!!passwordError || undefined}
							/>
							<Button
								type="button"
								variant="outline"
								size="icon"
								onclick={() => (password = generatePassword())}
								aria-label="Generate a new password"
							>
								<RefreshCwIcon />
							</Button>
						</div>
						{#if passwordError}
							<Field.Error>{passwordError}</Field.Error>
						{:else}
							<Field.Description
								>We'll email it to them. They replace it after confirming their email.</Field.Description
							>
						{/if}
					</Field.Field>
					<Field.Field>
						<Field.Label for="new-user-quota">Storage for their own files</Field.Label>
						<QuotaInput id="new-user-quota" bind:value={quota} />
						<Field.Description>Shared files don't count toward this.</Field.Description>
					</Field.Field>
					{#if teams.list?.length}
						<Field.Field>
							<Field.Label>Teams</Field.Label>
							<TeamMembershipsInput bind:value={memberships} />
							<Field.Description
								>They'll see their teams' files. Leads also see their teammates' cards and leads.</Field.Description
							>
						</Field.Field>
					{/if}
					{#if session.isOwner}
						<Field.Field orientation="horizontal">
							<Switch id="new-user-admin" bind:checked={admin} />
							<Field.Content>
								<Field.Label for="new-user-admin">Admin</Field.Label>
								<Field.Description
									>Admins manage users, every card, lead and file. Only you manage storage.</Field.Description
								>
							</Field.Content>
						</Field.Field>
					{/if}
					{#if error}
						<Field.Error>{error}</Field.Error>
					{/if}
				</Field.Group>
				<Dialog.Footer>
					<Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close>
					<Button type="submit" disabled={creating || !username || !email || !password}>
						{#if creating}<Spinner data-icon="inline-start" />{/if}
						Create user
					</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>
