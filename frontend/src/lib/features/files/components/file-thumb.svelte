<script lang="ts">
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import ImageOffIcon from '@lucide/svelte/icons/image-off';
	import { fileUrl, type FileKind } from '$lib/features/files/api';
	import { cn } from '$lib/utils';

	let {
		id,
		kind,
		hasThumb = false,
		fit = 'cover',
		full = false,
		class: className
	}: {
		id: string;
		kind: FileKind;
		/** The file has a small preview: for PDFs, their first page. */
		hasThumb?: boolean;
		/** Logos and banners are shown whole (contain); photos fill the tile (cover). */
		fit?: 'cover' | 'contain';
		/** Show the image itself rather than its preview (large views). */
		full?: boolean;
		class?: string;
	} = $props();

	let failed = $state(false);
	// Images always ask for the preview; the server falls back to the image when there isn't one.
	const src = $derived(kind === 'image' ? fileUrl(id, { thumb: !full }) : hasThumb ? fileUrl(id, { thumb: true }) : '');
	$effect(() => {
		void src;
		failed = false;
	});
</script>

<div
	class={cn(
		'bg-muted text-muted-foreground relative grid place-items-center overflow-hidden',
		fit === 'contain' && kind === 'image' && 'checkerboard',
		className
	)}
>
	{#if src && !failed}
		<img
			{src}
			alt=""
			loading="lazy"
			decoding="async"
			class={cn(
				'absolute inset-0 size-full',
				kind === 'pdf'
					? 'bg-white object-cover object-top'
					: fit === 'contain'
						? 'object-contain p-[8%]'
						: 'object-cover'
			)}
			onerror={() => (failed = true)}
		/>
	{:else if kind === 'image'}
		<ImageOffIcon class="size-6" aria-label="Image unavailable" />
	{:else}
		<span class="flex flex-col items-center gap-1">
			<FileTextIcon class="size-7" />
			<span class="text-[10px] font-semibold tracking-wider">PDF</span>
		</span>
	{/if}
</div>

<style>
	/* A light checkerboard behind logos, so transparency and white logos stay visible. */
	.checkerboard {
		background-color: var(--muted);
		background-image:
			linear-gradient(45deg, color-mix(in oklab, var(--foreground) 6%, transparent) 25%, transparent 25%),
			linear-gradient(-45deg, color-mix(in oklab, var(--foreground) 6%, transparent) 25%, transparent 25%),
			linear-gradient(45deg, transparent 75%, color-mix(in oklab, var(--foreground) 6%, transparent) 75%),
			linear-gradient(-45deg, transparent 75%, color-mix(in oklab, var(--foreground) 6%, transparent) 75%);
		background-size: 16px 16px;
		background-position:
			0 0,
			0 8px,
			8px -8px,
			-8px 0;
	}
</style>
