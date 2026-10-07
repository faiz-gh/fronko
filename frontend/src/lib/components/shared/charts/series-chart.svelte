<script lang="ts">
	import { cn } from '$lib/utils';

	type Series = { key: string; label: string; color: string; values: number[] };

	let {
		dates,
		series,
		label,
		height = 220,
		format = (n: number) => n.toLocaleString(),
		class: className
	}: {
		/** ISO days (YYYY-MM-DD), oldest first; one value per day in each series. */
		dates: string[];
		/** Two to four series share one axis; colours follow the series, never the rank. */
		series: Series[];
		/** What the chart shows, for the accessible name and table caption. */
		label: string;
		height?: number;
		format?: (n: number) => string;
		class?: string;
	} = $props();

	const M = { top: 12, right: 12, bottom: 24, left: 44 };

	let width = $state(0);
	let hover = $state<number | null>(null);
	let hidden = $state<Record<string, boolean>>({});

	const shown = $derived(series.filter((s) => !hidden[s.key]));
	const dayFormat = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
	const fullFormat = new Intl.DateTimeFormat(undefined, {
		weekday: 'short',
		month: 'short',
		day: 'numeric',
		timeZone: 'UTC'
	});
	const day = (iso: string) => dayFormat.format(new Date(iso + 'T00:00:00Z'));
	const fullDay = (iso: string) => fullFormat.format(new Date(iso + 'T00:00:00Z'));

	function niceMax(n: number): number {
		if (n <= 0) return 4;
		const step = 10 ** Math.floor(Math.log10(n));
		return [1, 2, 2.5, 5, 10].map((m) => m * step).find((v) => v >= n) ?? 10 * step;
	}

	const innerW = $derived(Math.max(0, width - M.left - M.right));
	const innerH = $derived(height - M.top - M.bottom);
	const max = $derived(niceMax(Math.max(4, ...shown.flatMap((s) => s.values))));
	const ticks = $derived([0, max / 4, max / 2, (max * 3) / 4, max].filter((t) => Number.isInteger(t)));

	const x = (i: number) => M.left + (dates.length <= 1 ? innerW / 2 : (i * innerW) / (dates.length - 1));
	const y = (v: number) => M.top + innerH - (v / max) * innerH;
	const linePath = (values: number[]) =>
		values.map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join('');

	// Date labels a whole number of days apart and at least LABEL_GAP px apart,
	// always including both ends. Rounding an uneven spacing could put two
	// neighbouring days side by side, so the step is fixed and the label just
	// before the last one is dropped when it would crowd it.
	const LABEL_GAP = 70;
	const labelIdx = $derived.by(() => {
		const n = dates.length;
		if (n <= 1) return n ? [0] : [];
		const perDay = innerW / (n - 1);
		const step = Math.max(1, Math.ceil(LABEL_GAP / Math.max(perDay, 1)));
		const idx: number[] = [];
		for (let i = 0; i < n - 1; i += step) idx.push(i);
		while (idx.length > 1 && n - 1 - idx[idx.length - 1] < step) idx.pop();
		idx.push(n - 1);
		return idx;
	});

	function onmove(e: PointerEvent) {
		if (!dates.length || innerW <= 0) return;
		const rect = (e.currentTarget as SVGElement).getBoundingClientRect();
		const px = e.clientX - rect.left - M.left;
		const i = dates.length === 1 ? 0 : Math.round((px / innerW) * (dates.length - 1));
		hover = Math.max(0, Math.min(dates.length - 1, i));
	}

	const tooltipLeft = $derived(hover === null ? 0 : Math.min(Math.max(x(hover), 80), Math.max(80, width - 80)));
	const total = (s: Series) => s.values.reduce((a, b) => a + b, 0);
</script>

<figure class={cn('flex flex-col gap-3', className)}>
	<!-- The legend doubles as a toggle; it is the identity channel, so it's always shown. -->
	<ul class="flex flex-wrap gap-x-4 gap-y-1.5 text-sm" aria-label="Series">
		{#each series as s (s.key)}
			<li>
				<button
					type="button"
					class={cn('flex items-center gap-1.5 rounded-sm transition-opacity', hidden[s.key] && 'opacity-40')}
					aria-pressed={!hidden[s.key]}
					onclick={() => (hidden = { ...hidden, [s.key]: !hidden[s.key] })}
				>
					<span class="inline-block h-0.5 w-3.5 rounded-full" style="background: {s.color}" aria-hidden="true"></span>
					<span class="text-muted-foreground">{s.label}</span>
					<span class="font-medium tabular">{format(total(s))}</span>
				</button>
			</li>
		{/each}
	</ul>

	<div class="relative" bind:clientWidth={width}>
		{#if width > 0 && dates.length}
			<svg
				{width}
				{height}
				class="block touch-pan-y overflow-visible"
				role="img"
				aria-label="{label}, {dates.length} days"
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
				{#each labelIdx as i (i)}
					<text
						x={x(i)}
						y={height - 4}
						text-anchor={dates.length === 1 ? 'middle' : i === 0 ? 'start' : i === dates.length - 1 ? 'end' : 'middle'}
						class="fill-muted-foreground text-[11px]"
					>
						{day(dates[i])}
					</text>
				{/each}
				{#if hover !== null}
					<line
						x1={x(hover)}
						x2={x(hover)}
						y1={M.top}
						y2={M.top + innerH}
						class="stroke-muted-foreground/40"
						stroke-width="1"
					/>
				{/if}
				{#each shown as s (s.key)}
					{#if dates.length > 1}
						<path
							d={linePath(s.values)}
							fill="none"
							stroke={s.color}
							stroke-width="2"
							stroke-linejoin="round"
							stroke-linecap="round"
						/>
					{/if}
					{#if hover !== null || dates.length === 1}
						{@const i = hover ?? 0}
						<circle cx={x(i)} cy={y(s.values[i] ?? 0)} r="4" fill={s.color} class="stroke-card" stroke-width="2" />
					{/if}
				{/each}
			</svg>

			{#if hover !== null}
				<div
					class="bg-popover text-popover-foreground pointer-events-none absolute top-0 z-10 min-w-36 -translate-x-1/2 rounded-md border px-2.5 py-1.5 text-xs shadow-md"
					style="left: {tooltipLeft}px"
				>
					<div class="text-muted-foreground mb-1">{fullDay(dates[hover])}</div>
					{#each shown as s (s.key)}
						<div class="flex items-center gap-1.5">
							<span class="inline-block h-0.5 w-3 rounded-full" style="background: {s.color}" aria-hidden="true"></span>
							<span class="text-muted-foreground flex-1">{s.label}</span>
							<span class="font-medium tabular">{format(s.values[hover] ?? 0)}</span>
						</div>
					{/each}
				</div>
			{/if}
		{/if}
	</div>

	<table class="sr-only">
		<caption>{label} by day</caption>
		<thead>
			<tr>
				<th scope="col">Day</th>
				{#each series as s (s.key)}<th scope="col">{s.label}</th>{/each}
			</tr>
		</thead>
		<tbody>
			{#each dates as d, i (d)}
				<tr>
					<td>{fullDay(d)}</td>
					{#each series as s (s.key)}<td>{s.values[i] ?? 0}</td>{/each}
				</tr>
			{/each}
		</tbody>
	</table>
</figure>
