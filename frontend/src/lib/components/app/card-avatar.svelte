<script lang="ts">
	import * as Avatar from '$lib/components/ui/avatar';
	import { ACCENTS, avatarSrc, initials, type CardData } from '$lib/card/card';
	import { cn } from '$lib/utils';

	let { card, fallback, class: className }: { card: CardData; fallback: string; class?: string } = $props();

	const avatar = $derived(avatarSrc(card));
</script>

<Avatar.Root class={cn('size-8 text-xs', className)}>
	{#if avatar}
		<Avatar.Image src={avatar} alt="" class="object-cover" />
	{/if}
	<Avatar.Fallback class="font-semibold text-white" style="background: {ACCENTS[card.accent]}">
		{initials(card.name, fallback)}
	</Avatar.Fallback>
</Avatar.Root>
