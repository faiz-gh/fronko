<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import BanIcon from '@lucide/svelte/icons/ban';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import KeyRoundIcon from '@lucide/svelte/icons/key-round';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import ShieldIcon from '@lucide/svelte/icons/shield';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import UndoIcon from '@lucide/svelte/icons/undo-2';
	import XIcon from '@lucide/svelte/icons/x';
	import { fileUrl, formatBytes, listFiles, type LibraryFile } from '$lib/api/files';
	import {
		deleteOrgUser,
		getFileGrants,
		getOrgUser,
		resetOrgUserPassword,
		ROLE_LABEL,
		setCardAssignee,
		setFileGrants,
		STATUS_LABEL,
		updateOrgUser,
		userStatus,
		type OrgUser,
		type OrgUserPatch
	} from '$lib/api/org';
	import * as Alert from '$lib/components/ui/alert';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Badge } from '$lib/components/ui/badge';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import CardAvatar from '$lib/components/app/card-avatar.svelte';
	import FileThumb from '$lib/components/app/file-thumb.svelte';
	import QuotaInput from '$lib/components/app/quota-input.svelte';
	import StorageMeter from '$lib/components/app/storage-meter.svelte';
	import UserAvatar from '$lib/components/app/user-avatar.svelte';
	import { normalizeCard } from '$lib/card/card';
	import { cards } from '$lib/cards.svelte';
	import { formatDateTime, plural, timeAgo } from '$lib/format';
	import { orgUsers } from '$lib/org-users.svelte';
	import { generatePassword, signInDetails } from '$lib/password';
	import { session } from '$lib/session.svelte';

	const userId = $derived(Number(page.params.id));

	let user = $state<OrgUser | null>(null);
	let loadError = $state('');
	let granted = $state<LibraryFile[] | null>(null);

	$effect(() => {
		if (session.ready && !session.isAdmin) goto('/dashboard', { replaceState: true });
	});

	$effect(() => {
		const id = userId;
		user = null;
		granted = null;
		loadError = '';
		getOrgUser(id)
			.then((u) => {
				user = u;
				quota = u.storage_quota_bytes;
				email = u.email ?? '';
			})
			.catch((e) => (loadError = e instanceof Error ? e.message : 'Failed to load this user'));
		listFiles({ area: 'granted', userId: id, pageSize: 100 })
			.then((res) => (granted = res.files))
			.catch(() => (granted = []));
	});

	const status = $derived(user ? userStatus(user) : null);
	// The owner manages admins; admins manage members. Nobody manages the owner here.
	const canManage = $derived(!!user && user.role !== 'owner' && (session.isOwner || user.role === 'member'));

	const theirCards = $derived((cards.list ?? []).filter((p) => p.assigned_user?.id === userId));
	const otherCards = $derived(
		(cards.list ?? [])
			.filter((p) => p.assigned_user?.id !== userId)
			// Unassigned cards first: they're the likely pick.
			.sort((a, b) => Number(!!a.assigned_user) - Number(!!b.assigned_user))
	);

	function updated(u: OrgUser) {
		user = u;
		orgUsers.upsert(u);
	}

	async function patch(change: OrgUserPatch, success: string) {
		if (!user) return false;
		try {
			updated(await updateOrgUser(user.id, change));
			toast.success(success);
			return true;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Update failed');
			return false;
		}
	}

	// Storage limit
	let quota = $state<number | null>(null);
	let savingQuota = $state(false);
	const quotaChanged = $derived(!!user && quota !== user.storage_quota_bytes);
	async function saveQuota() {
		savingQuota = true;
		await patch({ quota_bytes: quota }, 'Storage limit saved');
		savingQuota = false;
	}

	// Correcting the email is only possible before it's verified.
	let email = $state('');
	let savingEmail = $state(false);
	async function saveEmail(event: SubmitEvent) {
		event.preventDefault();
		savingEmail = true;
		await patch({ email: email.trim() }, `Email updated. A new code was sent to ${email.trim()}.`);
		savingEmail = false;
	}

	// Cards
	async function assign(profileId: number, to: number | null) {
		try {
			const p = await setCardAssignee(profileId, to);
			cards.upsert(p);
			if (user) user.card_count += to === null ? -1 : 1;
			toast.success(to === null ? 'Card returned to the organisation' : `Card assigned to ${user?.username}`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not change the card');
		}
	}

	// Granted files
	async function revoke(file: LibraryFile) {
		try {
			const { users } = await getFileGrants(file.id);
			await setFileGrants(
				file.id,
				users.filter((u) => u.id !== userId).map((u) => u.id)
			);
			granted = granted?.filter((f) => f.id !== file.id) ?? null;
			toast.success('Access removed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not remove access');
		}
	}

	// Reset password
	let resetOpen = $state(false);
	let tempPassword = $state('');
	let resetting = $state(false);
	let resetDone = $state(false);
	function openReset() {
		tempPassword = generatePassword();
		resetDone = false;
		resetOpen = true;
	}
	async function resetPassword(event: SubmitEvent) {
		event.preventDefault();
		if (!user) return;
		resetting = true;
		try {
			updated(await resetOrgUserPassword(user.id, tempPassword));
			resetDone = true;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not reset the password');
		} finally {
			resetting = false;
		}
	}
	async function copyReset() {
		if (!user) return;
		try {
			await navigator.clipboard.writeText(signInDetails(user.username, tempPassword));
			toast.success('Sign-in details copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}

	// Delete
	let deleteOpen = $state(false);
	let deleting = $state(false);
	async function remove() {
		if (!user) return;
		deleting = true;
		try {
			await deleteOrgUser(user.id);
			orgUsers.remove(user.id);
			// Their cards are unassigned now.
			if (session.username) cards.load(session.username, true);
			toast.success(`${user.username} was deleted`);
			deleteOpen = false;
			await goto('/dashboard/users');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not delete this user');
		} finally {
			deleting = false;
		}
	}
</script>

<svelte:head>
	<title>{user?.username ?? 'User'} · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-5xl flex-col gap-8 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<a href="/dashboard/users" class="text-muted-foreground hover:text-foreground flex w-fit items-center gap-1.5 text-sm">
		<ArrowLeftIcon class="size-4" />
		Users
	</a>

	{#if loadError}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load this user</p>
			<p class="text-muted-foreground text-sm">{loadError}</p>
		</div>
	{:else if !user || !status}
		<div class="flex items-center gap-4">
			<Skeleton class="size-14 rounded-full" />
			<div class="flex flex-col gap-2">
				<Skeleton class="h-5 w-40" />
				<Skeleton class="h-3 w-56" />
			</div>
		</div>
		<Skeleton class="h-40 rounded-xl" />
	{:else}
		<header class="flex flex-wrap items-start justify-between gap-4">
			<div class="flex min-w-0 items-center gap-4">
				<UserAvatar username={user.username} class="size-14 text-lg" />
				<div class="flex min-w-0 flex-col gap-1">
					<h1 class="flex flex-wrap items-center gap-2 text-2xl font-semibold tracking-tight">
						<span class="truncate">{user.username}</span>
						<Badge variant="secondary">{ROLE_LABEL[user.role]}</Badge>
						{#if status !== 'active'}
							<Badge variant={status === 'suspended' ? 'destructive' : 'outline'}>{STATUS_LABEL[status]}</Badge>
						{/if}
					</h1>
					<p class="text-muted-foreground truncate text-sm">
						{user.email}
						· {user.last_login_at ? `Last signed in ${timeAgo(user.last_login_at)}` : 'Hasn’t signed in yet'}
					</p>
				</div>
			</div>
			{#if canManage}
				<div class="flex gap-2">
					<Button variant="outline" href="/dashboard/leads?user={user.id}">
						<InboxIcon data-icon="inline-start" />
						{plural(user.lead_count, 'lead')}
					</Button>
					<DropdownMenu.Root>
						<DropdownMenu.Trigger class={buttonVariants({ variant: 'outline', size: 'icon' })} aria-label="More actions">
							<EllipsisIcon />
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end" class="w-56">
							<DropdownMenu.Group>
								<DropdownMenu.Item onSelect={openReset}>
									<KeyRoundIcon />
									Reset password
								</DropdownMenu.Item>
								{#if session.isOwner}
									<DropdownMenu.Item
										onSelect={() =>
											user &&
											patch(
												{ role: user.role === 'admin' ? 'member' : 'admin' },
												user.role === 'admin' ? `${user.username} is now a member` : `${user.username} is now an admin`
											)}
									>
										<ShieldIcon />
										{user.role === 'admin' ? 'Make member' : 'Make admin'}
									</DropdownMenu.Item>
								{/if}
								{#if user.suspended_at}
									<DropdownMenu.Item onSelect={() => patch({ suspended: false }, 'Account restored')}>
										<UndoIcon />
										Restore account
									</DropdownMenu.Item>
								{:else}
									<DropdownMenu.Item
										onSelect={() => patch({ suspended: true }, `${user?.username} is suspended and was signed out`)}
									>
										<BanIcon />
										Suspend
									</DropdownMenu.Item>
								{/if}
							</DropdownMenu.Group>
							<DropdownMenu.Separator />
							<DropdownMenu.Group>
								<DropdownMenu.Item variant="destructive" onSelect={() => (deleteOpen = true)}>
									<Trash2Icon />
									Delete user
								</DropdownMenu.Item>
							</DropdownMenu.Group>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				</div>
			{/if}
		</header>

		{#if !canManage}
			<Alert.Root>
				<ShieldIcon />
				<Alert.Title>Only {session.orgName}'s owner can manage this account.</Alert.Title>
			</Alert.Root>
		{:else if status === 'suspended'}
			<Alert.Root variant="destructive">
				<BanIcon />
				<Alert.Title>Suspended {user.suspended_at ? timeAgo(user.suspended_at) : ''}</Alert.Title>
				<Alert.Description>
					They can't sign in. Their cards stay live and keep collecting leads.
				</Alert.Description>
			</Alert.Root>
		{:else if status === 'unverified'}
			<section class="bg-card flex flex-col gap-4 rounded-xl border p-5">
				<div class="flex flex-col gap-1">
					<h2 class="font-semibold">Waiting for them to confirm their email</h2>
					<p class="text-muted-foreground text-sm">
						They'll get a code when they first sign in. If the address is wrong, correct it here; you can't change it
						after they confirm it.
					</p>
				</div>
				<form class="flex flex-wrap items-end gap-2" onsubmit={saveEmail}>
					<Field.Field class="max-w-sm flex-1">
						<Field.Label for="user-email">Email</Field.Label>
						<Input id="user-email" type="email" bind:value={email} required disabled={savingEmail} />
					</Field.Field>
					<Button type="submit" variant="outline" disabled={savingEmail || email.trim() === user.email}>
						{#if savingEmail}<Spinner data-icon="inline-start" />{/if}
						Save email
					</Button>
				</form>
			</section>
		{:else if status === 'temporary_password'}
			<Alert.Root>
				<KeyRoundIcon />
				<Alert.Title>They haven't replaced their temporary password yet</Alert.Title>
				<Alert.Description>They'll be asked to choose one the next time they sign in.</Alert.Description>
			</Alert.Root>
		{/if}

		<section class="flex flex-col gap-3" aria-labelledby="cards-heading">
			<div class="flex items-center justify-between gap-3">
				<h2 id="cards-heading" class="text-sm font-semibold">
					Cards <span class="text-muted-foreground tabular ml-1 font-normal">{theirCards.length}</span>
				</h2>
				{#if canManage}
					<DropdownMenu.Root>
						<DropdownMenu.Trigger class={buttonVariants({ variant: 'outline', size: 'sm' })} disabled={otherCards.length === 0}>
							<PlusIcon data-icon="inline-start" />
							Assign a card
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end" class="max-h-80 w-72">
							{#each otherCards as profile (profile.id)}
								{@const card = normalizeCard(profile.data)}
								<DropdownMenu.Item onSelect={() => assign(profile.id, userId)}>
									<CardAvatar {card} fallback={profile.slug} class="size-6 text-[10px]" />
									<span class="flex min-w-0 flex-1 flex-col">
										<span class="truncate">{card.name || profile.slug}</span>
										<span class="text-muted-foreground truncate text-xs">
											{profile.assigned_user ? `From ${profile.assigned_user.username}` : 'Unassigned'}
										</span>
									</span>
								</DropdownMenu.Item>
							{/each}
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				{/if}
			</div>
			<div class="bg-card overflow-hidden rounded-xl border">
				{#if cards.list === null}
					<div class="p-5"><Skeleton class="h-10" /></div>
				{:else}
					<ul class="divide-y">
						{#each theirCards as profile (profile.id)}
							{@const card = normalizeCard(profile.data)}
							<li class="flex items-center gap-3 px-5 py-3">
								<CardAvatar {card} fallback={profile.slug} />
								<a href="/dashboard/{profile.id}" class="flex min-w-0 flex-1 flex-col hover:underline">
									<span class="truncate text-sm font-medium">{card.name || profile.slug}</span>
									<span class="text-muted-foreground truncate font-mono text-[11px]">/p/{profile.slug}</span>
								</a>
								{#if canManage}
									<Button variant="ghost" size="sm" onclick={() => assign(profile.id, null)}>Unassign</Button>
								{/if}
							</li>
						{:else}
							<li class="text-muted-foreground px-5 py-8 text-center text-sm">
								No cards yet. Assign one so they can edit it and collect leads.
							</li>
						{/each}
					</ul>
				{/if}
			</div>
			<p class="text-muted-foreground text-xs">
				Leads stay with whoever held the card when they arrived, so reassigning a card doesn't move past leads.
			</p>
		</section>

		<section class="flex flex-col gap-3" aria-labelledby="storage-heading">
			<h2 id="storage-heading" class="text-sm font-semibold">Storage</h2>
			<div class="bg-card flex flex-col gap-5 rounded-xl border p-5">
				<StorageMeter used={user.used_bytes} quota={user.storage_quota_bytes} class="max-w-md" />
				{#if canManage}
					<Field.Field>
						<Field.Label for="user-quota">Limit for their own files</Field.Label>
						<div class="flex flex-wrap items-center gap-3">
							<QuotaInput id="user-quota" bind:value={quota} disabled={savingQuota} />
							<Button variant="outline" onclick={saveQuota} disabled={!quotaChanged || savingQuota}>
								{#if savingQuota}<Spinner data-icon="inline-start" />{/if}
								Save limit
							</Button>
						</div>
						<Field.Description>
							Shared files don't count. Lowering the limit below what they use only stops new uploads.
						</Field.Description>
					</Field.Field>
				{/if}
			</div>
		</section>

		<section class="flex flex-col gap-3" aria-labelledby="files-heading">
			<div class="flex flex-col gap-0.5">
				<h2 id="files-heading" class="text-sm font-semibold">Extra files they can use</h2>
				<p class="text-muted-foreground text-sm">
					Everyone sees their own files and the shared area. Give access to more from
					<a href="/dashboard/files" class="text-foreground underline-offset-4 hover:underline">Files</a>.
				</p>
			</div>
			<div class="bg-card overflow-hidden rounded-xl border">
				{#if granted === null}
					<div class="p-5"><Skeleton class="h-10" /></div>
				{:else}
					<ul class="divide-y">
						{#each granted as file (file.id)}
							<li class="flex items-center gap-3 px-5 py-3">
								<a href={fileUrl(file.id)} target="_blank" rel="noopener" class="shrink-0">
									<FileThumb id={file.id} kind={file.kind} class="size-10 rounded-md" />
								</a>
								<span class="flex min-w-0 flex-1 flex-col">
									<span class="truncate text-sm font-medium">{file.title || file.name}</span>
									<span class="text-muted-foreground truncate text-xs">
										{formatBytes(file.size_bytes)} ·
										{file.area === 'personal' ? `${file.owner?.username}’s file` : 'Organisation file'}
									</span>
								</span>
								<Button variant="ghost" size="icon-sm" onclick={() => revoke(file)} aria-label="Remove access">
									<XIcon />
								</Button>
							</li>
						{:else}
							<li class="text-muted-foreground px-5 py-8 text-center text-sm">Nothing extra shared with them.</li>
						{/each}
					</ul>
				{/if}
			</div>
		</section>

		<p class="text-muted-foreground text-xs">
			Added {formatDateTime(user.created_at)}.
		</p>
	{/if}
</div>

<Dialog.Root bind:open={resetOpen}>
	<Dialog.Content class="sm:max-w-md">
		{#if resetDone && user}
			<Dialog.Header>
				<Dialog.Title class="text-lg">Password reset</Dialog.Title>
				<Dialog.Description>
					{user.username} was signed out everywhere. Share the new temporary password with them; they'll choose their own
					when they sign in.
				</Dialog.Description>
			</Dialog.Header>
			<dl class="bg-muted/50 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 rounded-lg border p-4 text-sm">
				<dt class="text-muted-foreground">Username</dt>
				<dd class="font-medium break-all">{user.username}</dd>
				<dt class="text-muted-foreground">Temporary password</dt>
				<dd class="font-mono font-medium break-all">{tempPassword}</dd>
			</dl>
			<Dialog.Footer>
				<Button variant="outline" onclick={copyReset}>
					<CopyIcon data-icon="inline-start" />
					Copy sign-in details
				</Button>
				<Button onclick={() => (resetOpen = false)}>Done</Button>
			</Dialog.Footer>
		{:else}
			<form onsubmit={resetPassword} class="flex flex-col gap-6">
				<Dialog.Header>
					<Dialog.Title class="text-lg">Reset {user?.username}'s password</Dialog.Title>
					<Dialog.Description>
						They'll be signed out everywhere and asked to choose a new password after signing in with this one.
					</Dialog.Description>
				</Dialog.Header>
				<Field.Field>
					<Field.Label for="temp-password">Temporary password</Field.Label>
					<div class="flex gap-2">
						<Input id="temp-password" class="font-mono" autocomplete="off" bind:value={tempPassword} minlength={8} required />
						<Button
							type="button"
							variant="outline"
							size="icon"
							onclick={() => (tempPassword = generatePassword())}
							aria-label="Generate a new password"
						>
							<RefreshCwIcon />
						</Button>
					</div>
				</Field.Field>
				<Dialog.Footer>
					<Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close>
					<Button type="submit" disabled={resetting || tempPassword.length < 8}>
						{#if resetting}<Spinner data-icon="inline-start" />{/if}
						Reset password
					</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>

<AlertDialog.Root bind:open={deleteOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete {user?.username}?</AlertDialog.Title>
			<AlertDialog.Description>
				Their account is removed, but their work stays: their {plural(user?.card_count ?? 0, 'card')} go back to the
				organisation, and their {formatBytes(user?.used_bytes ?? 0)} of files move into the organisation's files, so
				cards using them keep working. Leads they collected stay on the cards. To keep the account but block sign-in,
				suspend it instead.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" onclick={remove} disabled={deleting}>
				{#if deleting}<Spinner data-icon="inline-start" />{/if}
				Delete user
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
