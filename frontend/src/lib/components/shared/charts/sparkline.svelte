<script lang="ts">
	let {
		values,
		color = 'var(--viz-1)',
		height = 28,
		class: className
	}: {
		values: number[];
		color?: string;
		height?: number;
		class?: string;
	} = $props();

	let width = $state(0);

	const path = $derived.by(() => {
		if (values.length < 2 || width <= 0) return { line: '', area: '' };
		const max = Math.max(1, ...values);
		const pad = 2;
		const x = (i: number) => (i * width) / (values.length - 1);
		const y = (v: number) => pad + (height - pad * 2) * (1 - v / max);
		const line = values.map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join('');
		return { line, area: `${line}L${width},${height}L0,${height}Z` };
	});
</script>

<!-- Decorative trend; the number beside it carries the value. -->
<div class={className} bind:clientWidth={width} aria-hidden="true">
	{#if path.line}
		<svg {width} {height} class="block overflow-visible">
			<path d={path.area} fill={color} fill-opacity="0.1" />
			<path d={path.line} fill="none" stroke={color} stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
		</svg>
	{:else}
		<div style="height: {height}px"></div>
	{/if}
</div>
