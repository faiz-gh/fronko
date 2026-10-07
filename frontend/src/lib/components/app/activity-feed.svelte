<script lang="ts">
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import LinkIcon from '@lucide/svelte/icons/link';
	import NfcIcon from '@lucide/svelte/icons/nfc';
	import QrCodeIcon from '@lucide/svelte/icons/qr-code';
	import SendIcon from '@lucide/svelte/icons/send';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import type { ActivityItem } from '$lib/api/analytics';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { formatDateTime, timeAgo } from '$lib/format';

	let {
		items,
		showUser = false,
		empty = 'When someone taps, scans or opens a card, it shows up here.'
	}: {
		/** Newest first; null while loading. */
		items: ActivityItem[] | null;
		/** Name who held the card (admins and team leads). */
		showUser?: boolean;
		empty?: string;
	} = $props();

	function describe(a: ActivityItem): { icon: typeof LinkIcon; text: string } {
		const card = a.card_name || a.slug;
		switch (a.type) {
			case 'view':
				if (a.source === 'nfc') return { icon: NfcIcon, text: `${card}’s card was tapped` };
				if (a.source === 'qr') return { icon: QrCodeIcon, text: `${card}’s QR code was scanned` };
				return { icon: LinkIcon, text: `${card}’s card was opened from a link` };
			case 'vcard':
				return { icon: UserPlusIcon, text: `Someone saved ${card}’s contact` };
			case 'form_submit':
				return { icon: SendIcon, text: `New lead from ${card}’s card` };
			case 'doc_open':
				return { icon: FileTextIcon, text: `“${a.label || 'A brochure'}” opened on ${card}’s card` };
		}
	}
</script>

<section class="bg-card flex flex-col overflow-hidden rounded-xl border" aria-labelledby="activity-heading">
	<div class="flex items-center justify-between gap-3 border-b px-5 py-3">
		<h2 id="activity-heading" class="text-sm font-semibold">Live activity</h2>
		<a href="/dashboard/analytics" class="text-muted-foreground hover:text-foreground text-sm">Analytics</a>
	</div>
	{#if items === null}
		<div class="flex flex-col gap-3 p-5">
			{#each [1, 2, 3] as i (i)}<Skeleton class="h-8" />{/each}
		</div>
	{:else if items.length === 0}
		<p class="text-muted-foreground px-6 py-10 text-center text-sm">{empty}</p>
	{:else}
		<ul class="divide-y">
			{#each items as a, i (i)}
				{@const d = describe(a)}
				<li class="flex items-start gap-3 px-5 py-2.5">
					<span class="bg-muted text-muted-foreground mt-0.5 grid size-7 shrink-0 place-items-center rounded-full">
						<d.icon class="size-3.5" />
					</span>
					<span class="flex min-w-0 flex-1 flex-col">
						<a href="/dashboard/analytics?card={a.profile_id}" class="truncate text-sm hover:underline">{d.text}</a>
						<span class="text-muted-foreground text-xs" title={formatDateTime(a.created_at)}>
							{timeAgo(a.created_at)}{showUser && a.assigned_user ? ` · ${a.assigned_user.username}` : ''}
						</span>
					</span>
				</li>
			{/each}
		</ul>
	{/if}
</section>
