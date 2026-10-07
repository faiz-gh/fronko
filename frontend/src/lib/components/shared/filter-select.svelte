<script lang="ts">
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import type { Component } from 'svelte';
	import { buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { cn } from '$lib/utils';

	type Option = { value: number; label: string };

	/** A dropdown filter over numeric ids; null is "all". */
	let {
		value,
		options,
		onchange,
		allLabel,
		icon: Icon,
		size = 'default',
		class: className
	}: {
		value: number | null;
		options: Option[];
		onchange: (value: number | null) => void;
		allLabel: string;
		icon?: Component<{ class?: string }>;
		size?: 'default' | 'sm';
		class?: string;
	} = $props();

	const selected = $derived(options.find((o) => o.value === value));
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger class={buttonVariants({ variant: 'outline', size, class: cn('max-w-56 justify-start gap-2', className) })}>
		{#if Icon}<Icon class="text-muted-foreground" />{/if}
		<span class="truncate">{selected?.label ?? allLabel}</span>
		<ChevronDownIcon class="text-muted-foreground ml-auto size-3.5" />
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="start" class="max-h-80 w-60">
		<DropdownMenu.RadioGroup
			value={value === null ? 'all' : String(value)}
			onValueChange={(v) => onchange(v === 'all' ? null : Number(v))}
		>
			<DropdownMenu.RadioItem value="all">{allLabel}</DropdownMenu.RadioItem>
			{#if options.length > 0}
				<DropdownMenu.Separator />
			{/if}
			{#each options as o (o.value)}
				<DropdownMenu.RadioItem value={String(o.value)}><span class="truncate">{o.label}</span></DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>
