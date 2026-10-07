<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import LeadsTable from '$lib/features/leads/components/leads-table.svelte';
	import { normalizeCard } from '$lib/features/cards/card';
	import { cards } from '$lib/features/cards/store.svelte';
	import { plural } from '$lib/core/format';
	import { orgUsers } from '$lib/features/orgs/users.svelte';
	import { session } from '$lib/core/session.svelte';
	import { teams } from '$lib/features/teams/store.svelte';

	// Filters live in the URL (?card=ID&user=ID|none&team=ID) so they survive reloads and can be linked to.
	const card = $derived.by(() => {
		const id = Number(page.url.searchParams.get('card'));
		return Number.isInteger(id) && id > 0 ? id : null;
	});
	const user = $derived.by((): number | 'none' | null => {
		if (!session.isAdmin) return null;
		const raw = page.url.searchParams.get('user');
		if (raw === 'none') return 'none';
		const id = Number(raw);
		return Number.isInteger(id) && id > 0 ? id : null;
	});

	const team = $derived.by((): number | null => {
		if (!session.seesOthers) return null;
		const id = Number(page.url.searchParams.get('team'));
		return Number.isInteger(id) && id > 0 ? id : null;
	});
	const selectedTeam = $derived(
		team === null ? undefined : (teams.byId(team) ?? session.teams.find((t) => t.id === team))
	);

	const selected = $derived(card === null ? null : (cards.list?.find((p) => p.id === card) ?? null));
	const selectedUser = $derived(typeof user === 'number' ? orgUsers.byId(user) : undefined);
	const totalLeads = $derived(cards.list?.reduce((sum, p) => sum + p.lead_count, 0) ?? null);

	function setFilter(next: { card?: number | null; user?: number | 'none' | null; team?: number | null }) {
		const params = new URLSearchParams();
		const c = next.card !== undefined ? next.card : card;
		const u = next.user !== undefined ? next.user : user;
		const t = next.team !== undefined ? next.team : team;
		if (c !== null) params.set('card', String(c));
		if (u !== null) params.set('user', String(u));
		if (t !== null) params.set('team', String(t));
		const qs = params.toString();
		// Same page, so keep scroll and focus where they are.
		goto(`/dashboard/leads${qs ? `?${qs}` : ''}`, { replace: true, reset: false });
	}

	const filename = $derived(
		[
			selected?.slug,
			selectedTeam?.name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
			selectedUser?.username,
			user === 'none' ? 'organisation' : null
		]
			.filter(Boolean)
			.join('-') || 'fronko'
	);
</script>

<svelte:head>
	<title>Leads · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Leads</h1>
		<p class="text-muted-foreground text-sm">
			{#if selected}
				People who shared their details from {normalizeCard(selected.data).name || selected.slug}{selectedUser
					? ` while ${selectedUser.username} held it`
					: ''}.
			{:else if selectedUser}
				People who shared their details from a card {selectedUser.username} held.
			{:else if selectedTeam}
				People who shared their details from a card someone in {selectedTeam.name} held.
			{:else if user === 'none'}
				People who shared their details from a card nobody was assigned to.
			{:else if totalLeads !== null}
				{plural(totalLeads, 'person', 'people')} shared their details from {session.isAdmin
					? `${session.orgName}’s cards`
					: session.isLead
						? 'your and your teammates’ cards'
						: 'your cards'}.
			{:else}
				People who shared their details from your cards.
			{/if}
		</p>
	</header>

	<LeadsTable
		{card}
		oncardchange={(c) => setFilter({ card: c })}
		{user}
		onuserchange={(u) => setFilter({ user: u })}
		{team}
		onteamchange={(t) => setFilter({ team: t })}
		filename="{filename}-leads"
	/>
</div>
