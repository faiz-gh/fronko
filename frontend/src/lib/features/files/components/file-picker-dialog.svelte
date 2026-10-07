<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CheckIcon from '@lucide/svelte/icons/check';
	import CropIcon from '@lucide/svelte/icons/crop';
	import SearchIcon from '@lucide/svelte/icons/search';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import {
		ACCEPT,
		checkUpload,
		fetchFileContent,
		fileCounts,
		fileTitle,
		formatBytes,
		listFiles,
		locationLabel,
		PURPOSES,
		purposesFor,
		uploadFile,
		type CropSpec,
		type FileKind,
		type FilePurpose,
		type LibraryFile
	} from '$lib/features/files/api';
	import { teamColor, type TeamRef } from '$lib/features/teams/api';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import FileThumb from './file-thumb.svelte';
	import ImageCropDialog from '$lib/components/shared/image-crop-dialog.svelte';
	import Pagination from '$lib/components/shared/pagination.svelte';
	import PurposeChips from './purpose-chips.svelte';
	import { fileLocations } from '$lib/features/files/locations';
	import { session } from '$lib/core/session.svelte';
	import { teams } from '$lib/features/teams/store.svelte';
	import { cn } from '$lib/utils';

	let {
		open = $bindable(false),
		purpose,
		kind = PURPOSES[purpose].kind,
		title,
		selected = [],
		excludePersonal = false,
		crop,
		onselect
	}: {
		open?: boolean;
		/** What the file is for: the picker starts on that purpose and uploads get it. */
		purpose: FilePurpose;
		kind?: FileKind;
		title: string;
		/** Ids already in use, marked in the grid. */
		selected?: string[];
		/** Leave out personal files (organisation branding can't use them). */
		excludePersonal?: boolean;
		/** Frame images to this shape: new uploads always, picked files when the user asks. */
		crop?: CropSpec;
		onselect: (file: LibraryFile) => void;
	} = $props();

	const who = $derived({
		isAdmin: session.isAdmin,
		orgName: session.orgName,
		teams: session.isAdmin
			? (teams.list ?? []).map<TeamRef>((t) => ({ id: t.id, name: t.name, color: t.color }))
			: session.teams,
		ledTeamIds: session.ledTeams.map((t) => t.id)
	});
	const locations = $derived(
		fileLocations(who).filter((l) => !excludePersonal || (l.key !== 'all' && l.query.area !== 'personal'))
	);

	let locKey = $state('');
	let filter = $state<FilePurpose | null>(null);
	let search = $state('');
	let query = $state('');
	let files = $state<LibraryFile[] | null>(null);
	let total = $state(0);
	let counts = $state<Partial<Record<FilePurpose, number>> | undefined>(undefined);
	let countTotal = $state<number | undefined>(undefined);
	let page = $state(1);
	let pageSize = $state(12);
	let error = $state('');
	let chosen = $state<LibraryFile | null>(null);
	let cropSource = $state<File | null>(null);
	let busy = $state(false);
	let progress = $state<number | null>(null);
	let input: HTMLInputElement | undefined = $state();

	const location = $derived(locations.find((l) => l.key === locKey) ?? locations[0]);
	const purposes = $derived(purposesFor(kind));

	// Every time it opens: start on this purpose, in the first location, with nothing chosen.
	$effect(() => {
		if (!open) return;
		filter = purpose;
		locKey = '';
		search = '';
		query = '';
		chosen = null;
	});

	$effect(() => {
		const value = search;
		const t = setTimeout(() => (query = value.trim()), 250);
		return () => clearTimeout(t);
	});

	$effect(() => {
		void location;
		void filter;
		void query;
		page = 1;
	});

	let requestId = 0;
	$effect(() => {
		if (!open || !location) return;
		const base = { ...location.query, kind, q: query || undefined };
		const id = ++requestId;
		error = '';
		Promise.all([
			listFiles({ ...base, purposes: filter ? [filter] : undefined, page, pageSize }),
			fileCounts(base)
		])
			.then(([res, c]) => {
				if (id !== requestId) return;
				files = res.files;
				total = res.total;
				counts = c.counts;
				countTotal = c.total;
			})
			.catch((e) => {
				if (id === requestId) error = e instanceof Error ? e.message : 'Failed to load files';
			});
	});

	function finish(file: LibraryFile) {
		onselect(file);
		open = false;
	}

	/** Clicking a file: use it straight away, unless it could be cropped first. */
	function pick(file: LibraryFile) {
		if (crop && file.kind === 'image') chosen = chosen?.id === file.id ? null : file;
		else finish(file);
	}

	async function cropChosen() {
		if (!chosen) return;
		busy = true;
		try {
			cropSource = await fetchFileContent(chosen);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not open that image');
		} finally {
			busy = false;
		}
	}

	function onFilePicked(file: File | undefined) {
		if (!file) return;
		const problem = checkUpload(file, kind);
		if (problem) {
			toast.error(problem);
			return;
		}
		if (crop && kind === 'image') cropSource = file;
		else upload(file);
	}

	/** Uploads a new file (or a cropped copy) where the user is looking, and uses it. */
	async function upload(file: File, titleHint = '') {
		progress = 0;
		try {
			const target = location?.upload ?? undefined;
			const uploaded = await uploadFile(file, {
				area: target?.area,
				teamId: target?.teamId,
				purpose,
				title: titleHint,
				onProgress: (p) => (progress = p)
			});
			finish(uploaded);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Upload failed');
		} finally {
			progress = null;
		}
	}

	const uploadHint = $derived(
		location?.upload
			? `Uploads go to ${location.label}.`
			: session.isAdmin
				? 'Uploads go to the organisation’s files.'
				: 'Uploads go to your files.'
	);
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="flex max-h-[92svh] flex-col gap-4 sm:max-w-4xl">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			<Dialog.Description>
				{PURPOSES[purpose].hint}. Pick one you can use, or upload a new one. {uploadHint}
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex flex-wrap items-center gap-2">
			<div class="relative min-w-40 flex-1">
				<SearchIcon class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
				<Input bind:value={search} placeholder="Search by name" class="pl-9" aria-label="Search files" />
			</div>
			{#if location}
				<Select.Root type="single" value={location.key} onValueChange={(v) => (locKey = v)}>
					<Select.Trigger class="w-48" aria-label="Location">
						<span class="flex items-center gap-2 truncate">
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
			{/if}
			<Button variant="outline" onclick={() => input?.click()} disabled={progress !== null}>
				{#if progress !== null}
					<Spinner data-icon="inline-start" />
					{Math.round(progress * 100)}%
				{:else}
					<UploadIcon data-icon="inline-start" />
					Upload new
				{/if}
			</Button>
			<input
				bind:this={input}
				type="file"
				class="sr-only"
				tabindex="-1"
				accept={ACCEPT[kind]}
				onchange={(e) => {
					onFilePicked(e.currentTarget.files?.[0]);
					e.currentTarget.value = '';
				}}
			/>
		</div>

		{#if purposes.length > 1}
			<PurposeChips bind:value={filter} {purposes} {counts} total={countTotal} />
		{/if}

		<div class="-mx-1 min-h-48 flex-1 overflow-y-auto px-1 py-0.5">
			{#if error}
				<p class="text-destructive text-sm">{error}</p>
			{:else if files === null}
				<div class="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5">
					{#each [1, 2, 3, 4, 5] as i (i)}
						<Skeleton class="aspect-square rounded-lg" />
					{/each}
				</div>
			{:else if files.length === 0}
				<div class="text-muted-foreground flex flex-col items-center gap-2 py-10 text-center text-sm">
					<p>
						{#if query}
							Nothing called “{query}” here.
						{:else if filter}
							No {PURPOSES[filter].plural.toLowerCase()} here yet.
						{:else}
							No {kind === 'image' ? 'images' : 'PDFs'} here yet.
						{/if}
					</p>
					{#if filter && filter !== purpose}
						<Button variant="link" size="sm" onclick={() => (filter = null)}>Show everything</Button>
					{:else if filter}
						<Button variant="link" size="sm" onclick={() => (filter = null)}>Show other {kind === 'image' ? 'images' : 'PDFs'}</Button>
					{/if}
				</div>
			{:else}
				<ul class="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5">
					{#each files as file (file.id)}
						{@const inUse = selected.includes(file.id)}
						{@const isChosen = chosen?.id === file.id}
						<li>
							<button
								type="button"
								onclick={() => pick(file)}
								ondblclick={() => finish(file)}
								aria-pressed={crop ? isChosen : undefined}
								class="group focus-visible:ring-ring/50 flex w-full flex-col gap-1.5 rounded-lg text-left outline-none focus-visible:ring-3"
							>
								<span class="relative block">
									<FileThumb
										id={file.id}
										kind={file.kind}
										hasThumb={file.has_thumb}
										fit={file.purpose === 'logo' || file.purpose === 'banner' ? 'contain' : 'cover'}
										class={cn(
											'aspect-square w-full rounded-lg ring-1 ring-black/5 transition-shadow group-hover:ring-2 group-hover:ring-foreground/40',
											(inUse || isChosen) && 'ring-brand ring-2'
										)}
									/>
									{#if inUse || isChosen}
										<span class="bg-brand text-brand-foreground absolute top-1.5 right-1.5 grid size-5 place-items-center rounded-full">
											<CheckIcon class="size-3" />
										</span>
									{/if}
									{#if file.purpose !== purpose}
										<span
											class="bg-background/90 absolute bottom-1.5 left-1.5 rounded-full px-1.5 py-0.5 text-[10px] font-medium shadow-sm"
										>
											{PURPOSES[file.purpose].label}
										</span>
									{/if}
								</span>
								<span class="truncate text-xs font-medium">{fileTitle(file)}</span>
								<span class="text-muted-foreground -mt-1 truncate text-[11px]">
									{formatBytes(file.size_bytes)}{location?.key === 'all' ? ` · ${locationLabel(file)}` : ''}
								</span>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		{#if files && total > pageSize}
			<Pagination bind:page bind:pageSize {total} pageSizes={[12, 24, 48]} />
		{/if}

		<Dialog.Footer class="items-center gap-2 sm:justify-between">
			<p class="text-muted-foreground hidden text-xs sm:block">
				{#if crop && chosen}
					Use it as it is, or crop it to fit. Cropping saves a copy; the original stays.
				{:else if crop}
					Pick an image to use or crop.
				{/if}
			</p>
			<div class="flex flex-col-reverse gap-2 sm:flex-row">
				<Dialog.Close>
					{#snippet child({ props })}
						<Button variant="outline" {...props}>Cancel</Button>
					{/snippet}
				</Dialog.Close>
				{#if crop}
					<Button variant="outline" disabled={!chosen || busy} onclick={() => chosen && finish(chosen)}>Use as is</Button>
					<Button disabled={!chosen || busy} onclick={cropChosen}>
						{#if busy}<Spinner data-icon="inline-start" />{:else}<CropIcon data-icon="inline-start" />{/if}
						Crop & use
					</Button>
				{/if}
			</div>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

{#if crop}
	<ImageCropDialog
		bind:file={cropSource}
		title={crop.title}
		aspect={crop.aspect}
		shape={crop.shape}
		outputWidth={crop.width}
		outputHeight={crop.height}
		format={crop.format}
		ratios={crop.ratios}
		onconfirm={(f) => upload(f, chosen?.title ? `${chosen.title} (cropped)` : '')}
	/>
{/if}
