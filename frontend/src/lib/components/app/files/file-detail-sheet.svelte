<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import CropIcon from '@lucide/svelte/icons/crop';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import LinkIcon from '@lucide/svelte/icons/link';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import UserRoundCheckIcon from '@lucide/svelte/icons/user-round-check';
	import {
		fetchFileContent,
		fileDimensions,
		fileTitle,
		fileUrl,
		fileUsage,
		formatBytes,
		locationLabel,
		PURPOSES,
		purposesFor,
		SLOT_LABEL,
		updateFile,
		uploadFile,
		type CropSpec,
		type FilePurpose,
		type FileUsage,
		type LibraryFile
	} from '$lib/api/files';
	import { teamColor } from '$lib/api/teams';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import FileThumb from '../file-thumb.svelte';
	import ImageCropDialog from '../image-crop-dialog.svelte';
	import PurposeIcon from './purpose-icon.svelte';
	import { locationKeyOf, type MoveTarget } from '$lib/file-locations';
	import { timeAgo } from '$lib/format';

	let {
		file = $bindable(null),
		editable,
		targets,
		onchanged,
		ondelete,
		onaccess,
		oncopied
	}: {
		/** The file shown; the sheet is open while this is set. */
		file?: LibraryFile | null;
		editable: (file: LibraryFile) => boolean;
		/** Where the user may move files. */
		targets: MoveTarget[];
		onchanged: (file: LibraryFile) => void;
		ondelete: (file: LibraryFile) => void;
		/** Admins: manage who can use the file. */
		onaccess?: (file: LibraryFile) => void;
		/** A cropped copy was uploaded. */
		oncopied: (file: LibraryFile) => void;
	} = $props();

	/** For images whose purpose has no fixed shape. */
	const FREE_CROP: CropSpec = {
		title: 'Crop image',
		aspect: 1,
		width: 1200,
		height: 1200,
		ratios: [
			{ label: '1:1', aspect: 1, width: 1200, height: 1200 },
			{ label: '4:3', aspect: 4 / 3, width: 1600, height: 1200 },
			{ label: '16:9', aspect: 16 / 9, width: 1600, height: 900 },
			{ label: '3:4', aspect: 3 / 4, width: 1200, height: 1600 }
		]
	};

	let title = $state('');
	let usage = $state<FileUsage | null>(null);
	let usageError = $state('');
	let saving = $state(false);
	let cropSource = $state<File | null>(null);
	let fetching = $state(false);
	let copying = $state(false);

	const canEdit = $derived(file ? editable(file) : false);
	const cropSpec = $derived(file ? (PURPOSES[file.purpose].crop ?? FREE_CROP) : FREE_CROP);
	const moveOptions = $derived(file && canEdit ? targets : []);

	let shownId = '';
	$effect(() => {
		const f = file;
		if (!f) {
			shownId = '';
			return;
		}
		if (f.id === shownId) return;
		shownId = f.id;
		title = f.title || '';
		usage = null;
		usageError = '';
		fileUsage(f.id)
			.then((u) => {
				if (file?.id === f.id) usage = u;
			})
			.catch((e) => (usageError = e instanceof Error ? e.message : 'Could not load where it’s used'));
	});

	async function save(patch: Parameters<typeof updateFile>[1], message?: string) {
		if (!file) return;
		saving = true;
		try {
			const updated = await updateFile(file.id, patch);
			file = updated;
			onchanged(updated);
			if (message) toast.success(message);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not save');
		} finally {
			saving = false;
		}
	}

	function commitTitle() {
		if (!file) return;
		const next = title.trim();
		if (next === file.title) return;
		save({ title: next });
	}

	function move(key: string) {
		const target = targets.find((t) => t.key === key);
		if (!target || !file) return;
		save({ area: target.area, team_id: target.teamId }, `Moved to ${target.label}`);
	}

	async function copyLink() {
		if (!file) return;
		try {
			await navigator.clipboard.writeText(new URL(fileUrl(file.id), location.origin).href);
			toast.success('Link copied');
		} catch {
			toast.error('Couldn’t copy the link');
		}
	}

	async function startCrop() {
		if (!file) return;
		fetching = true;
		try {
			cropSource = await fetchFileContent(file);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not open that image');
		} finally {
			fetching = false;
		}
	}

	async function uploadCopy(cropped: File) {
		const source = file;
		if (!source) return;
		copying = true;
		try {
			// The copy goes where the original is when the user can add files there.
			const where = canEdit && source.area !== 'personal' ? { area: source.area, teamId: source.team?.id } : {};
			const copy = await uploadFile(cropped, {
				...where,
				title: source.title ? `${source.title} (cropped)` : '',
				purpose: source.purpose
			});
			toast.success('Cropped copy saved');
			oncopied(copy);
			file = copy;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not save the copy');
		} finally {
			copying = false;
		}
	}
