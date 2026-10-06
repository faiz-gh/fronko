<script lang="ts">
	import { goto } from '$app/navigation';
	import ArrowDownWideNarrowIcon from '@lucide/svelte/icons/arrow-down-wide-narrow';
	import BuildingIcon from '@lucide/svelte/icons/building-2';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { listOrgs, type OrgSort, type OrgStatusFilter, type OrgUsage } from '$lib/api/admin';
	import { Badge } from '$lib/components/ui/badge';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Empty from '$lib/components/ui/empty';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import Pagination from '$lib/components/app/pagination.svelte';
	import StorageCell from '$lib/components/app/admin/storage-cell.svelte';
	import { formatDateTime, timeAgo } from '$lib/format';

	const SORTS: { value: OrgSort; label: string }[] = [
		{ value: 'newest', label: 'Newest' },
		{ value: 'oldest', label: 'Oldest' },
		{ value: 'name', label: 'Name' },
		{ value: 'last_active', label: 'Recently active' },
		{ value: 'users', label: 'Most users' },
		{ value: 'cards', label: 'Most cards' },
		{ value: 'leads', label: 'Most leads' },
		{ value: 'storage', label: 'Most storage' }
	];

	let query = $state('');
	let search = $state('');
	let status = $state<OrgStatusFilter>('');
	let sort = $state<OrgSort>('newest');
	let pageNum = $state(1);
	let pageSize = $state(25);

	let orgs = $state<OrgUsage[] | null>(null);
	let total = $state(0);
	let loading = $state(false);
	let error = $state('');

	// Search as you type, without a request per keystroke.
	$effect(() => {
		const q = query.trim();
		const t = setTimeout(() => {
			if (q !== search) {
				search = q;
				pageNum = 1;
			}
		}, 250);
		return () => clearTimeout(t);
	});

	let requestId = 0;
	async function load() {
		const id = ++requestId;
		loading = true;
		error = '';
		try {
			const res = await listOrgs({ q: search, status, sort, page: pageNum, pageSize });
			if (id !== requestId) return;
			orgs = res.items;
			total = res.total;
		} catch (e) {
			if (id === requestId) error = e instanceof Error ? e.message : 'Failed to load organisations';
		} finally {
			if (id === requestId) loading = false;
		}
	}

	$effect(() => {
		void [search, status, sort, pageNum, pageSize];
		load();
	});

	const filtered = $derived(search !== '' || status !== '');
	const sortLabel = $derived(SORTS.find((s) => s.value === sort)?.label);
</script>

