<script lang="ts">
	import { cn } from '$lib/utils';

	let { class: className, href = '/' }: { class?: string; href?: string } = $props();
</script>

<a {href} class={cn('flex items-center gap-2 text-[17px] font-semibold tracking-tight', className)}>
	<!-- Stacked-card F: the stem follows the text colour, the arms are Fronko orange (#ea580c). -->
	<svg viewBox="0 0 64 64" class="size-7 shrink-0" aria-hidden="true">
		<rect x="12" y="6" width="13" height="52" rx="4" fill="currentColor" />
		<rect x="12" y="6" width="40" height="13" rx="4" class="fill-orange-600 dark:fill-orange-400" />
		<rect x="12" y="26" width="29" height="13" rx="4" class="fill-orange-600 dark:fill-orange-400" opacity="0.55" />
	</svg>
	fronko
</a>
