<script lang="ts">
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import * as Avatar from '$lib/components/ui/avatar';
	import { avatarSrc, coverSrc, initials, type CardData } from '$lib/card/card';
	import type { HeaderStyle } from '$lib/card/blocks';
	import { cn } from '$lib/utils';

	let {
		card,
		slug,
		style,
		logo = null
	}: {
		card: CardData;
		slug: string;
		style: HeaderStyle;
		/** The organisation's logo, when the card shows it. */
		logo?: { src: string; name: string } | null;
	} = $props();

	const name = $derived(card.name.trim() || slug);
	const subtitle = $derived([card.title, card.company].map((s) => s.trim()).filter(Boolean).join(' · '));
	const avatar = $derived(avatarSrc(card));
	const cover = $derived(coverSrc(card));
	const location = $derived(card.location.trim());
</script>

{#snippet photo(size: string, emblemSize: string)}
	{@const ring = cover && style === 'banner' ? 'ring-(--card-accent)' : 'ring-card'}
	<span class="relative inline-flex shrink-0">
		<Avatar.Root class={cn(size, 'ring-4', ring, cover && style === 'banner' && 'bg-card')}>
			{#if avatar}
				<Avatar.Image src={avatar} alt={name} class="object-cover" />
			{/if}
			<Avatar.Fallback class="bg-(--card-accent) font-semibold text-white">
				{initials(card.name, slug)}
			</Avatar.Fallback>
		</Avatar.Root>
		{@render emblem(emblemSize, ring)}
	</span>
{/snippet}

<!--
	The organisation's logo as an emblem on the photo's lower right. It wears the
	same ring as the photo, so the two rings merge into a bump on the border.
	Logos come in any colour, so they sit on white.
-->
{#snippet emblem(size: string, ring: string)}
	{#if logo}
		<span class={cn('absolute right-0 bottom-0 overflow-hidden rounded-full bg-white ring-4', size, ring)}>
			<img src={logo.src} alt={logo.name} class="size-full object-cover" />
		</span>
	{/if}
{/snippet}

{#snippet place()}
	{#if location}
		<p class="text-muted-foreground flex items-center gap-1 text-xs @lg:text-sm">
			<MapPinIcon class="size-3.5 shrink-0" />
			{location}
		</p>
	{/if}
{/snippet}

{#if style === 'badge'}
	<!-- A conference name badge: lanyard slot, big name, company in caps. -->
	<div
		class="relative flex flex-col items-center gap-4 bg-(--card-accent) px-6 pt-6 pb-8 text-center text-white"
		style="background-image: radial-gradient(120% 140% at 100% 0%, oklch(1 0 0 / 0.22), transparent 55%)"
	>
		<span class="h-2.5 w-16 rounded-full bg-black/25 ring-1 ring-white/20" aria-hidden="true"></span>
		<span class="relative inline-flex">
			<Avatar.Root class="size-28 text-3xl ring-4 ring-white/90 @lg:size-32">
				{#if avatar}
					<Avatar.Image src={avatar} alt={name} class="object-cover" />
				{/if}
				<Avatar.Fallback class="bg-white/15 font-semibold text-white">{initials(card.name, slug)}</Avatar.Fallback>
			</Avatar.Root>
			{@render emblem('size-10 @lg:size-11', 'ring-white/90')}
		</span>
		<div class="flex flex-col gap-1.5">
			<h1 class="text-3xl leading-tight font-bold tracking-tight text-balance @lg:text-4xl">{name}</h1>
			{#if card.title.trim()}
				<p class="text-base font-medium text-white/85">{card.title}</p>
			{/if}
			{#if card.company.trim()}
				<p class="text-xs font-semibold tracking-[0.18em] text-white/70 uppercase">{card.company}</p>
			{/if}
		</div>
	</div>
	{#if location}
		<div class="flex justify-center px-6 pt-5">{@render place()}</div>
	{/if}
{:else if style === 'compact'}
	<div class="flex items-center gap-4 px-6 pt-6">
		{@render photo('size-16 text-lg', 'size-6')}
		<div class="flex min-w-0 flex-col gap-0.5">
			<h1 class="text-xl font-semibold tracking-tight text-balance">{name}</h1>
			{#if subtitle}
				<p class="text-muted-foreground text-sm font-medium">{subtitle}</p>
			{/if}
			{@render place()}
		</div>
	</div>
{:else}
	{#if cover}
		<!-- With a cover photo, the accent becomes a frame: a stripe under the banner and a ring around the avatar. -->
		<div class="bg-muted aspect-3/1 border-b-4 border-(--card-accent)">
			<img src={cover} alt="" class="size-full object-cover" />
		</div>
	{:else}
		<div
			class="h-28 bg-(--card-accent) @lg:h-36"
			style="background-image: radial-gradient(120% 140% at 100% 0%, oklch(1 0 0 / 0.28), transparent 55%), radial-gradient(90% 120% at 0% 100%, oklch(0 0 0 / 0.22), transparent 60%)"
		></div>
	{/if}
	<div class="-mt-12 flex flex-col items-center gap-3 px-6 text-center @lg:-mt-14">
		{@render photo('size-24 text-2xl @lg:size-28', 'size-9 @lg:size-10')}
		<div class="flex flex-col items-center gap-1">
			<h1 class="text-2xl font-semibold tracking-tight text-balance @lg:text-3xl">{name}</h1>
			{#if subtitle}
				<p class="text-muted-foreground text-sm font-medium @lg:text-base">{subtitle}</p>
			{/if}
			{@render place()}
		</div>
	</div>
{/if}
