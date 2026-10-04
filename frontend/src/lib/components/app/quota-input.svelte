<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import { Switch } from '$lib/components/ui/switch';
	import { Label } from '$lib/components/ui/label';

	const MB = 1024 * 1024;

	/** Bytes, or null for unlimited. Edited in megabytes. */
	let {
		value = $bindable(),
		id,
		disabled = false
	}: { value: number | null; id: string; disabled?: boolean } = $props();

	// Remember the last limit so switching "Unlimited" off brings it back.
	let lastLimit = $state(value ?? 500 * MB);
	const megabytes = $derived(value === null ? '' : String(Math.round((value / MB) * 10) / 10));
</script>

<div class="flex flex-wrap items-center gap-3">
	<div class="flex items-stretch">
		<Input
			{id}
			type="number"
			inputmode="decimal"
			min="0"
			step="any"
			class="w-28 rounded-r-none"
			value={megabytes}
			disabled={disabled || value === null}
			oninput={(e) => {
				const n = Number(e.currentTarget.value);
				if (e.currentTarget.value !== '' && Number.isFinite(n) && n >= 0) {
					value = Math.round(n * MB);
					lastLimit = value;
				}
			}}
		/>
		<span class="text-muted-foreground bg-muted flex items-center rounded-r-lg border border-l-0 px-3 text-xs">MB</span>
	</div>
	<div class="flex items-center gap-2">
		<Switch
			id="{id}-unlimited"
			checked={value === null}
			{disabled}
			onCheckedChange={(unlimited) => (value = unlimited ? null : lastLimit)}
		/>
		<Label for="{id}-unlimited" class="font-normal">No limit</Label>
	</div>
</div>
