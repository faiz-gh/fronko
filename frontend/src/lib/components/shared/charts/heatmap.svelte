<script lang="ts">
	import { cn } from '$lib/utils';

	let { data, class: className }: { data: number[][]; class?: string } = $props();

	// Monday first reads more naturally for working weeks; the data is Sunday = 0.
	const order = [1, 2, 3, 4, 5, 6, 0];
	const dayNames = $derived.by(() => {
		const f = new Intl.DateTimeFormat(undefined, { weekday: 'short', timeZone: 'UTC' });
		// 2026-10-04 is a Sunday.
		return Array.from({ length: 7 }, (_, d) => f.format(new Date(Date.UTC(2026, 9, 4 + d))));
	});
	const hourLabel = (h: number) => `${String(h).padStart(2, '0')}:00`;

	const max = $derived(Math.max(0, ...data.flat()));
	const total = $derived(data.flat().reduce((a, b) => a + b, 0));
	function shade(v: number): string {
		if (v <= 0 || max === 0) return 'var(--viz-seq-0)';
		const step = Math.min(4, Math.max(1, Math.ceil((v / max) * 4)));
		return `var(--viz-seq-${step})`;
	}

	const peak = $derived.by(() => {
		let best = { d: 0, h: 0, v: 0 };
		data.forEach((row, d) => row.forEach((v, h) => v > best.v && (best = { d, h, v })));
		return best;
	});
	let hover = $state<{ d: number; h: number } | null>(null);
</script>

<div class={cn('flex flex-col gap-2', className)}>
	<p class="text-muted-foreground min-h-4 text-xs">
		{#if hover}
			{dayNames[hover.d]}
			{hourLabel(hover.h)}: <span class="text-foreground font-medium">{data[hover.d]?.[hover.h] ?? 0}</span> views
		{:else if total > 0}
			Busiest: {dayNames[peak.d]} around {hourLabel(peak.h)} ({peak.v} views)
		{:else}
			No views in this period
		{/if}
	</p>
	<div class="overflow-x-auto">
		<table
			class="w-full min-w-[480px] table-fixed border-separate"
			style="border-spacing: 2px"
			aria-label="Views by weekday and hour"
		>
			<colgroup
				><col class="w-10" />{#each Array.from({ length: 24 }, (_, h) => h) as h (h)}<col />{/each}</colgroup
			>
			<tbody>
				{#each order as d (d)}
					<tr>
						<th scope="row" class="text-muted-foreground pr-1 text-left text-[11px] font-normal">{dayNames[d]}</th>
						{#each data[d] ?? [] as v, h (h)}
							<td
								class="h-4 rounded-[3px]"
								style="background: {shade(v)}"
								title="{dayNames[d]} {hourLabel(h)}: {v} views"
								onpointerenter={() => (hover = { d, h })}
								onpointerleave={() => (hover = null)}><span class="sr-only">{v}</span></td
							>
						{/each}
					</tr>
				{/each}
				<tr>
					<td></td>
					{#each Array.from({ length: 24 }, (_, h) => h) as h (h)}
						<td class="text-muted-foreground overflow-visible pt-1 text-left text-[10px] whitespace-nowrap"
							>{h % 6 === 0 ? hourLabel(h) : ''}</td
						>
					{/each}
				</tr>
			</tbody>
		</table>
	</div>
	<div class="text-muted-foreground flex items-center justify-end gap-1 text-[11px]" aria-hidden="true">
		Fewer
		{#each [0, 1, 2, 3, 4] as s (s)}<span
				class="inline-block size-3 rounded-[3px]"
				style="background: var(--viz-seq-{s})"
			></span>{/each}
		More
	</div>
</div>
