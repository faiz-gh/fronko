<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { tick } from 'svelte';
	import { toast } from 'svelte-sonner';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { session } from '$lib/core/session.svelte';
	import { listConnections } from '$lib/features/integrations/api';
	import ConnectionPanel from '$lib/features/integrations/components/connection-panel.svelte';
	import DomainsPanel from '$lib/features/integrations/components/domains-panel.svelte';
	import NewConnection from '$lib/features/integrations/components/new-connection.svelte';
	import ProviderLogo from '$lib/features/integrations/components/provider-logo.svelte';
	import { canConnect } from '$lib/features/integrations/fields';
	import { SCOPE_HELP, SCOPE_LABEL } from '$lib/features/integrations/registry';
	import { integrations } from '$lib/features/integrations/store.svelte';
	import type { Connection, Scope } from '$lib/features/integrations/types';

	const REQUEST_URL = 'https://github.com/faiz-gh/fronko/issues/new?template=integration_request.yml';

	const providerId = $derived(page.params.provider ?? '');

	$effect(() => {
		if (session.ready && session.username) integrations.load(session.username);
	});
	const entry = $derived(integrations.byId(providerId));
	const category = $derived(entry ? integrations.catalog?.categories.find((c) => c.id === entry.category) : undefined);

	let connections = $state<Connection[] | null>(null);
	let loadError = $state('');
	$effect(() => {
		const id = providerId;
		if (!session.ready) return;
		connections = null;
		loadError = '';
		listConnections(id)
			.then((list) => (connections = list))
			.catch((e) => (loadError = e instanceof Error ? e.message : 'Failed to load connections'));
	});

	// In categories with one connection per owner (booking pages, directory,
	// single sign-on), a connection to another provider takes the place.
	const takenBy = $derived.by((): Partial<Record<Scope, string>> => {
		const catalog = integrations.catalog;
		if (!entry || !catalog?.categories.find((c) => c.id === entry.category)?.single) return {};
		const out: Partial<Record<Scope, string>> = {};
		for (const p of catalog.providers) {
			if (p.category !== entry.category || p.id === entry.id) continue;
			for (const c of p.connections) out[c.scope] ??= p.name;
		}
		return out;
	});

	// Scopes this user may still add: each owner gets one connection unless
	// the provider allows more.
	const addableScopes = $derived.by((): Scope[] => {
		if (!entry || !connections || entry.status === 'coming_soon' || entry.unavailable) return [];
		return entry.scopes.filter(
			(s) =>
				canConnect(entry, s, session.isAdmin) &&
				!takenBy[s] &&
				(entry.multiple || !connections!.some((c) => c.scope === s))
		);
	});
	const blockedScopes = $derived(
		entry ? entry.scopes.filter((s) => takenBy[s] && canConnect(entry, s, session.isAdmin)) : []
	);

	let adding = $state(false);
	const showNew = $derived(adding || (connections?.length === 0 && addableScopes.length > 0));

	function upsert(c: Connection) {
		if (!connections) return;
		const i = connections.findIndex((x) => x.id === c.id);
		connections = i === -1 ? [...connections, c] : connections.with(i, c);
		integrations.refresh();
	}

	function remove(id: number) {
		connections = connections?.filter((c) => c.id !== id) ?? null;
		integrations.refresh();
	}

	async function created(c: Connection) {
		adding = false;
		upsert(c);
		toast.success(c.status === 'active' ? `${c.name} connected` : `${c.name} saved`);
		await tick();
		document.getElementById(`connection-${c.id}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	// Coming back from an OAuth provider: say how it went, then tidy the URL.
	$effect(() => {
		const params = page.url.searchParams;
		const ok = params.get('oauth') === 'connected';
		const failed = params.get('oauth_error');
		if (!ok && !failed) return;
		const target = params.get('connection');
		if (ok) toast.success('Authorised');
		if (failed) toast.error(failed);
		goto(page.url.pathname, { replace: true, reset: false });
		if (target) {
			tick().then(() =>
				document.getElementById(`connection-${target}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
			);
		}
	});
</script>

