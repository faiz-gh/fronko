<script lang="ts">
	import { page as appPage } from '$app/state';
	import { toast } from 'svelte-sonner';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import UserRoundCheckIcon from '@lucide/svelte/icons/user-round-check';
	import {
		deleteFile,
		fileUrl,
		formatBytes,
		listFiles,
		renameFile,
		type FileArea,
		type FileKind,
		type LibraryFile
	} from '$lib/api/files';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import FileAccessDialog from '$lib/components/app/file-access-dialog.svelte';
	import FileDropzone from '$lib/components/app/file-dropzone.svelte';
	import FileThumb from '$lib/components/app/file-thumb.svelte';
	import Pagination from '$lib/components/app/pagination.svelte';
	import StorageMeter from '$lib/components/app/storage-meter.svelte';
	import UserPicker from '$lib/components/app/user-picker.svelte';
	import { normalizeCard } from '$lib/card/card';
	import { cards } from '$lib/cards.svelte';
	import { plural, timeAgo } from '$lib/format';
	import { session } from '$lib/session.svelte';
	import { storage } from '$lib/storage.svelte';
	import { cn } from '$lib/utils';

	type Area = FileArea | 'granted';
	type AreaTab = { value: Area; label: string; description: string };

	// Admins look after the organisation's files and can see everyone's; members
	// have their own files, the shared area, and whatever they've been given.
	const areaTabs = $derived<AreaTab[]>(
		session.isAdmin
			? [
					{ value: 'org', label: 'Organisation', description: 'Private to admins. Share single files with people as needed.' },
					{ value: 'shared', label: 'Shared', description: 'Everyone in the organisation can use these on their cards.' },
					{ value: 'personal', label: 'Users’ files', description: 'What each person uploaded for themselves.' }
				]
			: [
					{ value: 'personal', label: 'My files', description: 'Your photos and brochures. Only you and your admins see them.' },
					{ value: 'shared', label: 'Shared', description: `From ${session.orgName}, for everyone to use.` },
					{ value: 'granted', label: 'Shared with me', description: `Files ${session.orgName} gave you access to.` }
				]
	);

	const initialArea = appPage.url.searchParams.get('area') as Area | null;
	let area = $state<Area>(initialArea ?? (session.isAdmin ? 'org' : 'personal'));
	let owner = $state<number | null>(null);
	let accessTarget = $state<LibraryFile | null>(null);

	const tab = $derived(areaTabs.find((t) => t.value === area) ?? areaTabs[0]);
	// Members upload to their own files; admins to the organisation's or the shared area.
	const uploadArea = $derived<FileArea | null>(
		session.isAdmin ? (area === 'org' || area === 'shared' ? area : null) : area === 'personal' ? 'personal' : null
	);

	function canEdit(file: LibraryFile) {
		return session.isAdmin || (file.area === 'personal' && file.owner?.username === session.username);
	}

	const tabs: { value: FileKind | ''; label: string }[] = [
		{ value: '', label: 'All' },
		{ value: 'image', label: 'Photos' },
		{ value: 'pdf', label: 'PDFs' }
	];

	let kind = $state<FileKind | ''>('');
	let page = $state(1);
	let pageSize = $state(24);
	let files = $state<LibraryFile[] | null>(null);
	let total = $state(0);
	let loading = $state(false);
	let error = $state('');
	let reload = $state(0);

	let renaming = $state<string | null>(null);
	let renameValue = $state('');
	let deleteTarget = $state<LibraryFile | null>(null);
	let deleting = $state(false);

	let lastFilter = '';
	let requestId = 0;
	$effect(() => {
		void reload;
		if (!storage.ready) return;
		const filter = JSON.stringify([kind, area, owner]);
		if (filter !== lastFilter) {
			const changed = lastFilter !== '';
			lastFilter = filter;
			if (changed) files = null;
			if (changed && page !== 1) {
				page = 1;
				return;
			}
		}
		const id = ++requestId;
		loading = true;
		error = '';
		listFiles({
			kind: kind || undefined,
			area,
			userId: session.isAdmin && area === 'personal' && owner !== null ? owner : undefined,
			page,
			pageSize
		})
			.then((res) => {
				if (id !== requestId) return;
				if (res.files.length === 0 && res.total > 0 && page > 1) {
					page = Math.ceil(res.total / pageSize);
					return;
				}
				files = res.files;
				total = res.total;
			})
			.catch((e) => {
				if (id === requestId) error = e instanceof Error ? e.message : 'Failed to load files';
			})
			.finally(() => {
				if (id === requestId) loading = false;
			});
	});

	// Which cards use each file, from the card data the sidebar already loaded.
	const usage = $derived.by(() => {
		const map = new Map<string, string[]>();
		const add = (id: string, name: string) => {
			if (!id) return;
			const names = map.get(id) ?? [];
			if (!names.includes(name)) names.push(name);
			map.set(id, names);
		};
		for (const p of cards.list ?? []) {
			const c = normalizeCard(p.data);
			const name = c.name || p.slug;
			add(c.avatar_file, name);
			add(c.cover_file, name);
			for (const d of c.documents) add(d.file, name);
			for (const b of c.blocks) if (b.type === 'gallery') for (const img of b.images) add(img.file, name);
		}
		return map;
	});

	function startRename(file: LibraryFile) {
		renaming = file.id;
		renameValue = file.title || file.name.replace(/\.[^.]+$/, '');
	}

	async function commitRename(file: LibraryFile) {
		if (renaming !== file.id) return;
		renaming = null;
		const title = renameValue.trim();
		if (title === file.title) return;
		try {
			const updated = await renameFile(file.id, title);
			if (files) files = files.map((f) => (f.id === updated.id ? updated : f));
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Rename failed');
		}
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await deleteFile(deleteTarget.id);
			toast.success('File deleted');
			deleteTarget = null;
			reload++;
			if (session.username) storage.load(session.username, true);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Delete failed');
		} finally {
			deleting = false;
		}
	}
