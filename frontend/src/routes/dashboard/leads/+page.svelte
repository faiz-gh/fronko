<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import LeadsTable from '$lib/components/app/leads-table.svelte';
	import { normalizeCard } from '$lib/card/card';
	import { cards } from '$lib/cards.svelte';
	import { plural } from '$lib/format';

	// The card filter lives in the URL (?card=ID) so it survives reloads and can be linked to.
	const card = $derived.by(() => {
		const id = Number(page.url.searchParams.get('card'));
		return Number.isInteger(id) && id > 0 ? id : null;
	});

	const selected = $derived(card === null ? null : (cards.list?.find((p) => p.id === card) ?? null));
	const totalLeads = $derived(cards.list?.reduce((sum, p) => sum + p.lead_count, 0) ?? null);

	function setCard(id: number | null) {
		// Same page, so keep scroll and focus where they are.
		goto(id === null ? '/dashboard/leads' : `/dashboard/leads?card=${id}`, { replace: true, reset: false });
	}
</script>

<svelte:head>
	<title>Leads · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Leads</h1>
		<p class="text-muted-foreground text-sm">
			{#if selected}
				People who shared their details from {normalizeCard(selected.data).name || selected.slug}.
			{:else if totalLeads !== null}
				{plural(totalLeads, 'person', 'people')} shared their details from your cards.
			{:else}
				People who shared their details from your cards.
			{/if}
		</p>
	</header>

	<LeadsTable
		{card}
		oncardchange={setCard}
		filename={selected ? `${selected.slug}-leads` : 'fronko-leads'}
	/>
</div>
