<script lang="ts">
	import { goto } from '$app/navigation';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import StarIcon from '@lucide/svelte/icons/star';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import { teamColor } from '$lib/features/teams/api';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import LoadError from '$lib/components/shared/load-error.svelte';
	import PageHeader from '$lib/components/shared/page-header.svelte';
	import * as Empty from '$lib/components/ui/empty';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import TeamDialog from '$lib/features/teams/components/team-dialog.svelte';
	import { plural } from '$lib/core/format';
	import { session } from '$lib/core/session.svelte';
	import { teams } from '$lib/features/teams/store.svelte';

	// Admins manage teams; everyone else sees the teams they're in.
	$effect(() => {
		if (session.ready && !session.isAdmin && session.teams.length === 0) goto('/dashboard', { replaceState: true });
	});

	let createOpen = $state(false);
	const myRole = (id: number) => session.teams.find((t) => t.id === id)?.role;
</script>

<svelte:head>
	<title>Teams · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<PageHeader title="Teams">
		{#snippet description()}
			{#if session.isAdmin}
				Group people like Sales or Finance. Each team has its own files, and its leads look after them and see their
				teammates’ cards and leads.
			{:else}
				The teams you're in at {session.orgName}.
			{/if}
		{/snippet}
		{#snippet actions()}
			{#if session.isAdmin}
				<Button onclick={() => (createOpen = true)}>
					<PlusIcon data-icon="inline-start" />
					New team
				</Button>
			{/if}
		{/snippet}
	</PageHeader>

	{#if teams.error && !teams.list}
		<LoadError what="teams" message={teams.error} onretry={() => teams.refresh()} />
	{:else if teams.list === null}
		<div class="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,280px),1fr))]">
			{#each [1, 2, 3] as i (i)}
				<Skeleton class="h-40 rounded-xl" />
			{/each}
		</div>
	{:else if teams.list.length === 0}
		<Empty.Root class="bg-card rounded-xl border border-dashed py-20">
			<Empty.Header>
				<Empty.Media variant="icon"><UsersRoundIcon /></Empty.Media>
				<Empty.Title>No teams yet</Empty.Title>
				<Empty.Description>
					Create a team for each department, add people to it, and pick its leads. Team files are shared with everyone
					in the team.
				</Empty.Description>
			</Empty.Header>
			{#if session.isAdmin}
				<Empty.Content>
					<Button onclick={() => (createOpen = true)}>
						<PlusIcon data-icon="inline-start" />
						New team
					</Button>
				</Empty.Content>
			{/if}
		</Empty.Root>
	{:else}
		<ul class="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,280px),1fr))]">
			{#each teams.list as team (team.id)}
				{@const role = myRole(team.id)}
				<li
					class="bg-card group relative flex flex-col overflow-hidden rounded-xl border transition-shadow hover:shadow-md"
				>
					<span class="h-1.5" style="background: {teamColor(team)}"></span>
					<div class="flex flex-1 flex-col gap-3 p-5">
						<div class="flex items-start justify-between gap-3">
							<h2 class="min-w-0 truncate text-base font-semibold">
								<a href="/dashboard/teams/{team.id}" class="after:absolute after:inset-0">{team.name}</a>
							</h2>
							{#if role === 'lead'}
								<Badge variant="secondary" class="shrink-0"><StarIcon />You lead</Badge>
							{:else if role}
								<Badge variant="outline" class="shrink-0">Member</Badge>
							{/if}
						</div>
						<p class="text-muted-foreground line-clamp-2 min-h-10 text-sm">
							{team.description || 'No description.'}
						</p>
						<div class="text-muted-foreground mt-auto flex flex-wrap gap-x-4 gap-y-1 border-t pt-3 text-xs">
							<span class="inline-flex items-center gap-1.5">
								<UsersRoundIcon class="size-3.5" />
								{plural(team.member_count, 'person', 'people')}
							</span>
							<span class="inline-flex items-center gap-1.5">
								<StarIcon class="size-3.5" />
								{plural(team.lead_count, 'lead')}
							</span>
							<span class="inline-flex items-center gap-1.5">
								<FolderIcon class="size-3.5" />
								{plural(team.file_count, 'file')}
							</span>
						</div>
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<TeamDialog bind:open={createOpen} onsaved={(t) => goto(`/dashboard/teams/${t.id}`)} />
