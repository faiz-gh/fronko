<script lang="ts">
	import BookOpenIcon from '@lucide/svelte/icons/book-open';
	import CircleUserIcon from '@lucide/svelte/icons/circle-user';
	import FileIcon from '@lucide/svelte/icons/file';
	import GalleryHorizontalIcon from '@lucide/svelte/icons/gallery-horizontal';
	import ImagesIcon from '@lucide/svelte/icons/images';
	import PanoramaIcon from '@lucide/svelte/icons/rectangle-horizontal';
	import ShapesIcon from '@lucide/svelte/icons/shapes';
	import type { FilePurpose } from '$lib/api/files';

	let { purpose, class: className }: { purpose: FilePurpose | 'all'; class?: string } = $props();

	const icons = {
		all: ImagesIcon,
		logo: ShapesIcon,
		banner: PanoramaIcon,
		avatar: CircleUserIcon,
		cover: GalleryHorizontalIcon,
		gallery: ImagesIcon,
		brochure: BookOpenIcon,
		other: FileIcon
	};
	const Icon = $derived(icons[purpose]);
</script>

<Icon class={className} aria-hidden="true" />
