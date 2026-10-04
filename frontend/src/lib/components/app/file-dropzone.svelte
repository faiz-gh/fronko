<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import { ACCEPT, checkUpload, uploadFile, type FileArea, type FileKind, type LibraryFile } from '$lib/api/files';
	import { cn } from '$lib/utils';

	let {
		kind,
		area,
		onuploaded,
		disabled = false,
		compact = false,
		multiple = true,
		class: className
	}: {
		/** Restrict uploads to one kind; otherwise images and PDFs are both accepted. */
		kind?: FileKind;
		/** Admins: the organisation's files (default) or the shared area. Members always upload to their own files. */
		area?: FileArea;
		onuploaded?: (file: LibraryFile) => void;
		disabled?: boolean;
		compact?: boolean;
		multiple?: boolean;
		class?: string;
	} = $props();

	type Item = { key: number; name: string; progress: number; error: string };

	let input: HTMLInputElement | undefined = $state();
	let dragging = $state(false);
	let items = $state<Item[]>([]);
	let nextKey = 0;

	const accept = $derived(kind ? ACCEPT[kind] : `${ACCEPT.image},${ACCEPT.pdf}`);
	const busy = $derived(items.some((i) => !i.error && i.progress < 1));
	const hint = $derived(
		kind === 'image'
			? 'JPEG, PNG or WebP, up to 5 MB'
			: kind === 'pdf'
				? 'PDF, up to 20 MB'
				: 'Images up to 5 MB, PDFs up to 20 MB'
	);

	async function handle(files: FileList | File[]) {
		const list = Array.from(files);
		if (!multiple) list.splice(1);
		// Upload one at a time so progress is meaningful and the rate limit isn't hit in a burst.
		for (const file of list) {
			const item: Item = { key: nextKey++, name: file.name, progress: 0, error: '' };
			items.push(item);
			const entry = items[items.length - 1];
			const problem = checkUpload(file, kind);
			if (problem) {
				entry.error = problem;
				continue;
			}
			try {
				const uploaded = await uploadFile(file, { area, onProgress: (p) => (entry.progress = Math.min(p, 0.99)) });
				entry.progress = 1;
				onuploaded?.(uploaded);
				// Finished rows fade from the list shortly after completing.
				setTimeout(() => (items = items.filter((i) => i.key !== entry.key)), 1200);
			} catch (e) {
				entry.error = e instanceof Error ? e.message : 'Upload failed';
				toast.error(`${file.name}: ${entry.error}`);
			}
		}
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		if (disabled || !e.dataTransfer?.files.length) return;
		handle(e.dataTransfer.files);
	}
</script>

<div class={cn('flex flex-col gap-2', className)}>
	<button
		type="button"
		{disabled}
		onclick={() => input?.click()}
		ondragenter={(e) => {
			e.preventDefault();
			if (!disabled) dragging = true;
		}}
		ondragover={(e) => e.preventDefault()}
		ondragleave={() => (dragging = false)}
		ondrop={onDrop}
		class={cn(
			'text-muted-foreground flex w-full items-center justify-center gap-3 rounded-xl border border-dashed text-sm transition-colors',
			'hover:border-foreground/30 hover:text-foreground focus-visible:ring-ring/50 outline-none focus-visible:ring-3',
			'disabled:pointer-events-none disabled:opacity-50',
			compact ? 'h-14 px-4' : 'flex-col px-6 py-10',
			dragging && 'border-brand bg-brand-soft text-foreground'
		)}
	>
		<span class={cn('bg-muted grid place-items-center rounded-full', compact ? 'size-8' : 'size-11')}>
			<UploadIcon class={compact ? 'size-4' : 'size-5'} />
		</span>
		<span class={cn('flex flex-col', compact ? 'items-start text-left' : 'items-center text-center')}>
			<span class="text-foreground font-medium">
				{busy ? 'Uploading…' : dragging ? 'Drop to upload' : 'Drop files here or click to upload'}
			</span>
			<span class="text-xs">{hint}</span>
		</span>
	</button>
	<input
		bind:this={input}
		type="file"
		class="sr-only"
		tabindex="-1"
		{accept}
		{multiple}
		onchange={(e) => {
			const files = e.currentTarget.files;
			if (files?.length) handle(files);
			e.currentTarget.value = '';
		}}
	/>

	{#if items.length > 0}
		<ul class="flex flex-col gap-1.5" aria-live="polite">
			{#each items as item (item.key)}
				<li class="bg-card flex items-center gap-3 rounded-lg border px-3 py-2 text-sm">
					{#if item.error}
						<CircleAlertIcon class="text-destructive size-4 shrink-0" />
					{/if}
					<span class="min-w-0 flex-1">
						<span class="block truncate">{item.name}</span>
						{#if item.error}
							<span class="text-destructive block text-xs">{item.error}</span>
						{:else}
							<span class="bg-muted mt-1.5 block h-1 overflow-hidden rounded-full">
								<span
									class="bg-brand block h-full rounded-full transition-[width] duration-200"
									style="width: {Math.round(item.progress * 100)}%"
								></span>
							</span>
						{/if}
					</span>
					{#if item.error}
						<button
							type="button"
							class="text-muted-foreground hover:text-foreground text-xs"
							onclick={() => (items = items.filter((i) => i.key !== item.key))}
						>
							Dismiss
						</button>
					{:else}
						<span class="text-muted-foreground tabular text-xs">{Math.round(item.progress * 100)}%</span>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>
