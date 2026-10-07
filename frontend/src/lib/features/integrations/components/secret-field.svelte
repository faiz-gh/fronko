<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import WandSparklesIcon from '@lucide/svelte/icons/wand-sparkles';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { generateSecret } from '../fields';
	import type { Field } from '../types';

	let {
		field,
		id,
		value = $bindable(),
		saved = false,
		invalid = false,
		disabled = false
	}: {
		field: Field;
		id: string;
		value: string | undefined;
		saved?: boolean;
		invalid?: boolean;
		disabled?: boolean;
	} = $props();

	// A generated value is shown so it can be copied; a typed one stays hidden.
	let generated = $state(false);

	function generate() {
		value = generateSecret();
		generated = true;
	}

	async function copy() {
		try {
			await navigator.clipboard.writeText(value ?? '');
			toast.success('Copied');
		} catch {
			toast.error("Couldn't copy; select the text and copy it instead");
		}
	}
</script>

<div class="flex gap-2">
	<Input
		{id}
		type={generated ? 'text' : 'password'}
		bind:value
		oninput={() => (generated = false)}
		placeholder={saved ? 'Saved · leave blank to keep' : (field.placeholder ?? '')}
		autocomplete="new-password"
		spellcheck={false}
		aria-invalid={invalid || undefined}
		{disabled}
		class="font-mono text-sm"
	/>
	{#if generated && value}
		<Button type="button" variant="outline" size="icon" onclick={copy} aria-label="Copy {field.label.toLowerCase()}">
			<CopyIcon />
		</Button>
	{:else if field.generate}
		<Button type="button" variant="outline" onclick={generate} {disabled}>
			<WandSparklesIcon data-icon="inline-start" />
			Generate
		</Button>
	{/if}
</div>
