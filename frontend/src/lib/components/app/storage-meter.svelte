<script lang="ts">
	import { formatBytes } from '$lib/api/files';
	import { cn } from '$lib/utils';

	let {
		used,
		quota,
		compact = false,
		class: className
	}: {
		used: number;
		/** null is unlimited. */
		quota: number | null;
		/** Just the bar and a short label, for table cells. */
		compact?: boolean;
		class?: string;
	} = $props();

	const fraction = $derived(quota === null ? 0 : quota === 0 ? 1 : Math.min(1, used / quota));
	const tone = $derived(fraction >= 1 ? 'bg-destructive' : fraction >= 0.85 ? 'bg-amber-500' : 'bg-brand');
</script>

<div class={cn('flex flex-col gap-1.5', className)}>
	{#if quota !== null}
		<div
			class="bg-muted h-1.5 overflow-hidden rounded-full"
			role="meter"
			aria-valuemin={0}
			aria-valuemax={quota}
			aria-valuenow={Math.min(used, quota)}
			aria-label="Storage used"
		>
			<div class={cn('h-full rounded-full transition-[width]', tone)} style="width: {fraction * 100}%"></div>
		</div>
	{/if}
	<span class={cn('text-muted-foreground tabular', compact ? 'text-xs' : 'text-sm')}>
		{formatBytes(used)}
		{#if quota === null}
			used{compact ? '' : ' · no limit'}
		{:else}
			of {formatBytes(quota)}
		{/if}
	</span>
</div>
