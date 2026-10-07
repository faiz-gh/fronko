<script lang="ts">
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import SendIcon from '@lucide/svelte/icons/send';
	import {
		FEEDBACK_STATUS_LABEL,
		getFeedback,
		replyFeedback,
		setFeedbackStatus,
		type Feedback,
		type FeedbackStatus
	} from '$lib/features/admin/api';
	import { ApiError } from '$lib/core/api';
	import { CATEGORY_LABEL } from '$lib/features/feedback/api';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import RatingStars from '$lib/features/admin/components/rating-stars.svelte';
	import { adminNav } from '$lib/features/admin/nav.svelte';
	import { formatDateTime } from '$lib/core/format';

	const id = $derived(Number(page.params.id));

	let feedback = $state<Feedback | null>(null);
	let notFound = $state(false);
	let error = $state('');
	let reply = $state('');
	let sending = $state(false);
	let replyError = $state('');
	let savingStatus = $state(false);

	async function load() {
		error = '';
		try {
			feedback = await getFeedback(id);
			// Opening new feedback marks it read.
			if (feedback.status === 'new') await changeStatus('read', false);
		} catch (e) {
			if (e instanceof ApiError && e.status === 404) notFound = true;
			else error = e instanceof Error ? e.message : 'Failed to load';
		}
	}

	$effect(() => {
		void id;
		load();
	});

	async function changeStatus(status: FeedbackStatus, announce = true) {
		if (!feedback || feedback.status === status) return;
		savingStatus = true;
		try {
			feedback = await setFeedbackStatus(feedback.id, status);
			adminNav.refresh();
			if (announce) toast.success(`Marked ${FEEDBACK_STATUS_LABEL[status].toLowerCase()}`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to update');
		} finally {
			savingStatus = false;
		}
	}

	async function sendReply(event: SubmitEvent) {
		event.preventDefault();
		if (!feedback || !reply.trim()) return;
		sending = true;
		replyError = '';
		try {
			feedback = await replyFeedback(feedback.id, reply.trim());
			const last = feedback.replies?.at(-1);
			if (last && !last.email_sent) toast.warning('Reply saved, but the email could not be sent');
			else toast.success(`Reply emailed to ${feedback.sender_email}`);
			reply = '';
			adminNav.refresh();
		} catch (e) {
			replyError = e instanceof Error ? e.message : 'Failed to send reply';
		} finally {
			sending = false;
		}
	}
</script>

<svelte:head>
	<title>Feedback · Fronko admin</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<a href="/admin/feedback" class="text-muted-foreground hover:text-foreground flex w-fit items-center gap-1.5 text-sm">
		<ArrowLeftIcon class="size-4" />
		Feedback
	</a>

	{#if notFound}
		<div class="bg-card rounded-xl border p-6"><p class="font-medium">Feedback not found</p></div>
	{:else if error}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load this feedback</p>
			<p class="text-muted-foreground text-sm">{error}</p>
			<Button variant="outline" onclick={load}>Try again</Button>
		</div>
	{:else if !feedback}
		<Skeleton class="h-48 rounded-xl" />
	{:else}
		<article class="bg-card flex flex-col gap-4 rounded-xl border p-5 sm:p-6">
			<div class="flex flex-wrap items-center gap-2">
				<Badge variant="outline">{CATEGORY_LABEL[feedback.category]}</Badge>
				{#if feedback.rating}<RatingStars rating={feedback.rating} />{/if}
			</div>
			<p class="text-[15px] leading-relaxed text-pretty whitespace-pre-wrap">{feedback.message}</p>
			<dl class="text-muted-foreground grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 border-t pt-4 text-sm">
				<dt>From</dt>
				<dd class="text-foreground break-all">{feedback.sender_email}</dd>
				<dt>Organisation</dt>
				<dd class="text-foreground">
					{#if feedback.org_id}
						<a href="/admin/orgs/{feedback.org_id}" class="hover:underline">{feedback.org_name}</a>
					{:else}
						{feedback.org_name} <span class="text-muted-foreground">(deleted)</span>
					{/if}
				</dd>
				{#if feedback.page_path}
					<dt>Page</dt>
					<dd class="text-foreground font-mono text-xs leading-5 break-all">{feedback.page_path}</dd>
				{/if}
				<dt>Sent</dt>
				<dd class="text-foreground">{formatDateTime(feedback.created_at)}</dd>
			</dl>
		</article>

		<div class="flex flex-wrap items-center justify-between gap-3">
			<span class="text-muted-foreground text-sm">Status</span>
			<ToggleGroup.Root
				type="single"
				variant="outline"
				size="sm"
				value={feedback.status}
				onValueChange={(v) => v && changeStatus(v as FeedbackStatus)}
				disabled={savingStatus}
				aria-label="Status"
			>
				{#each ['new', 'read', 'resolved'] as const as s (s)}
					<ToggleGroup.Item value={s}>{FEEDBACK_STATUS_LABEL[s]}</ToggleGroup.Item>
				{/each}
			</ToggleGroup.Root>
		</div>

		{#if feedback.replies?.length}
			<section class="flex flex-col gap-3" aria-label="Replies">
				{#each feedback.replies as r (r.id)}
					<div class="bg-brand-soft/50 flex flex-col gap-2 rounded-xl border p-4 sm:ml-8">
						<div class="text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
							<span class="text-foreground font-medium">{r.admin_email ?? 'A former admin'}</span>
							<span>{formatDateTime(r.created_at)}</span>
							{#if !r.email_sent}
								<span class="text-destructive flex items-center gap-1">
									<CircleAlertIcon class="size-3" /> Email not sent
								</span>
							{/if}
						</div>
						<p class="text-sm leading-relaxed whitespace-pre-wrap">{r.body}</p>
					</div>
				{/each}
			</section>
		{/if}

		<form onsubmit={sendReply} class="flex flex-col gap-3">
			<Field.Field>
				<Field.Label for="reply">Reply</Field.Label>
				<Textarea id="reply" bind:value={reply} rows={5} maxlength={5000} placeholder="Write a reply…" />
				<Field.Description>
					Sent by email to {feedback.sender_email}, with their message quoted.
				</Field.Description>
				{#if replyError}<Field.Error>{replyError}</Field.Error>{/if}
			</Field.Field>
			<Button type="submit" class="self-end" disabled={sending || !reply.trim()}>
				{#if sending}<Spinner data-icon="inline-start" />{:else}<SendIcon data-icon="inline-start" />{/if}
				Send reply
			</Button>
		</form>
	{/if}
</div>
