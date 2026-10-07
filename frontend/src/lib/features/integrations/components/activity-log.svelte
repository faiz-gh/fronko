<script lang="ts">
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { formatDateTime, timeAgo } from '$lib/core/format';
	import { cn } from '$lib/utils';
	import { listActivity } from '../api';
	import type { Activity } from '../types';

	/** Bump `version` to reload, e.g. after a test. */
	let { connectionId, version = 0 }: { connectionId: number; version?: number } = $props();

	const PAGE = 20;
	let items = $state<Activity[] | null>(null);
	let error = $state('');
	let more = $state(false);
	let loadingMore = $state(false);

	$effect(() => {
		void version;
		const id = connectionId;
		error = '';
		listActivity(id, undefined, PAGE)
			.then((page) => {
				items = page;
				more = page.length === PAGE;
			})
			.catch((e) => (error = e instanceof Error ? e.message : 'Failed to load activity'));
	});

	async function loadMore() {
		if (!items?.length) return;
		loadingMore = true;
		try {
			const page = await listActivity(connectionId, items[items.length - 1].id, PAGE);
			items = [...items, ...page];
			more = page.length === PAGE;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load activity';
		} finally {
			loadingMore = false;
		}
	}

	const KIND: Record<Activity['kind'], string> = {
		push_lead: 'Lead',
		test: 'Test',
		setup: 'Setup',
		provision: 'Directory',
		sign_in: 'Sign-in'
	};
</script>

{#if error && !items}
	<p class="text-destructive text-sm">{error}</p>
{:else if items === null}
	<div class="flex flex-col gap-2">
		{#each [1, 2, 3] as i (i)}<Skeleton class="h-10" />{/each}
	</div>
{:else if items.length === 0}
	<p class="text-muted-foreground text-sm">Nothing yet. Leads, tests and changes show up here.</p>
{:else}
	<ol class="flex flex-col divide-y rounded-lg border">
		{#each items as a (a.id)}
			<li class="flex gap-3 px-3 py-2.5 text-sm">
				<span
					class={cn(
						'mt-0.5 shrink-0',
						a.outcome === 'success' && 'text-emerald-600 dark:text-emerald-400',
						a.outcome === 'retrying' && 'text-amber-600 dark:text-amber-400',
						a.outcome === 'failed' && 'text-destructive'
					)}
				>
					{#if a.outcome === 'success'}
						<CircleCheckIcon class="size-4" aria-label="Succeeded" />
					{:else if a.outcome === 'retrying'}
						<ClockIcon class="size-4" aria-label="Will retry" />
					{:else}
						<CircleAlertIcon class="size-4" aria-label="Failed" />
					{/if}
				</span>
				<div class="flex min-w-0 flex-1 flex-col gap-0.5">
					<p class="break-words">{a.summary}</p>
					<p class="text-muted-foreground text-xs">
						<span>{KIND[a.kind]}</span>
						{#if a.lead_id}· <a href="/dashboard/leads" class="hover:underline">lead #{a.lead_id}</a>{/if}
						{#if a.attempt && a.attempt > 1}· attempt {a.attempt}{/if}
						{#if a.outcome === 'retrying'}· will retry{/if}
						{#if a.user}· {a.user.username}{/if}
						· <time datetime={a.created_at} title={formatDateTime(a.created_at)}>{timeAgo(a.created_at)}</time>
					</p>
					{#if a.detail && Object.keys(a.detail).length > 0}
						<details class="group mt-1">
							<summary class="text-muted-foreground hover:text-foreground w-fit cursor-pointer text-xs select-none">
								Details
							</summary>
							<dl class="bg-muted/50 mt-1.5 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 rounded-md p-2 text-xs">
								{#each Object.entries(a.detail) as [k, v] (k)}
									<dt class="text-muted-foreground">{k.replaceAll('_', ' ')}</dt>
									<dd class="font-mono break-all">{typeof v === 'string' ? v : JSON.stringify(v)}</dd>
								{/each}
							</dl>
						</details>
					{/if}
				</div>
			</li>
		{/each}
	</ol>
	{#if more}
		<Button variant="ghost" size="sm" class="mt-2 self-start" onclick={loadMore} disabled={loadingMore}>
			{#if loadingMore}<Spinner data-icon="inline-start" />{/if}
			Show older
		</Button>
	{/if}
{/if}
