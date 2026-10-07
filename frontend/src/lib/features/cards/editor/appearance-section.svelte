<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import * as Field from '$lib/components/ui/field';
	import { Switch } from '$lib/components/ui/switch';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { branding } from '$lib/features/branding/store.svelte';
	import { cn } from '$lib/utils';
	import { ACCENTS, type AccentKey, type CardData } from '../card';

	let { card = $bindable() }: { card: CardData } = $props();
</script>

<FormSection panel id="appearance" title="Appearance" description="How your card looks to visitors.">
	<Field.Group class="gap-6">
		<Field.Field>
			<Field.Label>Accent colour</Field.Label>
			<div class="flex flex-wrap gap-2.5" role="radiogroup" aria-label="Accent colour">
				{#each Object.entries(ACCENTS) as [key, color] (key)}
					<button
						type="button"
						role="radio"
						aria-checked={card.accent === key}
						aria-label={key}
						title={key}
						onclick={() => card && (card.accent = key as AccentKey)}
						class={cn(
							'ring-offset-background grid size-9 place-items-center rounded-full text-white ring-offset-2 transition-shadow',
							card.accent === key ? 'ring-foreground ring-2' : 'hover:ring-border hover:ring-2'
						)}
						style="background: {color}"
					>
						{#if card.accent === key}<CheckIcon class="size-4" />{/if}
					</button>
				{/each}
			</div>
			{#if card.cover_file}
				<Field.Description>With a cover image, the accent frames your photo and the banner.</Field.Description>
			{/if}
		</Field.Field>
		<Field.Field>
			<Field.Label>Card theme</Field.Label>
			<div class="grid max-w-md grid-cols-2 gap-3" role="radiogroup" aria-label="Card theme">
				{#each ['light', 'dark'] as const as theme (theme)}
					<button
						type="button"
						role="radio"
						aria-checked={card.theme === theme}
						onclick={() => card && (card.theme = theme)}
						class={cn(
							'flex flex-col gap-2 rounded-xl border p-2 text-left text-sm font-medium transition-colors',
							card.theme === theme ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
						)}
					>
						<span
							class={cn(
								'flex h-16 flex-col overflow-hidden rounded-lg ring-1 ring-black/5',
								theme === 'dark' ? 'bg-neutral-900' : 'bg-white'
							)}
							aria-hidden="true"
						>
							<span class="h-5" style="background: {ACCENTS[card.accent]}"></span>
							<span class="flex flex-col gap-1 p-2">
								<span class={cn('h-1.5 w-12 rounded-full', theme === 'dark' ? 'bg-white/70' : 'bg-neutral-800')}></span>
								<span class={cn('h-1.5 w-8 rounded-full', theme === 'dark' ? 'bg-white/30' : 'bg-neutral-300')}></span>
							</span>
						</span>
						<span class="px-1 capitalize">{theme}</span>
					</button>
				{/each}
			</div>
		</Field.Field>
		{#if branding.value?.logo_file}
			{@const required = branding.value.logo_policy === 'required'}
			<Field.Field orientation="horizontal" class="bg-card rounded-xl border p-4">
				<Field.Content>
					<Field.Label for="org-logo">Show {branding.value.name} logo</Field.Label>
					<Field.Description>
						{required
							? `Required by ${branding.value.name} on every card.`
							: 'Shown in the corner of your card’s header.'}
					</Field.Description>
				</Field.Content>
				{#if required}
					<Switch id="org-logo" checked disabled />
				{:else}
					<Switch id="org-logo" bind:checked={card.show_org_logo} />
				{/if}
			</Field.Field>
		{/if}
	</Field.Group>
</FormSection>
