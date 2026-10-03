<script lang="ts">
	import { cn } from '$lib/utils';

	let { class: className, href = '/' }: { class?: string; href?: string } = $props();
</script>

<a {href} class={cn('flex items-center gap-2.5 text-[15px] font-semibold tracking-tight', className)}>
	<!-- Two offset cards: the product in one glyph. -->
	<svg viewBox="0 0 28 28" class="size-7 shrink-0" aria-hidden="true">
		<rect width="28" height="28" rx="8" class="fill-brand" />
		<rect x="6" y="9" width="13" height="9" rx="2" fill="white" opacity="0.45" />
		<rect x="9" y="11.5" width="13" height="9" rx="2" fill="white" />
	</svg>
	Fronko
</a>
