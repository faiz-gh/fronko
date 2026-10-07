<script lang="ts">
	import { cn } from '$lib/utils';
	import { initialsOf, logoFor } from '../registry';

	let { id, name, class: className = 'size-10' }: { id: string; name: string; class?: string } = $props();

	const logo = $derived(logoFor(id));
</script>

<span
	class={cn('grid shrink-0 place-items-center rounded-lg ring-1 ring-black/5 dark:ring-white/10', className)}
	style="background: color-mix(in oklab, {logo.color} 12%, transparent); color: {logo.color}"
	aria-hidden="true"
>
	{#if logo.path}
		<svg viewBox="0 0 24 24" fill="currentColor" class="size-1/2"><path d={logo.path} /></svg>
	{:else if logo.icon}
		<logo.icon class="size-1/2" />
	{:else}
		<span class="text-[0.8em] font-semibold tracking-tight">{initialsOf(name)}</span>
	{/if}
</span>
