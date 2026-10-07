<script lang="ts">
	import { fileUrl, type PublicFile } from '$lib/features/files/api';
	import type { GalleryImage } from '$lib/features/cards/blocks';
	import { cn } from '$lib/utils';

	let { images, files }: { images: GalleryImage[]; files?: Record<string, PublicFile> } = $props();

	// Images whose file is gone from the library are hidden, like brochures.
	const shown = $derived(images.filter((img) => !files || files[img.file]));
</script>

{#if shown.length > 0}
	<ul class={cn('grid gap-2', shown.length === 1 ? 'grid-cols-1' : 'grid-cols-2')}>
		{#each shown as img (img.id)}
			<li class="flex flex-col gap-1.5">
				<a
					href={fileUrl(img.file)}
					target="_blank"
					rel="noopener noreferrer"
					data-track="gallery_open"
					data-track-target={img.file}
					data-track-label={img.caption.trim() || 'Image'}
					class="bg-muted block overflow-hidden rounded-xl ring-1 ring-black/5"
				>
					<img
						src={fileUrl(img.file)}
						alt={img.caption}
						loading="lazy"
						class={cn(
							'w-full object-cover transition-transform hover:scale-[1.03]',
							shown.length === 1 ? 'aspect-video' : 'aspect-square'
						)}
					/>
				</a>
				{#if img.caption.trim()}
					<p class="text-muted-foreground px-0.5 text-xs leading-snug">{img.caption}</p>
				{/if}
			</li>
		{/each}
	</ul>
{/if}
