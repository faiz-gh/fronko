<script lang="ts">
	import { goto } from '$app/navigation';
	import { page as appPage } from '$app/state';
	import { toast } from 'svelte-sonner';
	import CloudUploadIcon from '@lucide/svelte/icons/cloud-upload';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import FolderInputIcon from '@lucide/svelte/icons/folder-input';
	import Grid2x2Icon from '@lucide/svelte/icons/grid-2x2';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import LayersIcon from '@lucide/svelte/icons/layers';
	import ListIcon from '@lucide/svelte/icons/list';
	import SearchIcon from '@lucide/svelte/icons/search';
	import ShareIcon from '@lucide/svelte/icons/share-2';
	import TagIcon from '@lucide/svelte/icons/tag';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import UserIcon from '@lucide/svelte/icons/user';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import XIcon from '@lucide/svelte/icons/x';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import {
		bulkFiles,
		fileCounts,
		listFiles,
		PURPOSE_ORDER,
		PURPOSES,
		purposesFor,
		SORT_LABEL,
		type FilePurpose,
		type FileSort,
		type LibraryFile
	} from '$lib/api/files';
	import { teamColor, type TeamRef } from '$lib/api/teams';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Empty from '$lib/components/ui/empty';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import FileAccessDialog from '$lib/components/app/file-access-dialog.svelte';
	import FileDropzone from '$lib/components/app/file-dropzone.svelte';
	import FileDetailSheet from '$lib/components/app/files/file-detail-sheet.svelte';
	import FileGrid from '$lib/components/app/files/file-grid.svelte';
	import FileTable from '$lib/components/app/files/file-table.svelte';
	import PurposeChips from '$lib/components/app/files/purpose-chips.svelte';
	import PurposeIcon from '$lib/components/app/files/purpose-icon.svelte';
	import Pagination from '$lib/components/app/pagination.svelte';
	import StorageMeter from '$lib/components/app/storage-meter.svelte';
	import UserPicker from '$lib/components/app/user-picker.svelte';
	import { fileLocations, moveTargets, type FileLocation } from '$lib/file-locations';
	import { plural } from '$lib/format';
	import { session } from '$lib/session.svelte';
	import { storage } from '$lib/storage.svelte';
	import { teams } from '$lib/teams.svelte';
	import { cn } from '$lib/utils';

	// ---- Where we are: location, search, purpose and sort live in the URL ----

	const who = $derived({
		isAdmin: session.isAdmin,
		orgName: session.orgName,
		// Admins see every team; everyone else the teams they're in.
		teams: session.isAdmin
			? (teams.list ?? []).map<TeamRef>((t) => ({ id: t.id, name: t.name, color: t.color }))
			: session.teams,
		ledTeamIds: session.ledTeams.map((t) => t.id)
	});
	const locations = $derived(fileLocations(who));
	const targets = $derived(moveTargets(who));

	const params = appPage.url.searchParams;
	let locKey = $state(params.get('loc') ?? params.get('area') ?? 'all');
	let search = $state(params.get('q') ?? '');
	let query = $state(params.get('q') ?? '');
	let purpose = $state<FilePurpose | null>((params.get('purpose') as FilePurpose | null) ?? null);
	let sort = $state<FileSort>((params.get('sort') as FileSort | null) ?? 'newest');
	let owner = $state<number | null>(null);

	const location = $derived<FileLocation>(locations.find((l) => l.key === locKey) ?? locations[0]);

	// Debounce typing into the search box.
	$effect(() => {
		const value = search;
		const t = setTimeout(() => (query = value.trim()), 250);
		return () => clearTimeout(t);
	});

	// Mirror the view into the URL so it survives reloads and can be linked.
	$effect(() => {
		const url = new URL(appPage.url.href);
		const set = (k: string, v: string | null) => (v ? url.searchParams.set(k, v) : url.searchParams.delete(k));
		set('loc', location.key === 'all' ? null : location.key);
		set('q', query || null);
		set('purpose', purpose);
		set('sort', sort === 'newest' ? null : sort);
		url.searchParams.delete('area');
		if (url.search !== appPage.url.search) goto(url, { replace: true, shallow: true });
	});

	// ---- Grid or list, remembered per browser ----

	let view = $state<'grid' | 'list'>('grid');
	try {
		if (localStorage.getItem('fronko.files.view') === 'list') view = 'list';
	} catch {
		// Storage unavailable: default to the grid.
	}
	function setView(v: 'grid' | 'list') {
		view = v;
		try {
			localStorage.setItem('fronko.files.view', v);
		} catch {
			// Not remembered; fine.
		}
	}

	// ---- Loading ----

	let page = $state(1);
	let pageSize = $state(24);
	let files = $state<LibraryFile[] | null>(null);
	let total = $state(0);
	let counts = $state<Partial<Record<FilePurpose, number>> | undefined>(undefined);
	let countTotal = $state<number | undefined>(undefined);
	let loading = $state(false);
	let error = $state('');
	let reload = $state(0);

	const baseQuery = $derived({
		...location.query,
		userId: session.isAdmin && location.query.area === 'personal' && owner !== null ? owner : undefined,
		q: query || undefined
	});

	let lastFilter = '';
	let requestId = 0;
	$effect(() => {
		void reload;
		if (!storage.ready) return;
		const filter = JSON.stringify([baseQuery, purpose, sort]);
		if (filter !== lastFilter) {
			const changed = lastFilter !== '';
			lastFilter = filter;
			selected = new Set();
			if (changed) files = null;
			if (changed && page !== 1) {
				page = 1;
				return;
			}
		}
		const id = ++requestId;
		loading = true;
		error = '';
		Promise.all([
			listFiles({ ...baseQuery, purposes: purpose ? [purpose] : undefined, sort, page, pageSize }),
			fileCounts(baseQuery)
		])
			.then(([res, c]) => {
				if (id !== requestId) return;
				if (res.files.length === 0 && res.total > 0 && page > 1) {
					page = Math.ceil(res.total / pageSize);
					return;
				}
				files = res.files;
				total = res.total;
				counts = c.counts;
				countTotal = c.total;
			})
			.catch((e) => {
				if (id === requestId) error = e instanceof Error ? e.message : 'Failed to load files';
			})
			.finally(() => {
				if (id === requestId) loading = false;
			});
	});

	function refresh() {
		reload++;
		if (session.username) storage.load(session.username, true);
	}

	// ---- Permissions ----

	function canEdit(f: LibraryFile) {
		if (session.isAdmin) return true;
		if (f.area === 'personal') return f.owner?.username === session.username;
		return f.area === 'team' && !!f.team && who.ledTeamIds.includes(f.team.id);
	}

	// ---- Selection ----

	let selected = $state<Set<string>>(new Set());
	let anchor: string | null = null;
	const selectedFiles = $derived((files ?? []).filter((f) => selected.has(f.id)));
	const selectable = $derived((files ?? []).some(canEdit));

	function toggle(file: LibraryFile, range: boolean) {
		const list = files ?? [];
		const next = new Set(selected);
		if (range && anchor) {
			const a = list.findIndex((f) => f.id === anchor);
			const b = list.findIndex((f) => f.id === file.id);
			if (a !== -1 && b !== -1) {
				for (const f of list.slice(Math.min(a, b), Math.max(a, b) + 1)) if (canEdit(f)) next.add(f.id);
				selected = next;
				return;
			}
		}
		if (next.has(file.id)) next.delete(file.id);
		else if (canEdit(file)) next.add(file.id);
		else {
			toast.info('You can’t change this file');
			return;
		}
		anchor = file.id;
		selected = next;
	}

	function toggleAll(on: boolean) {
		selected = on ? new Set((files ?? []).filter(canEdit).map((f) => f.id)) : new Set();
	}

	// Purposes every selected file can take (images and PDFs differ).
	const bulkPurposes = $derived(
		PURPOSE_ORDER.filter((p) => selectedFiles.every((f) => purposesFor(f.kind).includes(p)))
	);

	let bulkBusy = $state(false);

	async function bulkUpdate(patch: Parameters<typeof bulkFiles>[2], done: string) {
		bulkBusy = true;
		try {
			const res = await bulkFiles([...selected], 'update', patch);
			if (res.done.length) toast.success(`${done}: ${plural(res.done.length, 'file')}`);
			if (res.failed.length) toast.error(`${plural(res.failed.length, 'file')} couldn’t be changed: ${res.failed[0].error}`);
			selected = new Set();
			refresh();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Something went wrong');
		} finally {
			bulkBusy = false;
		}
	}

	// ---- Detail, access, delete ----

	let detail = $state<LibraryFile | null>(null);
	let accessTarget = $state<LibraryFile | null>(null);
	let deleteTargets = $state<LibraryFile[] | null>(null);
	let deleting = $state(false);
	const deleteUses = $derived((deleteTargets ?? []).reduce((n, f) => n + f.use_count, 0));

	function replaceFile(updated: LibraryFile) {
		if (files) files = files.map((f) => (f.id === updated.id ? updated : f));
		// It may no longer belong in this view (moved, re-purposed); recount.
		reload++;
	}

	async function confirmDelete() {
		if (!deleteTargets) return;
		deleting = true;
		try {
			const ids = deleteTargets.map((f) => f.id);
			const res = await bulkFiles(ids, 'delete');
			if (res.done.length) toast.success(res.done.length === 1 ? 'File deleted' : `${res.done.length} files deleted`);
			if (res.failed.length) toast.error(`${plural(res.failed.length, 'file')} couldn’t be deleted: ${res.failed[0].error}`);
			if (detail && res.done.includes(detail.id)) detail = null;
			deleteTargets = null;
			selected = new Set();
			refresh();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Delete failed');
		} finally {
			deleting = false;
		}
	}

	// ---- Uploading: the button, the dropzone, or dropping anywhere on the page ----

	let dropzone = $state<ReturnType<typeof FileDropzone> | null>(null);
	let dragDepth = $state(0);
	const canUpload = $derived(!!location.upload && storage.ready);

	function hasFiles(e: DragEvent) {
		return !!e.dataTransfer && Array.from(e.dataTransfer.types).includes('Files');
	}

	// Empty-state copy for the current filters.
	const emptyText = $derived.by(() => {
		if (query) return { title: 'No matches', body: `Nothing called “${query}” here. Try another word or clear the search.` };
		if (purpose)
			return {
				title: `No ${PURPOSES[purpose].plural.toLowerCase()} here yet`,
				body: canUpload ? `${PURPOSES[purpose].hint}. Drop one here to add it.` : PURPOSES[purpose].hint + '.'
			};
		if (location.key === 'granted') return { title: 'Nothing shared with you yet', body: 'Files your organisation gives you or your teams show up here.' };
		if (canUpload) return { title: 'No files yet', body: 'Drop logos, photos, banners or PDF brochures here, or use Upload.' };
		return { title: 'No files here yet', body: location.description };
	});