</script>

<svelte:head>
	<title>Files · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Files</h1>
		<p class="text-muted-foreground text-sm">
			Photos and PDF brochures in {session.orgName}'s storage. Upload once, use on any card.
		</p>
	</header>

	{#if !storage.status}
		<Skeleton class="h-40 rounded-xl" />
	{:else if !storage.ready}
		<div class="bg-card flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-16 text-center">
			<span class="bg-muted text-muted-foreground grid size-11 place-items-center rounded-full">
				<FolderIcon class="size-5" />
			</span>
			<p class="font-medium">
				{#if !storage.status.enabled}
					File storage isn't enabled on this server
				{:else if session.isOwner}
					Connect your storage to upload files
				{:else}
					{session.orgName} hasn't connected storage yet
				{/if}
			</p>
			<p class="text-muted-foreground max-w-md text-sm">
				{#if !storage.status.enabled}
					Ask whoever runs this server to set SECRETS_KEY.
				{:else if session.isOwner}
					Photos and brochures are stored in your own S3-compatible bucket (Cloudflare R2, Backblaze B2, AWS S3 or
					MinIO). Everyone in your organisation uploads to it.
				{:else}
					Ask the owner to connect a bucket in Settings. Until then, nobody can upload photos or brochures.
				{/if}
			</p>
			{#if storage.status.enabled && session.isOwner}
				<Button href="/dashboard/settings?tab=storage" class="mt-1">Connect storage</Button>
			{/if}
		</div>
	{:else}
		<div class="flex flex-col gap-4">
			<div class="flex flex-wrap items-center gap-3 border-b">
				<div class="-mb-px flex gap-1 overflow-x-auto" role="tablist" aria-label="Area">
					{#each areaTabs as t (t.value)}
						<button
							type="button"
							role="tab"
							aria-selected={area === t.value}
							onclick={() => (area = t.value)}
							class={cn(
								'border-b-2 px-3 py-2 text-sm font-medium whitespace-nowrap transition-colors',
								area === t.value
									? 'border-foreground text-foreground'
									: 'text-muted-foreground hover:text-foreground border-transparent'
							)}
						>
							{t.label}
						</button>
					{/each}
				</div>
			</div>
			<p class="text-muted-foreground text-sm">{tab.description}</p>
		</div>

		{#if !session.isAdmin && area === 'personal' && storage.status}
			<StorageMeter used={storage.status.used_bytes} quota={storage.status.quota_bytes} class="max-w-sm" />
		{/if}

		{#if uploadArea}
			<FileDropzone
				area={session.isAdmin ? uploadArea : undefined}
				onuploaded={() => {
					reload++;
					if (!session.isAdmin && session.username) storage.load(session.username, true);
				}}
			/>
		{/if}

		<div class="flex flex-wrap items-center justify-between gap-3">
			<div class="flex flex-wrap items-center gap-2">
				<div class="bg-muted inline-flex rounded-lg p-[3px]" role="tablist" aria-label="File type">
					{#each tabs as t (t.value)}
						<button
							type="button"
							role="tab"
							aria-selected={kind === t.value}
							onclick={() => (kind = t.value)}
							class={cn(
								'h-8 rounded-md px-3 text-sm font-medium transition-colors',
								kind === t.value ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
							)}
						>
							{t.label}
						</button>
					{/each}
				</div>
				{#if session.isAdmin && area === 'personal'}
					<UserPicker
						value={owner}
						onchange={(v) => (owner = typeof v === 'number' ? v : null)}
						allLabel="Everyone"
						allowNone={false}
					/>
				{/if}
			</div>
			{#if loading && files}
				<Spinner class="text-muted-foreground size-4" />
			{/if}
		</div>

		{#if error && !files}
			<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
				<p class="font-medium">Couldn't load your files</p>
				<p class="text-muted-foreground text-sm">{error}</p>
				<Button variant="outline" onclick={() => reload++}>Try again</Button>
			</div>
		{:else if files === null}
			<div class="grid gap-4 grid-cols-2 sm:[grid-template-columns:repeat(auto-fill,minmax(200px,1fr))]">
				{#each [1, 2, 3, 4, 5] as i (i)}
					<Skeleton class="aspect-[4/5] rounded-xl" />
				{/each}
			</div>
		{:else if files.length === 0}
			<p class="text-muted-foreground py-10 text-center text-sm">
				{#if kind !== ''}
					No {kind === 'image' ? 'photos' : 'PDFs'} here yet.
				{:else if uploadArea}
					No files yet. Drop a photo or PDF above.
				{:else if area === 'granted'}
					Nothing has been shared with you yet.
				{:else if area === 'shared'}
					Nothing shared yet.
				{:else}
					No files here yet.
				{/if}
			</p>
		{:else}
			<ul
				class={cn(
					'grid gap-4 transition-opacity grid-cols-2 sm:[grid-template-columns:repeat(auto-fill,minmax(200px,1fr))]',
					loading && 'opacity-60'
				)}
			>
				{#each files as file (file.id)}
					{@const usedOn = usage.get(file.id) ?? []}
					{@const editable = canEdit(file)}
					{@const shareable = session.isAdmin && file.area !== 'shared'}
					<li class="bg-card group flex flex-col overflow-hidden rounded-xl border">
						<a href={fileUrl(file.id)} target="_blank" rel="noopener" class="block" aria-label="Open {file.title || file.name}">
							<FileThumb id={file.id} kind={file.kind} class="aspect-[4/3] w-full" />
						</a>
						<div class="flex flex-1 flex-col gap-1 p-3">
							<div class="flex items-start gap-1">
								{#if renaming === file.id}
									<Input
										bind:value={renameValue}
										class="h-8 text-sm"
										aria-label="File title"
										autofocus
										onblur={() => commitRename(file)}
										onkeydown={(e) => {
											if (e.key === 'Enter') e.currentTarget.blur();
											if (e.key === 'Escape') renaming = null;
										}}
									/>
								{:else}
									<p class="min-w-0 flex-1 truncate text-sm font-medium" title={file.title || file.name}>
										{file.title || file.name}
									</p>
									<DropdownMenu.Root>
										<DropdownMenu.Trigger
											class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: '-mt-1 -mr-1 size-7' })}
											aria-label="File actions"
										>
											<EllipsisIcon />
										</DropdownMenu.Trigger>
										<DropdownMenu.Content align="end" class="w-44">
											<DropdownMenu.Group>
												<DropdownMenu.Item onSelect={() => window.open(fileUrl(file.id), '_blank')}>
													<ExternalLinkIcon />
													Open
												</DropdownMenu.Item>
												{#if editable}
													<DropdownMenu.Item onSelect={() => startRename(file)}>
														<PencilIcon />
														Rename
													</DropdownMenu.Item>
												{/if}
												{#if shareable}
													<DropdownMenu.Item onSelect={() => (accessTarget = file)}>
														<UserRoundCheckIcon />
														Manage access
													</DropdownMenu.Item>
												{/if}
											</DropdownMenu.Group>
											{#if editable}
												<DropdownMenu.Separator />
												<DropdownMenu.Group>
													<DropdownMenu.Item variant="destructive" onSelect={() => (deleteTarget = file)}>
														<Trash2Icon />
														Delete
													</DropdownMenu.Item>
												</DropdownMenu.Group>
											{/if}
										</DropdownMenu.Content>
									</DropdownMenu.Root>
								{/if}
							</div>
							<p class="text-muted-foreground text-xs">
								{file.kind === 'pdf' ? 'PDF' : 'Photo'} · {formatBytes(file.size_bytes)} · {timeAgo(file.created_at)}
							</p>
							{#if session.isAdmin && file.area === 'personal' && file.owner}
								<p class="text-muted-foreground truncate text-xs">{file.owner.username}'s file</p>
							{:else if file.former_owner}
								<p class="text-muted-foreground truncate text-xs" title="Moved here when {file.former_owner} was deleted">
									From {file.former_owner}
								</p>
							{/if}
							<p class="text-muted-foreground mt-auto truncate pt-1 text-xs" title={usedOn.join(', ')}>
								{usedOn.length ? `Used on ${usedOn.join(', ')}` : 'Not used on a card'}
							</p>
						</div>
					</li>
				{/each}
			</ul>
			<Pagination bind:page bind:pageSize {total} pageSizes={[12, 24, 48, 96]} disabled={loading} />
		{/if}
	{/if}
</div>

<AlertDialog.Root open={deleteTarget !== null} onOpenChange={(open) => !open && (deleteTarget = null)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete {deleteTarget?.title || deleteTarget?.name}?</AlertDialog.Title>
			<AlertDialog.Description>
				{@const usedOn = deleteTarget ? (usage.get(deleteTarget.id) ?? []) : []}
				It's removed from {session.orgName}'s storage permanently.
				{#if usedOn.length}
					It's used on {plural(usedOn.length, 'card')} ({usedOn.join(', ')}), which will stop showing it.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{#if deleting}<Spinner data-icon="inline-start" />{/if}
				Delete file
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<FileAccessDialog bind:file={accessTarget} />
