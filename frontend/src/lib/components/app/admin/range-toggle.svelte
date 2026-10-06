<script lang="ts">
	import * as ToggleGroup from '$lib/components/ui/toggle-group';

	let { days = $bindable(30) }: { days?: number } = $props();

	const RANGES = [
		{ days: 30, label: '30 days' },
		{ days: 90, label: '90 days' },
		{ days: 365, label: '1 year' }
	];
</script>

<ToggleGroup.Root
	type="single"
	variant="outline"
	size="sm"
	value={String(days)}
	onValueChange={(v) => v && (days = Number(v))}
	aria-label="Time range"
>
	{#each RANGES as r (r.days)}
		<ToggleGroup.Item value={String(r.days)}>{r.label}</ToggleGroup.Item>
	{/each}
</ToggleGroup.Root>
