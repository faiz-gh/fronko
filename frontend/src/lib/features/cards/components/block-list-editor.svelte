<script lang="ts">
	import { toast } from 'svelte-sonner';
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import CalendarCheckIcon from '@lucide/svelte/icons/calendar-check';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import EyeIcon from '@lucide/svelte/icons/eye';
	import EyeOffIcon from '@lucide/svelte/icons/eye-off';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import HeadingIcon from '@lucide/svelte/icons/heading';
	import IdCardIcon from '@lucide/svelte/icons/id-card';
	import ImagesIcon from '@lucide/svelte/icons/images';
	import LinkIcon from '@lucide/svelte/icons/link';
	import MinusIcon from '@lucide/svelte/icons/minus';
	import MousePointerClickIcon from '@lucide/svelte/icons/mouse-pointer-click';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TextIcon from '@lucide/svelte/icons/text';
	import TicketIcon from '@lucide/svelte/icons/ticket';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import UserRoundIcon from '@lucide/svelte/icons/user-round';
	import XIcon from '@lucide/svelte/icons/x';
	import { ACCEPT, checkUpload, fileUrl, uploadFile, type LibraryFile, type PublicFile } from '$lib/features/files/api';
	import {
		BLOCK_INFO,
		HEADER_STYLES,
		MAX_BLOCKS,
		MAX_CAPTION,
		MAX_EVENT_FIELD,
		MAX_GALLERY_IMAGES,
		MAX_HEADING,
		MAX_TEXT,
		SINGLETON_BLOCKS,
		newBlock,
		type BlockOf,
		type BlockType,
		type CardBlock,
		type HeaderStyle
	} from '$lib/features/cards/blocks';
	import { newId } from '$lib/features/cards/card';
	import { storage } from '$lib/features/files/storage.svelte';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import FilePickerDialog from '$lib/features/files/components/file-picker-dialog.svelte';
	import { cn } from '$lib/utils';

	let {
		blocks = $bindable(),
		files,
		onfile
	}: {
		blocks: CardBlock[];
		/** Library file metadata, for gallery thumbnails' names. */
		files: Record<string, PublicFile>;
		/** Called with each file added to a gallery, so the editor can remember its metadata. */
		onfile: (file: LibraryFile) => void;
	} = $props();

	const ICONS: Record<BlockType, typeof TextIcon> = {
		header: IdCardIcon,
		bio: UserRoundIcon,
		quick_actions: PhoneIcon,
		booking: CalendarCheckIcon,
		actions: MousePointerClickIcon,
		links: LinkIcon,
		documents: FileTextIcon,
		heading: HeadingIcon,
		text: TextIcon,
		gallery: ImagesIcon,
		event: TicketIcon,
		divider: MinusIcon
	};

	/** Where the content of a field block is edited. */
	const EDITED_IN: Partial<Record<BlockType, [string, string]>> = {
		bio: ['#profile', 'Edit your bio under About you.'],
		quick_actions: ['#contact', 'Uses your email, phone and website from Contact details.'],
		booking: ['#booking', 'Shows the booking page chosen under Booking.'],
		actions: ['#sharing', 'Save contact, Share your contact (if collecting leads, under Link & tap) and Share.'],
		links: ['#links', 'Edit your links under Links.'],
		documents: ['#brochures', 'Edit your brochures under Brochures.']
	};

	let expanded = $state<string | null>(null);

	// Field blocks appear once; free-content blocks can be added any number of times.
	const addable = $derived(
		(Object.keys(BLOCK_INFO) as BlockType[]).filter(
			(t) => t !== 'header' && !(SINGLETON_BLOCKS.includes(t) && blocks.some((b) => b.type === t))
		)
	);

	function add(type: BlockType) {
		if (blocks.length >= MAX_BLOCKS) return;
		const block = newBlock(type);
		blocks.push(block);
		if (type === 'heading' || type === 'text' || type === 'gallery' || type === 'event') expanded = block.id;
	}

	// The header stays first, so nothing moves above it.
	function move(index: number, delta: number) {
		const target = index + delta;
		if (target < 1 || target >= blocks.length) return;
		const [item] = blocks.splice(index, 1);
		blocks.splice(target, 0, item);
	}

	function hasSettings(b: CardBlock): boolean {
		return (
			b.type === 'header' ||
			b.type === 'heading' ||
			b.type === 'text' ||
			b.type === 'gallery' ||
			b.type === 'event' ||
			b.type in EDITED_IN
		);
	}

	/** A short preview of the block's content, shown on its row. */
	function summary(b: CardBlock): string {
		switch (b.type) {
			case 'header':
				return `${HEADER_STYLES[b.style]} style`;
			case 'heading':
			case 'text':
				return b.text.trim() || 'Empty';
			case 'gallery':
				return b.images.length === 1 ? '1 image' : `${b.images.length} images`;
			case 'event':
				return (
					[b.role, b.name]
						.map((s) => s.trim())
						.filter(Boolean)
						.join(' · ') || 'Empty'
				);
			default:
				return BLOCK_INFO[b.type].description;
		}
	}

	// ---- Gallery images --------------------------------------------------------
	let pickerFor = $state<BlockOf<'gallery'> | null>(null);
	let uploadFor = $state<BlockOf<'gallery'> | null>(null);
	let uploading = $state<string | null>(null);
	let uploadInput: HTMLInputElement | undefined = $state();

	function addImage(gallery: BlockOf<'gallery'>, file: LibraryFile) {
		if (gallery.images.length >= MAX_GALLERY_IMAGES) return;
		onfile(file);
		if (gallery.images.some((i) => i.file === file.id)) {
			toast.info('That image is already in this gallery');
			return;
		}
		gallery.images.push({ id: newId(), file: file.id, caption: '' });
	}

	async function uploadImages(gallery: BlockOf<'gallery'>, picked: File[]) {
		const room = MAX_GALLERY_IMAGES - gallery.images.length;
		if (picked.length > room) toast.info(`A gallery holds up to ${MAX_GALLERY_IMAGES} images`);
		uploading = gallery.id;
		try {
			for (const f of picked.slice(0, room)) {
				const problem = checkUpload(f, 'image');
				if (problem) {
					toast.error(`${f.name}: ${problem}`);
					continue;
				}
				addImage(gallery, await uploadFile(f, { purpose: 'gallery' }));
			}
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Upload failed');
		} finally {
			uploading = null;
		}
	}

	function moveImage(gallery: BlockOf<'gallery'>, index: number, delta: number) {
		const target = index + delta;
		if (target < 0 || target >= gallery.images.length) return;
		const [item] = gallery.images.splice(index, 1);
		gallery.images.splice(target, 0, item);
	}
