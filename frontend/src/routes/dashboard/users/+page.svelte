<script lang="ts">
	import { goto } from '$app/navigation';
	import SearchIcon from '@lucide/svelte/icons/search';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import UsersIcon from '@lucide/svelte/icons/users';
	import { teamColor } from '$lib/api/teams';
	import { ROLE_LABEL, STATUS_LABEL, userStatus, type OrgUser, type UserStatus } from '$lib/api/org';
	import { Badge, type BadgeVariant } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Empty from '$lib/components/ui/empty';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import CreateUserDialog from '$lib/components/app/create-user-dialog.svelte';
	import StorageMeter from '$lib/components/app/storage-meter.svelte';
	import TeamPicker from '$lib/components/app/team-picker.svelte';
	import UserAvatar from '$lib/components/app/user-avatar.svelte';
	import { formatDateTime, plural, timeAgo } from '$lib/format';
	import { orgUsers } from '$lib/org-users.svelte';
	import { session } from '$lib/session.svelte';
	import { teams } from '$lib/teams.svelte';

	let createOpen = $state(false);
	let query = $state('');
	let team = $state<number | null>(null);

	// Admins only: members are sent back to the overview.
	$effect(() => {
		if (session.ready && !session.isAdmin) goto('/dashboard', { replaceState: true });
	});

	// Totals (cards, leads, storage) change elsewhere, so refresh on each visit.
	$effect(() => {
		orgUsers.refresh();
	});

	const people = $derived(orgUsers.assignable);
	const visible = $derived.by(() => {
		const q = query.trim().toLowerCase();
		return people.filter(
			(u) =>
				(team === null || u.teams.some((t) => t.id === team)) &&
				(!q || u.username.toLowerCase().includes(q) || (u.email ?? '').toLowerCase().includes(q))
		);
	});
	const pending = $derived(people.filter((u) => ['unverified', 'temporary_password'].includes(userStatus(u))).length);

	const STATUS_VARIANT: Record<UserStatus, BadgeVariant> = {
		suspended: 'destructive',
		unverified: 'outline',
		temporary_password: 'outline',
		active: 'secondary'
	};

	function open(user: OrgUser) {
		goto(`/dashboard/users/${user.id}`);
	}
</script>

