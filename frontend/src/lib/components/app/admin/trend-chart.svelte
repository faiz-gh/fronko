<script lang="ts">
	import { cn } from '$lib/utils';

	type Point = { date: string; value: number };

	let {
		label,
		points,
		format = (n: number) => n.toLocaleString(),
		class: className
	}: {
		/** Names the single series, so the chart needs no legend. */
		label: string;
		/** Oldest first, one per day. */
		points: Point[];
		format?: (n: number) => string;
		class?: string;
	} = $props();

	const HEIGHT = 140;
	const M = { top: 10, right: 10, bottom: 22, left: 52 };

	let width = $state(0);
	let hover = $state<number | null>(null);

	const dayFormat = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
	const fullFormat = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeZone: 'UTC' });
	const day = (iso: string) => dayFormat.format(new Date(iso + 'T00:00:00Z'));
	const fullDay = (iso: string) => fullFormat.format(new Date(iso + 'T00:00:00Z'));

	/** The smallest of 1, 2, 2.5, 5 × 10ⁿ at or above n, so the top tick is a round number. */
	function niceMax(n: number): number {
		if (n <= 0) return 1;
		const step = 10 ** Math.floor(Math.log10(n));
		return [1, 2, 2.5, 5, 10].map((m) => m * step).find((v) => v >= n) ?? 10 * step;
	}

	const innerW = $derived(Math.max(0, width - M.left - M.right));
	const innerH = HEIGHT - M.top - M.bottom;
	// Counts never sit between whole numbers, so keep the middle tick whole too.
	const max = $derived(niceMax(Math.max(2, ...points.map((p) => p.value))));
	const ticks = $derived([0, max / 2, max]);

	const x = (i: number) => M.left + (points.length === 1 ? innerW / 2 : (i * innerW) / (points.length - 1));
	const y = (v: number) => M.top + innerH - (v / max) * innerH;

	const line = $derived(points.map((p, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(p.value).toFixed(1)}`).join(''));
	const area = $derived(
		points.length > 1 ? `${line}L${x(points.length - 1).toFixed(1)},${y(0)}L${x(0).toFixed(1)},${y(0)}Z` : ''
	);

	const last = $derived(points.at(-1));
	const change = $derived(points.length > 1 ? points[points.length - 1].value - points[0].value : null);
	const active = $derived(hover ?? (points.length ? points.length - 1 : null));

	// The crosshair snaps to the nearest day, so the reader aims at a date, not at the 2px line.
	function onmove(e: PointerEvent) {
		if (!points.length || innerW <= 0) return;
		const rect = (e.currentTarget as SVGElement).getBoundingClientRect();
		const px = e.clientX - rect.left - M.left;
		const i = points.length === 1 ? 0 : Math.round((px / innerW) * (points.length - 1));
		hover = Math.max(0, Math.min(points.length - 1, i));
	}

	const tooltipLeft = $derived(hover === null ? 0 : Math.min(Math.max(x(hover), 70), Math.max(70, width - 70)));
</script>

<figure class={cn('bg-card flex flex-col gap-3 rounded-xl border p-4 sm:p-5', className)}>
	<figcaption class="flex flex-col gap-0.5">
		<span class="text-muted-foreground text-sm">{label}</span>
		<span class="text-2xl font-semibold tracking-tight">
			{last ? format(last.value) : '–'}
		</span>
		<span class="text-muted-foreground text-xs">
			{#if change !== null}
				{change === 0 ? 'No change' : `${change > 0 ? '+' : '−'}${format(Math.abs(change))}`} since {day(points[0].date)}
			{:else if last}
				First day of data: {fullDay(last.date)}
			{:else}
				No data yet
			{/if}
		</span>
	</figcaption>

	<div class="relative" bind:clientWidth={width}>
		{#if width > 0 && points.length}
			<svg
				{width}
				height={HEIGHT}
				class="block touch-pan-y overflow-visible"
				role="img"
				aria-label="{label}, {points.length} days, latest {last ? format(last.value) : 'none'}"
				onpointermove={onmove}
				onpointerdown={onmove}
				onpointerleave={() => (hover = null)}
			>
				{#each ticks as t (t)}
					<line x1={M.left} x2={width - M.right} y1={y(t)} y2={y(t)} class="stroke-border" stroke-width="1" />
					<text x={M.left - 8} y={y(t)} dy="0.32em" text-anchor="end" class="fill-muted-foreground tabular text-[11px]">
						{format(t)}
					</text>
				{/each}
				<text x={x(0)} y={HEIGHT - 4} text-anchor={points.length === 1 ? 'middle' : 'start'} class="fill-muted-foreground text-[11px]">
					{day(points[0].date)}
				</text>
				{#if points.length > 1}
					<text x={x(points.length - 1)} y={HEIGHT - 4} text-anchor="end" class="fill-muted-foreground text-[11px]">
						{day(points[points.length - 1].date)}
					</text>
					<path d={area} class="fill-brand" fill-opacity="0.1" />
					<path d={line} fill="none" class="stroke-brand" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
				{/if}
				{#if hover !== null}
					<line x1={x(hover)} x2={x(hover)} y1={M.top} y2={M.top + innerH} class="stroke-muted-foreground/40" stroke-width="1" />
				{/if}
				{#if active !== null}
					<circle cx={x(active)} cy={y(points[active].value)} r="4" class="fill-brand stroke-card" stroke-width="2" />
				{/if}
			</svg>

			{#if hover !== null}
				<div
					class="bg-popover text-popover-foreground pointer-events-none absolute top-0 z-10 -translate-x-1/2 rounded-md border px-2.5 py-1.5 text-xs shadow-md"
					style="left: {tooltipLeft}px"
				>
					<div class="text-muted-foreground">{fullDay(points[hover].date)}</div>
					<div class="flex items-center gap-1.5 font-medium">
						<span class="bg-brand inline-block h-0.5 w-3 rounded-full" aria-hidden="true"></span>
						<span class="tabular">{format(points[hover].value)}</span>
					</div>
				</div>
			{/if}
		{:else if width > 0}
			<div class="text-muted-foreground grid place-items-center text-sm" style="height: {HEIGHT}px">No data yet</div>
		{/if}
	</div>

	<!-- The same numbers as a table, for screen readers. -->
	<table class="sr-only">
		<caption>{label} by day</caption>
		<thead><tr><th scope="col">Day</th><th scope="col">{label}</th></tr></thead>
		<tbody>
			{#each points as p (p.date)}
				<tr><td>{fullDay(p.date)}</td><td>{format(p.value)}</td></tr>
			{/each}
		</tbody>
	</table>
</figure>
