<script lang="ts">
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import { FEEDBACK_STATUS_LABEL, listFeedback, type Feedback, type FeedbackStatus } from '$lib/api/admin';
	import { CATEGORY_LABEL } from '$lib/api/feedback';
	import { Badge, type BadgeVariant } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Empty from '$lib/components/ui/empty';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Tabs from '$lib/components/ui/tabs';
	import Pagination from '$lib/components/app/pagination.svelte';
	import RatingStars from '$lib/components/app/admin/rating-stars.svelte';
	import { formatDateTime, timeAgo } from '$lib/format';
	import { cn } from '$lib/utils';

	type Tab = FeedbackStatus | 'all';

	let tab = $state<Tab>('new');
	let pageNum = $state(1);
	let pageSize = $state(25);
	let items = $state<Feedback[] | null>(null);
	let total = $state(0);
	let counts = $state<Record<FeedbackStatus, number> | null>(null);
	let loading = $state(false);
	let error = $state('');

	let requestId = 0;
	async function load() {
		const id = ++requestId;
		loading = true;
		error = '';
		try {
			const res = await listFeedback({ status: tab === 'all' ? '' : tab, page: pageNum, pageSize });
			if (id !== requestId) return;
			items = res.items;
			total = res.total;
			counts = res.counts;
		} catch (e) {
			if (id === requestId) error = e instanceof Error ? e.message : 'Failed to load feedback';
		} finally {
			if (id === requestId) loading = false;
		}
	}

	$effect(() => {
		void [tab, pageNum, pageSize];
		load();
	});

	const TABS: Tab[] = ['new', 'read', 'resolved', 'all'];
	const tabCount = (t: Tab) =>
		counts ? (t === 'all' ? counts.new + counts.read + counts.resolved : counts[t]) : undefined;

	const STATUS_VARIANT: Record<FeedbackStatus, BadgeVariant> = { new: 'default', read: 'secondary', resolved: 'outline' };
</script>

<svelte:head>
	<title>Feedback · Fronko admin</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-5xl flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Feedback</h1>
		<p class="text-muted-foreground text-sm">What people tell you from the “Send feedback” menu. Replies go out by email.</p>
	</header>

	<Tabs.Root
		value={tab}
		onValueChange={(v) => {
			tab = v as Tab;
			pageNum = 1;
		}}
	>
		<Tabs.List>
			{#each TABS as t (t)}
				<Tabs.Trigger value={t}>
					{t === 'all' ? 'All' : FEEDBACK_STATUS_LABEL[t]}
					{#if tabCount(t) !== undefined}
						<span class="text-muted-foreground tabular ml-1 text-xs">{tabCount(t)}</span>
					{/if}
				</Tabs.Trigger>
			{/each}
		</Tabs.List>
	</Tabs.Root>

	{#if error && !items}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load feedback</p>
			<p class="text-muted-foreground text-sm">{error}</p>
			<Button variant="outline" onclick={load}>Try again</Button>
		</div>
	{:else if items === null}
		<div class="flex flex-col gap-3">
			{#each [1, 2, 3] as i (i)}<Skeleton class="h-24 rounded-xl" />{/each}
		</div>
	{:else if items.length === 0}
		<Empty.Root class="bg-card rounded-xl border border-dashed py-16">
			<Empty.Header>
				<Empty.Media variant="icon"><MessageSquareIcon /></Empty.Media>
				<Empty.Title>{tab === 'new' ? 'All caught up' : 'Nothing here'}</Empty.Title>
				<Empty.Description>
					{tab === 'new' ? 'New feedback shows up here.' : `No ${tab === 'all' ? '' : tab + ' '}feedback yet.`}
				</Empty.Description>
			</Empty.Header>
		</Empty.Root>
	{:else}
		<ul class={cn('flex flex-col gap-3', loading && 'opacity-60 transition-opacity')}>
			{#each items as f (f.id)}
				<li>
					<a
						href="/admin/feedback/{f.id}"
						class="bg-card hover:bg-muted/40 flex flex-col gap-2 rounded-xl border p-4 transition-colors sm:p-5"
					>
						<div class="flex flex-wrap items-center gap-2 text-sm">
							<Badge variant={STATUS_VARIANT[f.status]}>{FEEDBACK_STATUS_LABEL[f.status]}</Badge>
							<Badge variant="outline">{CATEGORY_LABEL[f.category]}</Badge>
							{#if f.rating}<RatingStars rating={f.rating} />{/if}
							<span class="text-muted-foreground ml-auto text-xs" title={formatDateTime(f.created_at)}>
								{timeAgo(f.created_at)}
							</span>
						</div>
						<p class="line-clamp-2 text-sm text-pretty whitespace-pre-line">{f.message}</p>
						<div class="text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
							<span>{f.sender_email}</span>
							<span>{f.org_name}</span>
							{#if f.reply_count}
								<span class="flex items-center gap-1">
									<ReplyIcon class="size-3" />
									{f.reply_count}
									{f.reply_count === 1 ? 'reply' : 'replies'}
								</span>
							{/if}
						</div>
					</a>
				</li>
			{/each}
		</ul>
		<Pagination bind:page={pageNum} bind:pageSize {total} disabled={loading} />
	{/if}
</div>
