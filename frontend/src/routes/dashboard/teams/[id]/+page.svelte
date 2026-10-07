<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import IdCardIcon from '@lucide/svelte/icons/id-card';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import XIcon from '@lucide/svelte/icons/x';
	import { me } from '$lib/features/auth/api';
	import { ROLE_LABEL } from '$lib/features/orgs/api';
	import {
		deleteTeam,
		setTeamMembers,
		teamColor,
		TEAM_ROLE_LABEL,
		type TeamDetail,
		type TeamMember,
		type TeamRole
	} from '$lib/features/teams/api';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import * as Empty from '$lib/components/ui/empty';
	import * as Popover from '$lib/components/ui/popover';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Table from '$lib/components/ui/table';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import TeamDialog from '$lib/features/teams/components/team-dialog.svelte';
	import UserAvatar from '$lib/components/shared/user-avatar.svelte';
	import { timeAgo } from '$lib/core/format';
	import { orgUsers } from '$lib/features/orgs/users.svelte';
	import { session } from '$lib/core/session.svelte';
	import { teams } from '$lib/features/teams/store.svelte';

	const id = $derived(Number(page.params.id));
	let team = $state<TeamDetail | null>(null);
	let error = $state('');
	let saving = $state(false);
	let editOpen = $state(false);
	let addOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	$effect(() => {
		const teamId = id;
		team = null;
		error = '';
		teams
			.loadDetail(teamId, true)
			.then((t) => {
				if (teamId === id) team = t;
			})
			.catch((e) => (error = e instanceof Error ? e.message : 'Failed to load the team'));
	});

	const leadsIt = $derived(session.ledTeams.some((t) => t.id === id));
	// Admins see everyone's cards and leads; leads see their team's.
	const seesWork = $derived(session.isAdmin || leadsIt);
	const memberIds = $derived(new Set(team?.members.map((m) => m.id) ?? []));
	const addable = $derived((orgUsers.list ?? []).filter((u) => !memberIds.has(u.id)));

	/** Saves the whole member list, then refreshes what depends on it. */
	async function saveMembers(members: { user_id: number; role: TeamRole }[], message: string) {
		if (!team) return;
		saving = true;
		try {
			const updated = await setTeamMembers(team.id, members);
			team = updated;
			teams.details[updated.id] = updated;
			teams.upsert(updated);
			orgUsers.refresh();
			// Our own teams may have changed what we can see.
			const mine = (list: TeamMember[]) => list.some((m) => m.username === session.username);
			if (mine(updated.members) || session.teams.some((t) => t.id === updated.id)) {
				me()
					.then((u) => session.signIn(u))
					.catch(() => {});
			}
			toast.success(message);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not update the team');
		} finally {
			saving = false;
		}
	}

	const current = () => team?.members.map((m) => ({ user_id: m.id, role: m.role })) ?? [];

	function add(userId: number, username: string) {
		addOpen = false;
		saveMembers([...current(), { user_id: userId, role: 'member' }], `${username} added`);
	}

	function remove(m: TeamMember) {
		saveMembers(
			current().filter((x) => x.user_id !== m.id),
			`${m.username} removed`
		);
	}

	function setRole(m: TeamMember, role: TeamRole) {
		if (role === m.role) return;
		saveMembers(
			current().map((x) => (x.user_id === m.id ? { ...x, role } : x)),
			role === 'lead' ? `${m.username} now leads ${team?.name}` : `${m.username} is now a member`
		);
	}

	async function confirmDelete() {
		if (!team) return;
		deleting = true;
		try {
			await deleteTeam(team.id);
			teams.remove(team.id);
			orgUsers.refresh();
			toast.success(`${team.name} deleted`);
			await goto('/dashboard/teams');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not delete the team');
		} finally {
			deleting = false;
		}
	}
</script>