<svelte:head>
	<title>Organisations · Fronko admin</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Organisations</h1>
		<p class="text-muted-foreground text-sm">
			Usage per organisation. Storage counts files uploaded through Fronko, not everything in the bucket.
		</p>
	</header>

	<div class="flex flex-wrap items-center gap-3">
		<div class="relative w-full max-w-sm">
			<SearchIcon class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
			<Input bind:value={query} placeholder="Search name or owner email" class="pl-9" aria-label="Search organisations" />
		</div>
		<ToggleGroup.Root
			type="single"
			variant="outline"
			value={status || 'all'}
			onValueChange={(v) => {
				if (!v) return;
				status = (v === 'all' ? '' : v) as OrgStatusFilter;
				pageNum = 1;
			}}
			aria-label="Status"
		>
			<ToggleGroup.Item value="all">All</ToggleGroup.Item>
			<ToggleGroup.Item value="active">Active</ToggleGroup.Item>
			<ToggleGroup.Item value="suspended">Suspended</ToggleGroup.Item>
		</ToggleGroup.Root>
		<DropdownMenu.Root>
			<DropdownMenu.Trigger class={buttonVariants({ variant: 'outline' })}>
				<ArrowDownWideNarrowIcon data-icon="inline-start" />
				{sortLabel}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start" class="w-48">
				<DropdownMenu.RadioGroup
					value={sort}
					onValueChange={(v) => {
						sort = v as OrgSort;
						pageNum = 1;
					}}
				>
					{#each SORTS as s (s.value)}
						<DropdownMenu.RadioItem value={s.value}>{s.label}</DropdownMenu.RadioItem>
					{/each}
				</DropdownMenu.RadioGroup>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>

	{#if error && !orgs}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load organisations</p>
			<p class="text-muted-foreground text-sm">{error}</p>
			<Button variant="outline" onclick={load}>Try again</Button>
		</div>
	{:else if orgs === null}
		<div class="bg-card flex flex-col gap-5 rounded-xl border p-5">
			{#each [1, 2, 3, 4, 5] as i (i)}
				<div class="flex items-center gap-3">
					<div class="flex flex-1 flex-col gap-1.5">
						<Skeleton class="h-3 w-40" />
						<Skeleton class="h-2.5 w-56" />
					</div>
					<Skeleton class="h-3 w-24" />
				</div>
			{/each}
		</div>
	{:else if orgs.length === 0 && !filtered}
		<Empty.Root class="bg-card rounded-xl border border-dashed py-20">
			<Empty.Header>
				<Empty.Media variant="icon"><BuildingIcon /></Empty.Media>
				<Empty.Title>No organisations yet</Empty.Title>
				<Empty.Description>Every account that registers creates one. They'll show up here.</Empty.Description>
			</Empty.Header>
		</Empty.Root>
	{:else}
		<div class="bg-card overflow-hidden rounded-xl border" aria-busy={loading}>
			<Table.Root>
				<Table.Header>
					<Table.Row class="bg-muted/40 hover:bg-muted/40">
						<Table.Head class="h-10 pl-5">Organisation</Table.Head>
						<Table.Head class="hidden h-10 text-right sm:table-cell">Users</Table.Head>
						<Table.Head class="hidden h-10 text-right sm:table-cell">Cards</Table.Head>
						<Table.Head class="hidden h-10 text-right md:table-cell">Leads</Table.Head>
						<Table.Head class="hidden h-10 lg:table-cell">Storage</Table.Head>
						<Table.Head class="hidden h-10 xl:table-cell">Created</Table.Head>
						<Table.Head class="h-10 pr-5 text-right">Last active</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body class={loading ? 'opacity-60 transition-opacity' : ''}>
					{#each orgs as org (org.id)}
						<Table.Row class="cursor-pointer" onclick={() => goto(`/admin/orgs/${org.id}`)}>
							<Table.Cell class="py-3 pl-5">
								<div class="flex min-w-0 flex-col">
									<span class="flex items-center gap-2">
										<a
											href="/admin/orgs/{org.id}"
											class="truncate font-medium hover:underline"
											onclick={(e) => e.stopPropagation()}
										>
											{org.name}
										</a>
										{#if org.suspended_at}
											<Badge variant="destructive">Suspended</Badge>
										{/if}
									</span>
									<span class="text-muted-foreground truncate text-xs">{org.owner_email ?? 'No owner email'}</span>
								</div>
							</Table.Cell>
							<Table.Cell class="tabular hidden py-3 text-right sm:table-cell">{org.user_count}</Table.Cell>
							<Table.Cell class="tabular hidden py-3 text-right sm:table-cell">{org.card_count}</Table.Cell>
							<Table.Cell class="tabular hidden py-3 text-right md:table-cell">{org.lead_count}</Table.Cell>
							<Table.Cell class="hidden py-3 text-sm lg:table-cell"><StorageCell {org} /></Table.Cell>
							<Table.Cell class="text-muted-foreground hidden py-3 text-xs whitespace-nowrap xl:table-cell">
								{formatDateTime(org.created_at)}
							</Table.Cell>
							<Table.Cell
								class="text-muted-foreground py-3 pr-5 text-right text-xs whitespace-nowrap"
								title={org.last_active_at ? formatDateTime(org.last_active_at) : undefined}
							>
								{org.last_active_at ? timeAgo(org.last_active_at) : 'Never'}
							</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row class="hover:bg-transparent">
							<Table.Cell colspan={7} class="text-muted-foreground h-32 text-center">
								No organisations match these filters.
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
		<Pagination bind:page={pageNum} bind:pageSize {total} disabled={loading} />
	{/if}
</div>
