<script lang="ts">
	import Share2Icon from '@lucide/svelte/icons/share-2';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import SendIcon from '@lucide/svelte/icons/send';
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils';
	import type { CardData } from '../card';

	let {
		card,
		saveHref,
		highlightSave = false,
		onOpenForm,
		onShare
	}: {
		card: CardData;
		/** The vCard download; omitted in the editor preview, where the buttons do nothing. */
		saveHref?: string;
		/** Pulses the Save button after an NFC or QR tap asked to save the contact. */
		highlightSave?: boolean;
		onOpenForm?: () => void;
		onShare?: () => void;
	} = $props();
</script>

<!-- The "Visitor buttons" block: shared by the public page and the editor preview. -->
<Button
	size="lg"
	class={cn(
		'h-11 w-full text-white transition-shadow hover:opacity-90',
		highlightSave && 'ring-offset-card animate-pulse ring-2 ring-(--card-accent) ring-offset-2'
	)}
	style="background: var(--card-accent)"
	href={saveHref}
>
	<UserPlusIcon data-icon="inline-start" />
	Save contact
</Button>
<div class="flex gap-2">
	{#if card.collect_leads}
		<Button size="lg" variant="outline" class="h-11 flex-1" onclick={onOpenForm}>
			<SendIcon data-icon="inline-start" />
			Share your contact
		</Button>
	{/if}
	<Button
		size="lg"
		variant="outline"
		class={cn('h-11', card.collect_leads ? 'px-3.5' : 'flex-1')}
		onclick={onShare}
		aria-label="Share this card"
	>
		<Share2Icon data-icon={card.collect_leads ? undefined : 'inline-start'} />
		{#if !card.collect_leads}Share{/if}
	</Button>
</div>