<svelte:head>
	<title>{team?.name ?? 'Team'} · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1200px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<a
		href="/dashboard/teams"
		class="text-muted-foreground hover:text-foreground inline-flex w-fit items-center gap-1.5 text-sm"
	>
		<ArrowLeftIcon class="size-4" />
		Teams
	</a>

	{#if error}
		<Empty.Root class="bg-card rounded-xl border border-dashed py-16">
			<Empty.Header>
				<Empty.Media variant="icon"><UsersRoundIcon /></Empty.Media>
				<Empty.Title>Team not found</Empty.Title>
				<Empty.Description>It may have been deleted, or you're not in it.</Empty.Description>
			</Empty.Header>
		</Empty.Root>
	{:else if !team}
		<Skeleton class="h-24 rounded-xl" />
		<Skeleton class="h-64 rounded-xl" />
	{:else}
		<header class="flex flex-wrap items-start justify-between gap-4">
			<div class="flex min-w-0 items-start gap-4">
				<span
					class="grid size-12 shrink-0 place-items-center rounded-xl text-lg font-semibold text-white"
					style="background: {teamColor(team)}"
					aria-hidden="true"
				>
					{team.name.slice(0, 1).toUpperCase()}
				</span>
				<div class="flex min-w-0 flex-col gap-1">
					<h1 class="truncate text-2xl font-semibold tracking-tight sm:text-3xl">{team.name}</h1>
					<p class="text-muted-foreground text-sm">{team.description || 'No description.'}</p>
				</div>
			</div>
			{#if session.isAdmin}
				<div class="flex gap-2">
					<Button variant="outline" onclick={() => (editOpen = true)}>
						<PencilIcon data-icon="inline-start" />
						Edit
					</Button>
					<Button variant="outline" class="text-destructive hover:text-destructive" onclick={() => (deleteOpen = true)}>
						<Trash2Icon data-icon="inline-start" />
						Delete
					</Button>
				</div>
			{/if}
		</header>

		<!-- Where the team's things are -->
		<div class="grid gap-3 sm:grid-cols-3">
			<a
				href="/dashboard/files?loc=team:{team.id}"
				class="bg-card hover:border-foreground/20 flex items-center gap-3 rounded-xl border p-4 transition-colors"
			>
				<span class="bg-muted grid size-9 place-items-center rounded-lg"><FolderIcon class="size-4" /></span>
				<span class="flex flex-col">
					<span class="text-sm font-medium">Team files</span>
					<span class="text-muted-foreground text-xs">{team.file_count} {team.file_count === 1 ? 'file' : 'files'}</span
					>
				</span>
			</a>
			{#if seesWork}
				<a
					href="/dashboard/cards?team={team.id}"
					class="bg-card hover:border-foreground/20 flex items-center gap-3 rounded-xl border p-4 transition-colors"
				>
					<span class="bg-muted grid size-9 place-items-center rounded-lg"><IdCardIcon class="size-4" /></span>
					<span class="flex flex-col">
						<span class="text-sm font-medium">Cards</span>
						<span class="text-muted-foreground text-xs">Held by people in the team</span>
					</span>
				</a>
				<a
					href="/dashboard/leads?team={team.id}"
					class="bg-card hover:border-foreground/20 flex items-center gap-3 rounded-xl border p-4 transition-colors"
				>
					<span class="bg-muted grid size-9 place-items-center rounded-lg"><InboxIcon class="size-4" /></span>
					<span class="flex flex-col">
						<span class="text-sm font-medium">Leads</span>
						<span class="text-muted-foreground text-xs">From the team's cards</span>
					</span>
				</a>
			{/if}
		</div>

		<section class="flex flex-col gap-3">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div class="flex items-center gap-2">
					<h2 class="text-lg font-semibold">People</h2>
					<span class="text-muted-foreground text-sm">{team.members.length}</span>
					{#if saving}<Spinner class="text-muted-foreground size-4" />{/if}
				</div>
				{#if session.isAdmin}
					<Popover.Root bind:open={addOpen}>
						<Popover.Trigger>
							{#snippet child({ props })}
								<Button {...props} variant="outline" disabled={saving}>
									<UserPlusIcon data-icon="inline-start" />
									Add people
								</Button>
							{/snippet}
						</Popover.Trigger>
						<Popover.Content class="w-72 p-0" align="end">
							<Command.Root>
								<Command.Input placeholder="Search people…" />
								<Command.List>
									<Command.Empty>Everyone's already in this team.</Command.Empty>
									<Command.Group>
										{#each addable as u (u.id)}
											<Command.Item value="{u.username} {u.email ?? ''}" onSelect={() => add(u.id, u.username)}>
												<UserAvatar username={u.username} class="size-6 text-[10px]" />
												<span class="flex min-w-0 flex-col">
													<span class="truncate">{u.username}</span>
													<span class="text-muted-foreground truncate text-xs">{u.email}</span>
												</span>
											</Command.Item>
										{/each}
									</Command.Group>
								</Command.List>
							</Command.Root>
						</Popover.Content>
					</Popover.Root>
				{/if}
			</div>

			{#if team.members.length === 0}
				<Empty.Root class="bg-card rounded-xl border border-dashed py-12">
					<Empty.Header>
						<Empty.Media variant="icon"><UsersRoundIcon /></Empty.Media>
						<Empty.Title>Nobody in this team yet</Empty.Title>
						<Empty.Description>
							{session.isAdmin
								? 'Add people, then make one or more of them leads.'
								: 'Your admins add people to teams.'}
						</Empty.Description>
					</Empty.Header>
				</Empty.Root>
			{:else}
				<div class="bg-card overflow-hidden rounded-xl border">
					<Table.Root>
						<Table.Header>
							<Table.Row class="bg-muted/40 hover:bg-muted/40">
								<Table.Head class="h-10 pl-5">Person</Table.Head>
								<Table.Head class="hidden h-10 sm:table-cell">In the organisation</Table.Head>
								<Table.Head class="hidden h-10 md:table-cell">Added</Table.Head>
								<Table.Head class="h-10 pr-5 text-right">Team role</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each team.members as m (m.id)}
								<Table.Row>
									<Table.Cell class="pl-5">
										<span class="flex items-center gap-3">
											<UserAvatar username={m.username} class="size-8 text-[11px]" />
											<span class="flex min-w-0 flex-col">
												{#if session.isAdmin}
													<a href="/dashboard/users/{m.id}" class="truncate font-medium hover:underline">{m.username}</a
													>
												{:else}
													<span class="truncate font-medium">{m.username}</span>
												{/if}
												<span class="text-muted-foreground truncate text-xs">{m.email}</span>
											</span>
										</span>
									</Table.Cell>
									<Table.Cell class="text-muted-foreground hidden sm:table-cell">{ROLE_LABEL[m.org_role]}</Table.Cell>
									<Table.Cell class="text-muted-foreground hidden md:table-cell">{timeAgo(m.added_at)}</Table.Cell>
									<Table.Cell class="pr-5">
										<span class="flex items-center justify-end gap-2">
											{#if session.isAdmin}
												<ToggleGroup.Root
													type="single"
													size="sm"
													variant="outline"
													value={m.role}
													onValueChange={(v) => v && setRole(m, v as TeamRole)}
													disabled={saving}
													aria-label="{m.username}'s role"
												>
													{#each ['member', 'lead'] as const as r (r)}
														<ToggleGroup.Item value={r} class="px-2.5 text-xs">{TEAM_ROLE_LABEL[r]}</ToggleGroup.Item>
													{/each}
												</ToggleGroup.Root>
												<Button
													variant="ghost"
													size="icon-sm"
													onclick={() => remove(m)}
													disabled={saving}
													aria-label="Remove {m.username} from the team"
												>
													<XIcon />
												</Button>
											{:else}
												<Badge variant={m.role === 'lead' ? 'secondary' : 'outline'}>{TEAM_ROLE_LABEL[m.role]}</Badge>
											{/if}
										</span>
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
				{#if session.isAdmin}
					<p class="text-muted-foreground text-xs">
						Leads look after the team's files and can see and edit their teammates' cards and leads. Admins can always
						see everything.
					</p>
				{/if}
			{/if}
		</section>
	{/if}
</div>

{#if team}
	<TeamDialog bind:open={editOpen} {team} onsaved={(t) => (team = t)} />
{/if}

<AlertDialog.Root bind:open={deleteOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete {team?.name}?</AlertDialog.Title>
			<AlertDialog.Description>
				People stay in the organisation; they just leave this team. Its {team?.file_count === 1
					? 'file moves'
					: 'files move'} to the organisation's files, so cards using them keep working.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{#if deleting}<Spinner data-icon="inline-start" />{/if}
				Delete team
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