<svelte:head>
	<title>{entry?.name ?? 'Integration'} · Integrations · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1200px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<a
		href="/dashboard/integrations"
		class="text-muted-foreground hover:text-foreground inline-flex w-fit items-center gap-1.5 text-sm"
	>
		<ArrowLeftIcon class="size-4" />
		Integrations
	</a>

	{#if integrations.error && !integrations.catalog}
		<p class="text-destructive text-sm">{integrations.error}</p>
	{:else if !integrations.catalog}
		<div class="flex items-center gap-4">
			<Skeleton class="size-14 rounded-xl" />
			<div class="flex flex-col gap-2"><Skeleton class="h-7 w-48" /><Skeleton class="h-4 w-80" /></div>
		</div>
		<Skeleton class="h-64 rounded-xl" />
	{:else if !entry}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Integration not found</p>
			<p class="text-muted-foreground text-sm">It may not exist, or it isn't available to you.</p>
			<Button variant="outline" href="/dashboard/integrations">See all integrations</Button>
		</div>
	{:else}
		<header class="flex flex-wrap items-start gap-4">
			<ProviderLogo id={entry.id} name={entry.name} class="size-14 rounded-xl text-lg" />
			<div class="flex min-w-0 flex-1 flex-col gap-1">
				<p class="text-muted-foreground text-xs font-medium tracking-wide uppercase">{category?.label}</p>
				<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">{entry.name}</h1>
				<p class="text-muted-foreground max-w-2xl text-sm">{entry.description}</p>
			</div>
			{#if entry.docs_url}
				<Button variant="outline" href={entry.docs_url} target="_blank" rel="noopener noreferrer">
					<ExternalLinkIcon data-icon="inline-start" />
					Setup guide
				</Button>
			{/if}
		</header>

		{#if entry.status === 'coming_soon'}
			<div class="bg-card flex flex-col items-start gap-3 rounded-xl border border-dashed p-6">
				<p class="font-medium">Coming soon</p>
				<p class="text-muted-foreground max-w-xl text-sm">
					{entry.name} isn't available yet. If you need it, say so in an integration request: it helps decide what comes next,
					and you can follow along.
				</p>
				<Button variant="outline" href={REQUEST_URL} target="_blank" rel="noopener noreferrer">
					<ExternalLinkIcon data-icon="inline-start" />
					Request {entry.name}
				</Button>
			</div>
		{:else}
			<div class="grid gap-8 lg:grid-cols-[minmax(0,1fr)_300px]">
				<div class="flex min-w-0 flex-col gap-4">
					{#if entry.unavailable}
						<Alert.Root>
							<CircleAlertIcon />
							<Alert.Title>Not available on this server</Alert.Title>
							<Alert.Description>{entry.unavailable}. Ask whoever runs your Fronko server to set it.</Alert.Description>
						</Alert.Root>
					{/if}

					{#if loadError}
						<p class="text-destructive text-sm">{loadError}</p>
					{:else if connections === null}
						<Skeleton class="h-64 rounded-xl" />
					{:else}
						{#each connections as c (c.id)}
							<ConnectionPanel
								{entry}
								connection={c}
								redirectUrl={integrations.catalog.oauth_redirect_url}
								onchange={upsert}
								ondelete={remove}
							/>
						{/each}

						{#each blockedScopes as s (s)}
							<Alert.Root>
								<CircleAlertIcon />
								<Alert.Title>
									{s === 'user' ? 'You already use' : 'Your organisation already uses'}
									{takenBy[s]} for this
								</Alert.Title>
								<Alert.Description>
									Only one {category?.label.toLowerCase()} connection is allowed{s === 'user' ? ' per person' : ''}. To
									switch to {entry.name}, remove the {takenBy[s]} connection first.
								</Alert.Description>
							</Alert.Root>
						{/each}

						{#if showNew}
							<NewConnection
								{entry}
								scopes={addableScopes}
								oncreated={created}
								oncancel={connections.length > 0 ? () => (adding = false) : undefined}
							/>
						{:else if addableScopes.length > 0}
							<Button variant="outline" class="self-start" onclick={() => (adding = true)}>
								<PlusIcon data-icon="inline-start" />
								Add another connection
							</Button>
						{/if}

						{#if entry.category === 'sso' && session.isAdmin}
							<DomainsPanel />
						{/if}
					{/if}
				</div>

				<aside class="flex flex-col gap-6 text-sm">
					{#if entry.setup_steps?.length}
						<section class="flex flex-col gap-3">
							<h2 class="font-semibold">How to set it up</h2>
							<ol class="flex flex-col gap-3">
								{#each entry.setup_steps as step, i (i)}
									<li class="flex gap-3">
										<span
											class="bg-muted text-muted-foreground grid size-6 shrink-0 place-items-center rounded-full text-xs font-medium"
										>
											{i + 1}
										</span>
										<span class="text-muted-foreground pt-0.5">{step}</span>
									</li>
								{/each}
							</ol>
						</section>
					{/if}
					<section class="flex flex-col gap-2">
						<h2 class="font-semibold">Who it's for</h2>
						<ul class="text-muted-foreground flex flex-col gap-2">
							{#each entry.scopes as s (s)}
								{#if SCOPE_HELP[entry.category][s]}
									<li>
										<span class="text-foreground font-medium">{SCOPE_LABEL[s]}:</span>
										{SCOPE_HELP[entry.category][s]}
									</li>
								{/if}
							{/each}
						</ul>
						{#if entry.scopes.includes('org') && !session.isAdmin}
							<p class="text-muted-foreground text-xs">Organisation connections are managed by your admins.</p>
						{/if}
					</section>
				</aside>
			</div>
		{/if}
	{/if}
</div>