</script>

<div class="flex flex-col gap-2">
	{#each blocks as block, i (block.id)}
		{@const Icon = ICONS[block.type]}
		{@const open = expanded === block.id}
		<div class={cn('bg-card rounded-xl border', block.hidden && 'bg-muted/40')}>
			<div class="flex items-center gap-2 p-2">
				<button
					type="button"
					class="flex min-w-0 flex-1 items-center gap-3 rounded-lg p-0.5 text-left disabled:cursor-default"
					onclick={() => (expanded = open ? null : block.id)}
					disabled={!hasSettings(block)}
					aria-expanded={hasSettings(block) ? open : undefined}
				>
					<span
						class={cn(
							'bg-muted text-muted-foreground grid size-9 shrink-0 place-items-center rounded-lg',
							block.hidden && 'opacity-50'
						)}
					>
						<Icon class="size-4" />
					</span>
					<span class={cn('flex min-w-0 flex-1 flex-col', block.hidden && 'opacity-50')}>
						<span class="text-sm font-medium">
							{BLOCK_INFO[block.type].label}
							{#if block.hidden}<span class="text-muted-foreground font-normal"> · Hidden</span>{/if}
						</span>
						<span class="text-muted-foreground truncate text-xs">{summary(block)}</span>
					</span>
					{#if hasSettings(block)}
						<ChevronDownIcon
							class={cn('text-muted-foreground size-4 shrink-0 transition-transform', open && 'rotate-180')}
						/>
					{/if}
				</button>
				{#if block.type !== 'header'}
					<div class="flex shrink-0 max-sm:flex-col">
						<Button variant="ghost" size="icon" disabled={i <= 1} onclick={() => move(i, -1)} aria-label="Move up">
							<ArrowUpIcon />
						</Button>
						<Button
							variant="ghost"
							size="icon"
							disabled={i === blocks.length - 1}
							onclick={() => move(i, 1)}
							aria-label="Move down"
						>
							<ArrowDownIcon />
						</Button>
					</div>
					<div class="flex shrink-0 max-sm:flex-col">
						<Button
							variant="ghost"
							size="icon"
							onclick={() => (block.hidden ? delete block.hidden : (block.hidden = true))}
							aria-label={block.hidden ? 'Show block' : 'Hide block'}
							title={block.hidden ? 'Show' : 'Hide'}
						>
							{#if block.hidden}<EyeOffIcon />{:else}<EyeIcon />{/if}
						</Button>
						<Button
							variant="ghost"
							size="icon"
							onclick={() => blocks.splice(i, 1)}
							aria-label="Remove block"
							title="Remove"
						>
							<XIcon />
						</Button>
					</div>
				{/if}
			</div>

			{#if open}
				<div class="border-t p-4">
					{#if block.type === 'header'}
						<Field.Field>
							<Field.Label id="header-style-{block.id}">Style</Field.Label>
							<ToggleGroup.Root
								type="single"
								variant="outline"
								value={block.style}
								onValueChange={(v) => v && (block.style = v as HeaderStyle)}
								aria-labelledby="header-style-{block.id}"
								class="w-full max-w-sm"
							>
								{#each Object.entries(HEADER_STYLES) as [value, label] (value)}
									<ToggleGroup.Item {value} class="flex-1">{label}</ToggleGroup.Item>
								{/each}
							</ToggleGroup.Root>
							<Field.Description>
								{#if block.style === 'badge'}A name badge: big name, company in capitals, no cover image.
								{:else if block.style === 'compact'}Photo beside your name, no banner.
								{:else}Your cover image (or accent colour) with your photo over it.{/if}
							</Field.Description>
						</Field.Field>
					{:else if block.type === 'heading'}
						<Input
							bind:value={block.text}
							maxlength={MAX_HEADING}
							placeholder="e.g. Selected work"
							aria-label="Heading text"
						/>
					{:else if block.type === 'text'}
						<Textarea
							bind:value={block.text}
							rows={4}
							maxlength={MAX_TEXT}
							placeholder="Write something…"
							aria-label="Text"
						/>
					{:else if block.type === 'event'}
						<div class="grid gap-4 sm:grid-cols-2">
							<Field.Field>
								<Field.Label for="event-role-{block.id}">Your role</Field.Label>
								<Input
									id="event-role-{block.id}"
									bind:value={block.role}
									maxlength={MAX_EVENT_FIELD}
									placeholder="Speaker"
								/>
								<Field.Description>Shown as a ribbon: Speaker, Sponsor, Attendee…</Field.Description>
							</Field.Field>
							<Field.Field>
								<Field.Label for="event-name-{block.id}">Event</Field.Label>
								<Input
									id="event-name-{block.id}"
									bind:value={block.name}
									maxlength={MAX_EVENT_FIELD}
									placeholder="Design Week 2026"
								/>
							</Field.Field>
							<Field.Field>
								<Field.Label for="event-dates-{block.id}">Dates</Field.Label>
								<Input
									id="event-dates-{block.id}"
									bind:value={block.dates}
									maxlength={MAX_EVENT_FIELD}
									placeholder="12–14 Nov"
								/>
							</Field.Field>
							<Field.Field>
								<Field.Label for="event-venue-{block.id}">Venue</Field.Label>
								<Input
									id="event-venue-{block.id}"
									bind:value={block.venue}
									maxlength={MAX_EVENT_FIELD}
									placeholder="ExCeL, London"
								/>
							</Field.Field>
						</div>
					{:else if block.type === 'gallery'}
						<div class="flex flex-col gap-2">
							{#each block.images as img, j (img.id)}
								<div class="flex items-center gap-2">
									<img
										src={fileUrl(img.file)}
										alt={files[img.file]?.name ?? ''}
										class="bg-muted size-12 shrink-0 rounded-lg object-cover"
									/>
									<Input
										bind:value={img.caption}
										maxlength={MAX_CAPTION}
										placeholder="Caption (optional)"
										aria-label="Image {j + 1} caption"
										class="shadow-none"
									/>
									<div class="flex shrink-0">
										<Button
											variant="ghost"
											size="icon"
											disabled={j === 0}
											onclick={() => moveImage(block, j, -1)}
											aria-label="Move image up"
										>
											<ArrowUpIcon />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											disabled={j === block.images.length - 1}
											onclick={() => moveImage(block, j, 1)}
											aria-label="Move image down"
										>
											<ArrowDownIcon />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => block.images.splice(j, 1)}
											aria-label="Remove image"
										>
											<XIcon />
										</Button>
									</div>
								</div>
							{/each}
							{#if block.images.length < MAX_GALLERY_IMAGES}
								<div class="flex flex-wrap gap-2">
									<Button
										variant="outline"
										disabled={!storage.ready || uploading !== null}
										onclick={() => {
											uploadFor = block;
											uploadInput?.click();
										}}
									>
										{#if uploading === block.id}<Spinner data-icon="inline-start" />{:else}<UploadIcon
												data-icon="inline-start"
											/>{/if}
										Upload images
									</Button>
									<Button variant="outline" disabled={!storage.ready} onclick={() => (pickerFor = block)}>
										<ImagesIcon data-icon="inline-start" />
										From library
									</Button>
								</div>
							{/if}
							<p class="text-muted-foreground text-xs">
								{#if storage.status && !storage.ready}
									<a href="/dashboard/settings?tab=storage" class="text-foreground underline underline-offset-4"
										>Connect storage</a
									>
									to add images.
								{:else}
									Up to {MAX_GALLERY_IMAGES} images. JPEG, PNG or WebP up to 5 MB each.
								{/if}
							</p>
						</div>
					{:else if EDITED_IN[block.type]}
						{@const [href, text] = EDITED_IN[block.type]!}
						<p class="text-muted-foreground text-sm">
							{text}
							<a {href} class="text-foreground ml-1 underline underline-offset-4">Go there</a>
						</p>
					{/if}
				</div>
			{/if}
		</div>
	{/each}

	<DropdownMenu.Root>
		<DropdownMenu.Trigger
			disabled={blocks.length >= MAX_BLOCKS || addable.length === 0}
			class={cn(
				buttonVariants({ variant: 'ghost' }),
				'text-muted-foreground hover:text-foreground hover:border-foreground/30 h-12 w-full gap-2 rounded-xl border border-dashed text-sm font-medium hover:bg-transparent'
			)}
		>
			<PlusIcon class="size-4" />
			{blocks.length >= MAX_BLOCKS ? `Up to ${MAX_BLOCKS} blocks` : 'Add block'}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start" class="w-72">
			<DropdownMenu.Group>
				{#each addable as type (type)}
					{@const Icon = ICONS[type]}
					<DropdownMenu.Item onSelect={() => add(type)} class="items-start">
						<Icon class="mt-0.5" />
						<span class="flex flex-col">
							<span>{BLOCK_INFO[type].label}</span>
							<span class="text-muted-foreground text-xs">{BLOCK_INFO[type].description}</span>
						</span>
					</DropdownMenu.Item>
				{/each}
			</DropdownMenu.Group>
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</div>

<input
	bind:this={uploadInput}
	type="file"
	accept={ACCEPT.image}
	multiple
	class="sr-only"
	tabindex="-1"
	onchange={(e) => {
		const picked = Array.from(e.currentTarget.files ?? []);
		e.currentTarget.value = '';
		if (uploadFor && picked.length) uploadImages(uploadFor, picked);
	}}
/>

<FilePickerDialog
	bind:open={() => pickerFor !== null, (v) => !v && (pickerFor = null)}
	purpose="gallery"
	title="Add to gallery"
	selected={pickerFor?.images.map((i) => i.file) ?? []}
	onselect={(file) => pickerFor && addImage(pickerFor, file)}
/>