</script>

<svelte:head>
	<title>Files · Fronko</title>
</svelte:head>

<svelte:window
	ondragenter={(e) => {
		if (!canUpload || !hasFiles(e)) return;
		e.preventDefault();
		dragDepth++;
	}}
	ondragover={(e) => {
		if (canUpload && hasFiles(e)) e.preventDefault();
	}}
	ondragleave={() => (dragDepth = Math.max(0, dragDepth - 1))}
	ondrop={(e) => {
		if (!canUpload || !hasFiles(e)) return;
		e.preventDefault();
		dragDepth = 0;
		if (e.dataTransfer?.files.length) dropzone?.upload(e.dataTransfer.files);
	}}
/>

{#if dragDepth > 0}
	<div class="bg-background/80 pointer-events-none fixed inset-0 z-40 grid place-items-center p-6 backdrop-blur-sm lg:pl-68">
		<div class="border-brand bg-brand-soft flex w-full max-w-lg flex-col items-center gap-3 rounded-2xl border-2 border-dashed px-8 py-14 text-center">
			<CloudUploadIcon class="text-brand size-10" />
			<p class="text-lg font-semibold">Drop to upload to {location.label}</p>
			<p class="text-muted-foreground text-sm">
				{purpose ? `They’ll be filed as ${PURPOSES[purpose].plural.toLowerCase()} where that fits.` : 'Images up to 5 MB, PDFs up to 20 MB.'}
			</p>
		</div>
	</div>
{/if}

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-wrap items-end justify-between gap-4">
		<div class="flex flex-col gap-1">
			<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Files</h1>
			<p class="text-muted-foreground text-sm">
				Logos, banners, photos and brochures in {session.orgName}'s storage. Upload once, use anywhere.
			</p>
		</div>
		{#if canUpload}
			<Button onclick={() => dropzone?.browse()}>
				<CloudUploadIcon data-icon="inline-start" />
				Upload
			</Button>
		{/if}
	</header>

	{#if !storage.status}
		<Skeleton class="h-40 rounded-xl" />
	{:else if !storage.ready}
		<Empty.Root class="bg-card rounded-xl border border-dashed py-16">
			<Empty.Header>
				<Empty.Media variant="icon"><FolderIcon /></Empty.Media>
				<Empty.Title>
					{#if !storage.status.enabled}
						File storage isn't enabled on this server
					{:else if session.isOwner}
						Connect your storage to upload files
					{:else}
						{session.orgName} hasn't connected storage yet
					{/if}
				</Empty.Title>
				<Empty.Description>
					{#if !storage.status.enabled}
						Ask whoever runs this server to set SECRETS_KEY.
					{:else if session.isOwner}
						Files are stored in your own S3-compatible bucket (Cloudflare R2, Backblaze B2, AWS S3 or MinIO).
						Everyone in your organisation uploads to it.
					{:else}
						Ask the owner to connect a bucket in Settings. Until then, nobody can upload files.
					{/if}
				</Empty.Description>
			</Empty.Header>
			{#if storage.status.enabled && session.isOwner}
				<Button href="/dashboard/settings?tab=storage">Connect storage</Button>
			{/if}
		</Empty.Root>
	{:else}
		<div class="grid gap-6 lg:grid-cols-[220px_minmax(0,1fr)] lg:gap-8">
			<!-- Locations: a rail on desktop, a select on small screens. -->
			<nav class="hidden flex-col gap-4 lg:flex" aria-label="File locations">
				<ul class="flex flex-col gap-0.5">
					{#each locations as loc (loc.key)}
						{@const active = loc.key === location.key}
						{#if loc.team && !locations[locations.indexOf(loc) - 1]?.team}
							<li class="text-muted-foreground mt-3 mb-1 px-3 text-[11px] font-semibold tracking-wider uppercase">Teams</li>
						{/if}
						<li>
							<button
								type="button"
								aria-current={active ? 'page' : undefined}
								onclick={() => (locKey = loc.key)}
								class={cn(
									'flex h-9 w-full items-center gap-2.5 rounded-lg px-3 text-left text-sm transition-colors',
									'focus-visible:ring-ring/50 outline-none focus-visible:ring-3',
									active ? 'bg-muted text-foreground font-medium' : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
								)}
							>
								{#if loc.team}
									<span class="size-2.5 shrink-0 rounded-full" style="background: {teamColor(loc.team)}"></span>
								{:else if loc.key === 'all'}
									<LayersIcon class="size-4 shrink-0" />
								{:else if loc.key === 'personal'}
									<UserIcon class="size-4 shrink-0" />
								{:else if loc.key === 'shared'}
									<UsersRoundIcon class="size-4 shrink-0" />
								{:else if loc.key === 'granted'}
									<ShareIcon class="size-4 shrink-0" />
								{:else}
									<FolderIcon class="size-4 shrink-0" />
								{/if}
								<span class="truncate">{loc.label}</span>
							</button>
						</li>
					{/each}
				</ul>
				{#if !session.isAdmin && storage.status}
					<StorageMeter used={storage.status.used_bytes} quota={storage.status.quota_bytes} class="px-1" />
				{/if}
			</nav>

			<div class="flex min-w-0 flex-col gap-5">
				<div class="flex flex-col gap-3">
					<div class="lg:hidden">
						<Select.Root type="single" value={location.key} onValueChange={(v) => (locKey = v)}>
							<Select.Trigger class="w-full" aria-label="Location">
								<span class="flex items-center gap-2">
									{#if location.team}<span class="size-2.5 rounded-full" style="background: {teamColor(location.team)}"></span>{/if}
									{location.label}
								</span>
							</Select.Trigger>
							<Select.Content>
								{#each locations as loc (loc.key)}
									<Select.Item value={loc.key} label={loc.label}>
										<span class="flex items-center gap-2">
											{#if loc.team}<span class="size-2.5 rounded-full" style="background: {teamColor(loc.team)}"></span>{/if}
											{loc.label}
										</span>
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
					<div class="flex flex-col gap-0.5">
						<h2 class="hidden text-lg font-semibold lg:block">{location.label}</h2>
						<p class="text-muted-foreground text-sm">{location.description}</p>
					</div>
					{#if !session.isAdmin && location.key === 'personal' && storage.status}
						<StorageMeter used={storage.status.used_bytes} quota={storage.status.quota_bytes} class="max-w-sm lg:hidden" />
					{/if}
				</div>

				<!-- Toolbar -->
				<div class="flex flex-wrap items-center gap-2">
					<div class="relative min-w-48 flex-1">
						<SearchIcon class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
						<Input bind:value={search} placeholder="Search by name" class="pr-9 pl-9" aria-label="Search files" />
						{#if search}
							<button
								type="button"
								class="text-muted-foreground hover:text-foreground absolute top-1/2 right-2.5 -translate-y-1/2"
								onclick={() => (search = '')}
								aria-label="Clear search"
							>
								<XIcon class="size-4" />
							</button>
						{/if}
					</div>
					{#if session.isAdmin && location.query.area === 'personal'}
						<UserPicker
							value={owner}
							onchange={(v) => (owner = typeof v === 'number' ? v : null)}
							allLabel="Everyone"
							allowNone={false}
						/>
					{/if}
					<DropdownMenu.Root>
						<DropdownMenu.Trigger class={buttonVariants({ variant: 'outline' })}>
							<ArrowUpDownIcon data-icon="inline-start" />
							<span class="hidden sm:inline">{SORT_LABEL[sort]}</span>
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end" class="w-44">
							<DropdownMenu.RadioGroup value={sort} onValueChange={(v) => (sort = v as FileSort)}>
								{#each Object.entries(SORT_LABEL) as [value, label] (value)}
									<DropdownMenu.RadioItem {value}>{label}</DropdownMenu.RadioItem>
								{/each}
							</DropdownMenu.RadioGroup>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
					<div class="bg-muted inline-flex rounded-lg p-[3px]" role="radiogroup" aria-label="View">
						{#each [{ v: 'grid', Icon: Grid2x2Icon, label: 'Grid' }, { v: 'list', Icon: ListIcon, label: 'List' }] as const as o (o.v)}
							<button
								type="button"
								role="radio"
								aria-checked={view === o.v}
								aria-label={o.label}
								onclick={() => setView(o.v)}
								class={cn(
									'grid size-8 place-items-center rounded-md transition-colors',
									view === o.v ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
								)}
							>
								<o.Icon class="size-4" />
							</button>
						{/each}
					</div>
				</div>

				<PurposeChips bind:value={purpose} purposes={PURPOSE_ORDER} {counts} total={countTotal} />

				{#if canUpload && location.upload}
					<FileDropzone
						bind:this={dropzone}
						compact
						area={location.upload.area}
						teamId={location.upload.teamId}
						{purpose}
						onuploaded={refresh}
					/>
				{/if}

				<!-- Bulk actions for the selection -->
				{#if selected.size > 0}
					<div
						class="bg-foreground text-background sticky top-16 z-20 flex flex-wrap items-center gap-2 rounded-xl px-3 py-2 shadow-lg lg:top-4"
						role="toolbar"
						aria-label="Selected files"
					>
						<button
							type="button"
							class="hover:bg-background/15 grid size-8 place-items-center rounded-md"
							onclick={() => (selected = new Set())}
							aria-label="Clear selection"
						>
							<XIcon class="size-4" />
						</button>
						<span class="mr-auto text-sm font-medium">{selected.size} selected</span>
						{#if bulkBusy}<Spinner class="size-4" />{/if}
						<DropdownMenu.Root>
							<DropdownMenu.Trigger
								disabled={bulkBusy || bulkPurposes.length === 0}
								class={buttonVariants({ variant: 'ghost', size: 'sm', class: 'hover:bg-background/15 hover:text-background' })}
							>
								<TagIcon data-icon="inline-start" />
								Purpose
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="end" class="w-48">
								{#each bulkPurposes as p (p)}
									<DropdownMenu.Item onSelect={() => bulkUpdate({ purpose: p }, `Marked as ${PURPOSES[p].label.toLowerCase()}`)}>
										<PurposeIcon purpose={p} class="size-4" />
										{PURPOSES[p].label}
									</DropdownMenu.Item>
								{/each}
							</DropdownMenu.Content>
						</DropdownMenu.Root>
						{#if targets.length > 0}
							<DropdownMenu.Root>
								<DropdownMenu.Trigger
									disabled={bulkBusy}
									class={buttonVariants({ variant: 'ghost', size: 'sm', class: 'hover:bg-background/15 hover:text-background' })}
								>
									<FolderInputIcon data-icon="inline-start" />
									Move
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="end" class="w-52">
									{#each targets as t, i (t.key)}
										{#if t.team && !targets[i - 1]?.team}
											<DropdownMenu.Separator />
											<DropdownMenu.Label>Teams</DropdownMenu.Label>
										{/if}
										<DropdownMenu.Item onSelect={() => bulkUpdate({ area: t.area, team_id: t.teamId }, `Moved to ${t.label}`)}>
											{#if t.team}
												<span class="size-2.5 rounded-full" style="background: {teamColor(t.team)}"></span>
											{:else}
												<FolderIcon class="size-4" />
											{/if}
											{t.label}
										</DropdownMenu.Item>
									{/each}
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						{/if}
						<Button
							size="sm"
							variant="ghost"
							class="hover:bg-background/15 text-red-300 hover:text-red-200"
							disabled={bulkBusy}
							onclick={() => (deleteTargets = selectedFiles)}
						>
							<Trash2Icon data-icon="inline-start" />
							Delete
						</Button>
					</div>
				{/if}

				{#if error && !files}
					<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
						<p class="font-medium">Couldn't load your files</p>
						<p class="text-muted-foreground text-sm">{error}</p>
						<Button variant="outline" onclick={() => reload++}>Try again</Button>
					</div>
				{:else if files === null}
					<div class="grid grid-cols-2 gap-4 sm:grid-cols-[repeat(auto-fill,minmax(190px,1fr))]">
						{#each [1, 2, 3, 4, 5, 6] as i (i)}
							<Skeleton class="aspect-[4/4.2] rounded-xl" />
						{/each}
					</div>
				{:else if files.length === 0}
					<Empty.Root class="bg-card rounded-xl border border-dashed py-14">
						<Empty.Header>
							<Empty.Media variant="icon">
								{#if purpose}<PurposeIcon {purpose} />{:else}<InboxIcon />{/if}
							</Empty.Media>
							<Empty.Title>{emptyText.title}</Empty.Title>
							<Empty.Description>{emptyText.body}</Empty.Description>
						</Empty.Header>
						{#if query || purpose}
							<Button
								variant="outline"
								onclick={() => {
									search = '';
									query = '';
									purpose = null;
								}}>Clear filters</Button
							>
						{/if}
					</Empty.Root>
				{:else}
					<div class={cn('transition-opacity', loading && 'opacity-60')} aria-busy={loading}>
						{#if view === 'grid'}
							<FileGrid
								{files}
								{selected}
								{selectable}
								showLocation={location.key === 'all' || location.key === 'granted'}
								onopen={(f) => (detail = f)}
								ontoggle={toggle}
							/>
						{:else}
							<FileTable {files} {selected} {selectable} onopen={(f) => (detail = f)} ontoggle={toggle} ontoggleall={toggleAll} />
						{/if}
					</div>
					{#if total > 12}
						<Pagination bind:page bind:pageSize {total} pageSizes={[12, 24, 48, 96]} />
					{/if}
				{/if}
			</div>
		</div>
	{/if}
</div>

<FileDetailSheet
	bind:file={detail}
	editable={canEdit}
	{targets}
	onchanged={replaceFile}
	ondelete={(f) => (deleteTargets = [f])}
	onaccess={session.isAdmin ? (f) => (accessTarget = f) : undefined}
	oncopied={refresh}
/>

<FileAccessDialog bind:file={accessTarget} />

<AlertDialog.Root open={deleteTargets !== null} onOpenChange={(open) => !open && (deleteTargets = null)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>
				{deleteTargets?.length === 1 ? 'Delete this file?' : `Delete ${deleteTargets?.length} files?`}
			</AlertDialog.Title>
			<AlertDialog.Description>
				{#if deleteUses > 0}
					{deleteTargets?.length === 1 ? 'It’s' : 'They’re'} used in {plural(deleteUses, 'place')}: cards, the
					organisation logo or the signature banner will stop showing {deleteTargets?.length === 1 ? 'it' : 'them'}.
				{:else}
					{deleteTargets?.length === 1 ? 'It isn’t' : 'They aren’t'} used anywhere.
				{/if}
				This removes {deleteTargets?.length === 1 ? 'it' : 'them'} from storage and can’t be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{#if deleting}<Spinner data-icon="inline-start" />{/if}
				Delete
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
