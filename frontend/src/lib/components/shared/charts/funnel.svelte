<script lang="ts">
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { cn } from '$lib/utils';

	type Step = { label: string; value: number; hint?: string };

	let { steps, class: className }: { steps: Step[]; class?: string } = $props();

	const top = $derived(Math.max(1, steps[0]?.value ?? 0));
	const pct = (a: number, b: number) => (b > 0 ? `${Math.round((a / b) * 100)}%` : '–');
	// An ordinal one-hue ramp: later stages are darker.
	const shades = ['var(--viz-seq-2)', 'var(--viz-seq-3)', 'var(--viz-seq-4)', 'var(--viz-seq-4)'];
</script>

<ol class={cn('flex flex-col gap-3', className)} aria-label="Conversion funnel">
	{#each steps as step, i (step.label)}
		<li class="flex flex-col gap-1.5">
			<div class="flex items-baseline gap-2 text-sm">
				<span class="flex-1">{step.label}</span>
				{#if i > 0}
					<span class="text-muted-foreground inline-flex items-center text-xs tabular">
						<ChevronRightIcon class="size-3" aria-hidden="true" />{pct(step.value, steps[i - 1].value)} of previous
					</span>
				{/if}
				<span class="font-medium tabular">{step.value.toLocaleString()}</span>
			</div>
			<div class="bg-muted h-5 w-full overflow-hidden rounded-md">
				<div
					class="h-full rounded-md"
					style="width: {Math.max(step.value > 0 ? 1 : 0, (step.value / top) * 100)}%; background: {shades[
						Math.min(i, shades.length - 1)
					]}"
				></div>
			</div>
			{#if step.hint}<p class="text-muted-foreground text-xs">{step.hint}</p>{/if}
		</li>
	{/each}
</ol>
