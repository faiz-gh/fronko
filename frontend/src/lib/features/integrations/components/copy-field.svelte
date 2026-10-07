<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';

	/** A value to copy into another system: a callback URL, a token, a DNS record. */
	let { id, label, value, help }: { id: string; label: string; value: string; help?: string } = $props();

	async function copy() {
		try {
			await navigator.clipboard.writeText(value);
			toast.success(`${label} copied`);
		} catch {
			toast.error("Couldn't copy; select the text and copy it instead");
		}
	}
</script>

<Field.Field>
	<Field.Label for={id}>{label}</Field.Label>
	<div class="flex gap-2">
		<Input
			{id}
			{value}
			readonly
			class="font-mono text-xs"
			onfocus={(e) => (e.currentTarget as HTMLInputElement).select()}
		/>
		<Button type="button" variant="outline" size="icon" onclick={copy} aria-label="Copy {label}">
			<CopyIcon />
		</Button>
	</div>
	{#if help}<Field.Description>{help}</Field.Description>{/if}
</Field.Field>
