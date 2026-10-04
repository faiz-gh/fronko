<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import IdCardIcon from '@lucide/svelte/icons/id-card';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SearchIcon from '@lucide/svelte/icons/search';
	import type { Profile } from '$lib/api/profile';
	import { Button } from '$lib/components/ui/button';
	import * as Empty from '$lib/components/ui/empty';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import CardTile from '$lib/components/app/card-tile.svelte';
	import DeleteCardDialog from '$lib/components/app/delete-card-dialog.svelte';
	import QrDialog from '$lib/components/app/qr-dialog.svelte';
	import UserPicker from '$lib/components/app/user-picker.svelte';
	import { normalizeCard } from '$lib/card/card';
	import { cards } from '$lib/cards.svelte';
	import { plural } from '$lib/format';
	import { session } from '$lib/session.svelte';

	// Members see their cards on the overview and in the sidebar.
	$effect(() => {
		if (session.ready && !session.isAdmin) goto('/dashboard', { replaceState: true });
	});

	// The user filter lives in the URL (?user=ID|none) so it can be linked to.
	const user = $derived.by((): number | 'none' | null => {
		const raw = page.url.searchParams.get('user');
		if (raw === 'none') return 'none';
		const id = Number(raw);
		return Number.isInteger(id) && id > 0 ? id : null;
	});
	let query = $state('');

	const all = $derived(cards.list ?? []);
	const unassigned = $derived(all.filter((p) => !p.assigned_user).length);
	const visible = $derived.by(() => {
		const q = query.trim().toLowerCase();
		return all.filter((p) => {
			if (user === 'none' && p.assigned_user) return false;
			if (typeof user === 'number' && p.assigned_user?.id !== user) return false;
			if (!q) return true;
			const c = normalizeCard(p.data);
			return [c.name, c.title, c.company, p.slug, p.assigned_user?.username ?? '']
				.some((v) => v.toLowerCase().includes(q));
		});
	});

	function setUser(value: number | 'none' | null) {
		goto(value === null ? '/dashboard/cards' : `/dashboard/cards?user=${value}`, { replace: true, reset: false });
	}

	let qrTarget = $state<Profile | null>(null);
	let qrOpen = $state(false);
	let deleteTarget = $state<Profile | null>(null);
</script>

<svelte:head>
	<title>Cards · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-wrap items-end justify-between gap-4">
		<div class="flex flex-col gap-1">
			<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Cards</h1>
			<p class="text-muted-foreground text-sm">
				{#if cards.list}
					{plural(all.length, 'card')}{unassigned > 0 ? `, ${unassigned} not assigned to anyone` : ''}. Assign each card
					to the person who uses it; they can edit everything but the link.
				{:else}
					Every card in {session.orgName}.
				{/if}
			</p>
		</div>
		<Button onclick={() => (cards.createOpen = true)}>
			<PlusIcon data-icon="inline-start" />
			New card
		</Button>
	</header>

	{#if cards.error}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load cards</p>
			<p class="text-muted-foreground text-sm">{cards.error}</p>
			<Button variant="outline" onclick={() => session.username && cards.load(session.username, true)}>Try again</Button>
		</div>
	{:else if cards.list && all.length === 0}
		<Empty.Root class="bg-card rounded-xl border border-dashed py-20">
			<Empty.Header>
				<Empty.Media variant="icon">
					<IdCardIcon />
				</Empty.Media>
				<Empty.Title>No cards yet</Empty.Title>
				<Empty.Description>Create a card, then assign it to someone on your team.</Empty.Description>
			</Empty.Header>
			<Empty.Content>
				<Button onclick={() => (cards.createOpen = true)}>
					<PlusIcon data-icon="inline-start" />
					New card
				</Button>
			</Empty.Content>
		</Empty.Root>
	{:else}
		<div class="flex flex-wrap items-center gap-2">
			<UserPicker value={user} onchange={setUser} allLabel="Everyone" />
			<div class="relative min-w-0 basis-full sm:max-w-sm sm:flex-1 sm:basis-auto">
				<SearchIcon class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
				<Input bind:value={query} placeholder="Search name, company, link or user" class="pl-9" aria-label="Search cards" />
			</div>
		</div>

		<div class="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,300px),1fr))]">
			{#if cards.list === null}
				{#each [1, 2, 3] as i (i)}
					<Skeleton class="h-56 rounded-xl" />
				{/each}
			{:else}
				{#each visible as profile (profile.id)}
					<CardTile
						{profile}
						onqr={(p) => {
							qrTarget = p;
							qrOpen = true;
						}}
						ondelete={(p) => (deleteTarget = p)}
					/>
				{:else}
					<p class="text-muted-foreground col-span-full py-12 text-center text-sm">No cards match this filter.</p>
				{/each}
			{/if}
		</div>
	{/if}
</div>

<QrDialog
	bind:open={qrOpen}
	slug={qrTarget?.slug ?? ''}
	name={qrTarget ? normalizeCard(qrTarget.data).name : ''}
/>
<DeleteCardDialog bind:target={deleteTarget} />