<svelte:head>
	<title>Users · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-wrap items-end justify-between gap-4">
		<div class="flex flex-col gap-1">
			<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Users</h1>
			<p class="text-muted-foreground text-sm">
				{#if orgUsers.list}
					{plural(people.length, 'person', 'people')} in {session.orgName}{pending > 0
						? `, ${pending} still setting up their account`
						: ''}.
				{:else}
					People in {session.orgName}.
				{/if}
			</p>
		</div>
		<Button onclick={() => (createOpen = true)}>
			<UserPlusIcon data-icon="inline-start" />
			New user
		</Button>
	</header>

	{#if orgUsers.error && !orgUsers.list}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load users</p>
			<p class="text-muted-foreground text-sm">{orgUsers.error}</p>
			<Button variant="outline" onclick={() => orgUsers.refresh()}>Try again</Button>
		</div>
	{:else if orgUsers.list === null}
		<div class="bg-card flex flex-col gap-5 rounded-xl border p-5">
			{#each [1, 2, 3, 4] as i (i)}
				<div class="flex items-center gap-3">
					<Skeleton class="size-8 rounded-full" />
					<div class="flex flex-1 flex-col gap-1.5">
						<Skeleton class="h-3 w-32" />
						<Skeleton class="h-2.5 w-48" />
					</div>
					<Skeleton class="h-3 w-20" />
				</div>
			{/each}
		</div>
	{:else if people.length === 0}
		<Empty.Root class="bg-card rounded-xl border border-dashed py-20">
			<Empty.Header>
				<Empty.Media variant="icon">
					<UsersIcon />
				</Empty.Media>
				<Empty.Title>Add your team</Empty.Title>
				<Empty.Description>
					Create an account for each person, then assign them cards. They'll see only their own cards, leads and files,
					plus anything you share.
				</Empty.Description>
			</Empty.Header>
			<Empty.Content>
				<Button onclick={() => (createOpen = true)}>
					<UserPlusIcon data-icon="inline-start" />
					New user
				</Button>
			</Empty.Content>
		</Empty.Root>
	{:else}
		<div class="flex flex-wrap items-center gap-2">
			{#if teams.list?.length}
				<TeamPicker value={team} options={teams.list} onchange={(v) => (team = v)} />
			{/if}
			<div class="relative min-w-0 basis-full sm:max-w-sm sm:flex-1 sm:basis-auto">
				<SearchIcon class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
				<Input bind:value={query} placeholder="Search username or email" class="pl-9" aria-label="Search users" />
			</div>
		</div>

		<div class="bg-card overflow-hidden rounded-xl border">
			<Table.Root>
				<Table.Header>
					<Table.Row class="bg-muted/40 hover:bg-muted/40">
						<Table.Head class="h-10 pl-5">User</Table.Head>
						<Table.Head class="hidden h-10 xl:table-cell">Teams</Table.Head>
						<Table.Head class="hidden h-10 md:table-cell">Status</Table.Head>
						<Table.Head class="hidden h-10 text-right sm:table-cell">Cards</Table.Head>
						<Table.Head class="hidden h-10 text-right sm:table-cell">Leads</Table.Head>
						<Table.Head class="hidden h-10 w-48 lg:table-cell">Storage</Table.Head>
						<Table.Head class="h-10 pr-5 text-right">Last sign-in</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each visible as user (user.id)}
						{@const status = userStatus(user)}
						<Table.Row class="cursor-pointer" onclick={() => open(user)}>
							<Table.Cell class="py-3 pl-5">
								<div class="flex items-center gap-3">
									<UserAvatar username={user.username} class="size-8 text-xs" />
									<div class="flex min-w-0 flex-col">
										<span class="flex items-center gap-2">
											<a
												href="/dashboard/users/{user.id}"
												class="truncate font-medium hover:underline"
												onclick={(e) => e.stopPropagation()}
											>
												{user.username}
											</a>
											{#if user.role === 'admin'}
												<Badge variant="secondary">{ROLE_LABEL.admin}</Badge>
											{/if}
										</span>
										<span class="text-muted-foreground truncate text-xs">{user.email}</span>
										{#if status !== 'active'}
											<Badge variant={STATUS_VARIANT[status]} class="mt-1 md:hidden">{STATUS_LABEL[status]}</Badge>
										{/if}
									</div>
								</div>
							</Table.Cell>
							<Table.Cell class="hidden max-w-64 py-3 xl:table-cell">
								{#if user.teams.length}
									<span class="flex flex-wrap gap-1">
										{#each user.teams as t (t.id)}
											<span
												class="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs"
												title={t.role === 'lead' ? `Leads ${t.name}` : t.name}
											>
												<span class="size-1.5 rounded-full" style="background: {teamColor(t)}"></span>
												{t.name}{t.role === 'lead' ? ' · Lead' : ''}
											</span>
										{/each}
									</span>
								{:else}
									<span class="text-muted-foreground text-xs">—</span>
								{/if}
							</Table.Cell>
							<Table.Cell class="hidden py-3 md:table-cell">
								<Badge variant={STATUS_VARIANT[status]}>{STATUS_LABEL[status]}</Badge>
							</Table.Cell>
							<Table.Cell class="tabular hidden py-3 text-right sm:table-cell">{user.card_count}</Table.Cell>
							<Table.Cell class="tabular hidden py-3 text-right sm:table-cell">{user.lead_count}</Table.Cell>
							<Table.Cell class="hidden py-3 lg:table-cell">
								<StorageMeter used={user.used_bytes} quota={user.storage_quota_bytes} compact />
							</Table.Cell>
							<Table.Cell
								class="text-muted-foreground py-3 pr-5 text-right text-xs whitespace-nowrap"
								title={user.last_login_at ? formatDateTime(user.last_login_at) : undefined}
							>
								{user.last_login_at ? timeAgo(user.last_login_at) : 'Never'}
							</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row class="hover:bg-transparent">
							<Table.Cell colspan={7} class="text-muted-foreground h-32 text-center">
								No users match this filter.
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
	{/if}
</div>

<CreateUserDialog bind:open={createOpen} />
