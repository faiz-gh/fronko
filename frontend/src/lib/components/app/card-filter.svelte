<script lang="ts">
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import LayersIcon from '@lucide/svelte/icons/layers';
	import { buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import CardAvatar from './card-avatar.svelte';
	import { normalizeCard } from '$lib/card/card';
	import { cards } from '$lib/cards.svelte';
	import { session } from '$lib/session.svelte';

	let {
		value,
		onchange
	}: {
		/** Selected card id, or null for all cards. */
		value: number | null;
		onchange: (value: number | null) => void;
	} = $props();

	const options = $derived(
		(cards.list ?? []).map((p) => ({ profile: p, card: normalizeCard(p.data) }))
	);
	const selected = $derived(options.find((o) => o.profile.id === value) ?? null);
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger class={buttonVariants({ variant: 'outline', class: 'max-w-64 justify-start gap-2' })}>
		{#if selected}
			<CardAvatar card={selected.card} fallback={selected.profile.slug} class="size-5 text-[9px]" />
			<span class="truncate">{selected.card.name || selected.profile.slug}</span>
		{:else}
			<LayersIcon class="text-muted-foreground" />
			<span>All cards</span>
		{/if}
		<ChevronDownIcon class="text-muted-foreground ml-auto size-3.5" />
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="start" class="w-64">
		<DropdownMenu.RadioGroup
			value={value === null ? 'all' : String(value)}
			onValueChange={(v) => onchange(v === 'all' ? null : Number(v))}
		>
			<DropdownMenu.RadioItem value="all">
				<span class="flex flex-1 items-center justify-between gap-2">
					All cards
					<span class="text-muted-foreground tabular text-xs">
						{options.reduce((sum, o) => sum + o.profile.lead_count, 0)}
					</span>
				</span>
			</DropdownMenu.RadioItem>
			{#if options.length > 0}
				<DropdownMenu.Separator />
			{/if}
			{#each options as { profile, card } (profile.id)}
				<DropdownMenu.RadioItem value={String(profile.id)}>
					<span class="flex min-w-0 flex-1 items-center gap-2">
						<CardAvatar {card} fallback={profile.slug} class="size-5 text-[9px]" />
						<span class="flex min-w-0 flex-col">
							<span class="truncate">{card.name || profile.slug}</span>
							<span class="text-muted-foreground truncate font-mono text-[11px]">/p/{session.orgHandle}/{profile.slug}</span>
						</span>
						<span class="text-muted-foreground tabular ml-auto text-xs">{profile.lead_count}</span>
					</span>
				</DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>
