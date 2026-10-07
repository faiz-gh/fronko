<script lang="ts">
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import BrandIcon from '$lib/components/shared/brand-icon.svelte';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { newId, safeUrl, type CardData } from '../card';

	const MAX_LINKS = 12;

	let { card = $bindable() }: { card: CardData } = $props();

	function addLink() {
		card.links.push({ id: newId(), label: '', url: '' });
	}

	function moveLink(index: number, delta: number) {
		const target = index + delta;
		if (target < 0 || target >= card.links.length) return;
		const [item] = card.links.splice(index, 1);
		card.links.splice(target, 0, item);
	}
</script>

<FormSection
	panel
	id="links"
	title="Links"
	description="Socials, portfolio, profiles. Known sites get their icon automatically."
>
	<div class="flex flex-col gap-2">
		{#each card.links as link, i (link.id)}
			{@const linkInvalid = !!link.url.trim() && !safeUrl(link.url)}
			<div class="bg-card flex items-start gap-2 rounded-xl border p-2">
				<span class="bg-muted text-muted-foreground grid size-9 shrink-0 place-items-center rounded-lg">
					<BrandIcon url={link.url} />
				</span>
				<div class="grid flex-1 gap-2 sm:grid-cols-[2fr_3fr]">
					<Input
						bind:value={link.label}
						placeholder="Label (optional)"
						aria-label="Link {i + 1} label"
						class="shadow-none"
					/>
					<Input
						bind:value={link.url}
						placeholder="github.com/you"
						aria-label="Link {i + 1} URL"
						aria-invalid={linkInvalid || undefined}
						class="shadow-none"
					/>
				</div>
				<div class="flex shrink-0 max-sm:flex-col">
					<Button variant="ghost" size="icon" disabled={i === 0} onclick={() => moveLink(i, -1)} aria-label="Move up">
						<ArrowUpIcon />
					</Button>
					<Button
						variant="ghost"
						size="icon"
						disabled={i === card.links.length - 1}
						onclick={() => moveLink(i, 1)}
						aria-label="Move down"
					>
						<ArrowDownIcon />
					</Button>
					<Button variant="ghost" size="icon" onclick={() => card?.links.splice(i, 1)} aria-label="Remove link">
						<XIcon />
					</Button>
				</div>
			</div>
		{/each}
		<button
			type="button"
			onclick={addLink}
			disabled={card.links.length >= MAX_LINKS}
			class="text-muted-foreground hover:text-foreground hover:border-foreground/30 flex h-12 items-center justify-center gap-2 rounded-xl border border-dashed text-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50"
		>
			<PlusIcon class="size-4" />
			{card.links.length >= MAX_LINKS ? `Up to ${MAX_LINKS} links` : 'Add link'}
		</button>
	</div>
</FormSection>