</script>

<Sheet.Root open={file !== null} onOpenChange={(open) => !open && (file = null)}>
	<Sheet.Content class="w-full gap-0 overflow-y-auto p-0 sm:max-w-md">
		{#if file}
			{@const dims = fileDimensions(file)}
			<Sheet.Header class="border-b px-5 py-4 pr-12">
				<Sheet.Title class="truncate">{fileTitle(file)}</Sheet.Title>
				<Sheet.Description class="flex items-center gap-1.5">
					<PurposeIcon purpose={file.purpose} class="size-3.5" />
					{PURPOSES[file.purpose].label} · {locationLabel(file)}
				</Sheet.Description>
			</Sheet.Header>

			<div class="flex flex-col gap-6 p-5">
				<!-- Preview -->
				<a
					href={fileUrl(file.id)}
					target="_blank"
					rel="noopener"
					class="group focus-visible:ring-ring/50 block overflow-hidden rounded-xl border outline-none focus-visible:ring-3"
					aria-label="Open {fileTitle(file)} in a new tab"
				>
					<FileThumb
						id={file.id}
						kind={file.kind}
						hasThumb={file.has_thumb}
						full
						fit={file.kind === 'image' ? 'contain' : 'cover'}
						class={file.kind === 'pdf' ? 'aspect-[3/4] w-full' : 'aspect-[4/3] w-full'}
					/>
				</a>

				<div class="flex flex-wrap gap-2">
					<Button variant="outline" size="sm" href={fileUrl(file.id)} target="_blank" rel="noopener">
						<ExternalLinkIcon data-icon="inline-start" />
						Open
					</Button>
					<Button variant="outline" size="sm" onclick={copyLink}>
						<CopyIcon data-icon="inline-start" />
						Copy link
					</Button>
					{#if file.kind === 'image'}
						<Button variant="outline" size="sm" onclick={startCrop} disabled={fetching || copying}>
							{#if fetching || copying}<Spinner data-icon="inline-start" />{:else}<CropIcon data-icon="inline-start" />{/if}
							Crop a copy
						</Button>
					{/if}
					{#if onaccess && file.area !== 'shared'}
						<Button variant="outline" size="sm" onclick={() => file && onaccess(file)}>
							<UserRoundCheckIcon data-icon="inline-start" />
							Access
						</Button>
					{/if}
				</div>

				<!-- Details -->
				<section class="flex flex-col gap-4">
					<label class="flex flex-col gap-1.5">
						<span class="text-muted-foreground text-xs font-medium">Title</span>
						{#if canEdit}
							<Input
								bind:value={title}
								placeholder={file.name}
								maxlength={120}
								onblur={commitTitle}
								onkeydown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
							/>
						{:else}
							<span class="text-sm">{fileTitle(file)}</span>
						{/if}
					</label>

					<div class="flex flex-col gap-1.5">
						<span class="text-muted-foreground text-xs font-medium">What it’s for</span>
						{#if canEdit}
							<Select.Root
								type="single"
								value={file.purpose}
								onValueChange={(v) => file && v !== file.purpose && save({ purpose: v as FilePurpose })}
								disabled={saving}
							>
								<Select.Trigger class="w-full">
									<span class="flex items-center gap-2">
										<PurposeIcon purpose={file.purpose} class="text-muted-foreground size-4" />
										{PURPOSES[file.purpose].label}
									</span>
								</Select.Trigger>
								<Select.Content>
									{#each purposesFor(file.kind) as p (p)}
										<Select.Item value={p} label={PURPOSES[p].label}>
											<span class="flex flex-col">
												<span class="flex items-center gap-2">
													<PurposeIcon purpose={p} class="text-muted-foreground size-4" />
													{PURPOSES[p].label}
												</span>
												<span class="text-muted-foreground pl-6 text-xs">{PURPOSES[p].hint}</span>
											</span>
										</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						{:else}
							<span class="text-sm">{PURPOSES[file.purpose].label}</span>
						{/if}
					</div>

					<div class="flex flex-col gap-1.5">
						<span class="text-muted-foreground text-xs font-medium">Location</span>
						{#if moveOptions.length > 0}
							<Select.Root type="single" value={locationKeyOf(file)} onValueChange={move} disabled={saving}>
								<Select.Trigger class="w-full">
									<span class="flex items-center gap-2">
										{#if file.team}<span class="size-2 rounded-full" style="background: {teamColor(file.team)}"></span>{/if}
										{locationLabel(file)}
									</span>
								</Select.Trigger>
								<Select.Content>
									{#if file.area === 'personal'}
										<Select.Item value="personal" label={locationLabel(file)} disabled>{locationLabel(file)}</Select.Item>
									{/if}
									{#each moveOptions as t (t.key)}
										<Select.Item value={t.key} label={t.label}>
											<span class="flex items-center gap-2">
												{#if t.team}<span class="size-2 rounded-full" style="background: {teamColor(t.team)}"></span>{/if}
												{t.label}
											</span>
										</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
							{#if file.area === 'personal'}
								<p class="text-muted-foreground text-xs">
									Moving it out of personal files frees up {file.owner?.username ?? 'their'} storage space.
								</p>
							{/if}
						{:else}
							<span class="flex items-center gap-2 text-sm">
								{#if file.team}<span class="size-2 rounded-full" style="background: {teamColor(file.team)}"></span>{/if}
								{locationLabel(file)}
							</span>
						{/if}
					</div>

					<dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
						<dt class="text-muted-foreground">Type</dt>
						<dd>{file.kind === 'pdf' ? 'PDF' : file.content_type.replace('image/', '').toUpperCase()}</dd>
						<dt class="text-muted-foreground">Size</dt>
						<dd>{formatBytes(file.size_bytes)}</dd>
						{#if dims}
							<dt class="text-muted-foreground">{file.kind === 'pdf' ? 'Pages' : 'Dimensions'}</dt>
							<dd>{dims}</dd>
						{/if}
						<dt class="text-muted-foreground">Uploaded</dt>
						<dd>
							{timeAgo(file.created_at)}{file.owner ? ` by ${file.owner.username}` : ''}
						</dd>
						<dt class="text-muted-foreground">File name</dt>
						<dd class="truncate" title={file.name}>{file.name}</dd>
					</dl>
				</section>

				<!-- Usage -->
				<section class="flex flex-col gap-2">
					<h3 class="flex items-center gap-1.5 text-sm font-medium">
						<LinkIcon class="text-muted-foreground size-4" />
						Where it’s used
					</h3>
					{#if usageError}
						<p class="text-destructive text-sm">{usageError}</p>
					{:else if !usage}
						<Skeleton class="h-10" />
					{:else if usage.cards.length === 0 && !usage.org_logo && !usage.signature_banner && usage.hidden_cards === 0}
						<p class="text-muted-foreground text-sm">Not used anywhere yet.</p>
					{:else}
						<ul class="flex flex-col divide-y rounded-lg border text-sm">
							{#if usage.org_logo}
								<li>
									<a href="/dashboard/settings?tab=branding" class="hover:bg-muted/60 flex justify-between gap-3 px-3 py-2">
										<span class="font-medium">Organisation logo</span>
										<span class="text-muted-foreground">Branding</span>
									</a>
								</li>
							{/if}
							{#if usage.signature_banner}
								<li>
									<a href="/dashboard/settings?tab=branding" class="hover:bg-muted/60 flex justify-between gap-3 px-3 py-2">
										<span class="font-medium">Email signature banner</span>
										<span class="text-muted-foreground">Branding</span>
									</a>
								</li>
							{/if}
							{#each usage.cards as c (`${c.profile_id}-${c.slot}`)}
								<li>
									<a href="/dashboard/{c.profile_id}" class="hover:bg-muted/60 flex justify-between gap-3 px-3 py-2">
										<span class="truncate font-medium">{c.name || c.slug}</span>
										<span class="text-muted-foreground shrink-0">{SLOT_LABEL[c.slot]}</span>
									</a>
								</li>
							{/each}
							{#if usage.hidden_cards > 0}
								<li class="text-muted-foreground px-3 py-2">
									{usage.hidden_cards} other {usage.hidden_cards === 1 ? 'card' : 'cards'} you can’t see
								</li>
							{/if}
						</ul>
					{/if}
				</section>

				{#if canEdit}
					<Button variant="outline" class="text-destructive hover:text-destructive self-start" onclick={() => file && ondelete(file)}>
						<Trash2Icon data-icon="inline-start" />
						Delete file
					</Button>
				{/if}
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>

<ImageCropDialog
	bind:file={cropSource}
	title={cropSpec.title}
	aspect={cropSpec.aspect}
	shape={cropSpec.shape}
	outputWidth={cropSpec.width}
	outputHeight={cropSpec.height}
	format={cropSpec.format}
	ratios={cropSpec.ratios}
	onconfirm={uploadCopy}
/>
