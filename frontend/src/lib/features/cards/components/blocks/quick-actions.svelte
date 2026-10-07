<script lang="ts">
	import MailIcon from '@lucide/svelte/icons/mail';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import { safeUrl, type CardData } from '$lib/features/cards/card';
	import { e164 } from '$lib/core/phone';
	import { cn } from '$lib/utils';

	let { card, align = 'center' }: { card: CardData; align?: 'center' | 'start' } = $props();

	const quick = $derived(
		[
			card.email.trim() && { kind: 'email', label: 'Email', href: `mailto:${card.email.trim()}`, icon: MailIcon },
			card.phone_number && {
				kind: 'call',
				label: 'Call',
				href: `tel:${e164(card.phone_country_code, card.phone_number)}`,
				icon: PhoneIcon
			},
			safeUrl(card.website) && { kind: 'website', label: 'Website', href: safeUrl(card.website)!, icon: GlobeIcon }
		].filter((q) => !!q)
	);
</script>

{#if quick.length > 0}
	<div class={cn('flex gap-3', align === 'center' ? 'justify-center' : 'justify-start')}>
		{#each quick as q (q.label)}
			<a
				href={q.href}
				target={q.label === 'Website' ? '_blank' : undefined}
				rel="noopener noreferrer"
				data-track="click"
				data-track-target={q.kind}
				data-track-label={q.label}
				class="group flex w-16 flex-col items-center gap-1.5"
			>
				<span
					class="bg-muted text-foreground grid size-11 place-items-center rounded-full transition-colors group-hover:bg-(--card-accent) group-hover:text-white"
				>
					<q.icon class="size-[18px]" />
				</span>
				<span class="text-muted-foreground text-xs">{q.label}</span>
			</a>
		{/each}
	</div>
{/if}
