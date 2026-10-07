<script lang="ts">
	import ArrowUpRightIcon from '@lucide/svelte/icons/arrow-up-right';
	import BrandIcon from '$lib/components/shared/brand-icon.svelte';
	import { linkLabel, safeUrl, type CardData } from '$lib/features/cards/card';

	let { card }: { card: CardData } = $props();

	const links = $derived(
		card.links.flatMap((l) => {
			const href = safeUrl(l.url);
			return href ? [{ ...l, href, text: linkLabel(l) }] : [];
		})
	);
</script>

{#if links.length > 0}
	<ul class="flex flex-col gap-2">
		{#each links as link (link.id)}
			<li>
				<a
					href={link.href}
					target="_blank"
					rel="noopener noreferrer"
					data-track="click"
					data-track-target={link.href}
					data-track-label={link.text}
					class="bg-muted/60 hover:bg-muted group flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-medium transition-colors"
				>
					<span class="text-(--card-accent)">
						<BrandIcon url={link.href} class="size-[18px]" />
					</span>
					<span class="flex-1 truncate">{link.text}</span>
					<ArrowUpRightIcon
						class="text-muted-foreground size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5"
					/>
				</a>
			</li>
		{/each}
	</ul>
{/if}
