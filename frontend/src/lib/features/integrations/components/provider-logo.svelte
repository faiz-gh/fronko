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
	{:else if logo.mark === 'microsoft'}
		<svg viewBox="0 0 24 24" class="size-1/2">
			<rect x="1" y="1" width="10.5" height="10.5" fill="#f25022" />
			<rect x="12.5" y="1" width="10.5" height="10.5" fill="#7fba00" />
			<rect x="1" y="12.5" width="10.5" height="10.5" fill="#00a4ef" />
			<rect x="12.5" y="12.5" width="10.5" height="10.5" fill="#ffb900" />
		</svg>
	{:else}
		<span class="text-[0.8em] font-semibold tracking-tight">{logo.initials ?? initialsOf(name)}</span>
	{/if}
</span>
