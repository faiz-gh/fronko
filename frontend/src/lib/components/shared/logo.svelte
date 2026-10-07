<script lang="ts">
	import { cn } from '$lib/utils';

	type Size = 'sm' | 'md' | 'lg';

	let { class: className, href = '/', size = 'md' }: { class?: string; href?: string; size?: Size } = $props();

	const sizes: Record<Size, { link: string; mark: string }> = {
		sm: { link: 'gap-2 text-base', mark: 'h-6' },
		md: { link: 'gap-2.5 text-xl', mark: 'h-8' },
		lg: { link: 'gap-3 text-2xl', mark: 'h-9' }
	};
</script>

<a {href} class={cn('flex items-center font-semibold tracking-[-0.03em]', sizes[size].link, className)}>
	<!-- Stacked-card F: the stem follows the text colour, the arms are Fronko orange (#ea580c).
	     The viewBox is cropped to the glyph so the mark fills its height. -->
	<svg viewBox="10 4 44 56" class={cn('w-auto shrink-0', sizes[size].mark)} aria-hidden="true">
		<rect x="12" y="6" width="13" height="52" rx="4" fill="currentColor" />
		<rect x="12" y="6" width="40" height="13" rx="4" class="fill-orange-600 dark:fill-orange-400" />
		<rect x="12" y="26" width="29" height="13" rx="4" class="fill-orange-600 dark:fill-orange-400" opacity="0.55" />
	</svg>
	fronko
</a>
