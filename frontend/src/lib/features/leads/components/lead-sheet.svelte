<script lang="ts">
	import { toast } from 'svelte-sonner';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { deleteLead, type Lead } from '$lib/features/leads/api';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Spinner } from '$lib/components/ui/spinner';
	import { formatDateTime, timeAgo } from '$lib/core/format';
	import { e164, formatPhone } from '$lib/core/phone';
	import { session } from '$lib/core/session.svelte';

	let {
		lead = $bindable(null),
		card,
		ondeleted
	}: {
		/** The lead shown; the sheet is open while this is set. */
		lead?: Lead | null;
		/** The card it came through, when known. */
		card?: { name: string; slug: string; color: string } | null;
		ondeleted?: (lead: Lead) => void;
	} = $props();

	const SOURCE_LABEL = { nfc: 'NFC tap', qr: 'QR code scan', link: 'Link' } as const;

	let confirming = $state(false);
	let deleting = $state(false);

	async function confirmDelete() {
		if (!lead) return;
		const target = lead;
		deleting = true;
		try {
			await deleteLead(target.id);
			toast.success(`Deleted ${target.name}'s details`);
			confirming = false;
			lead = null;
			ondeleted?.(target);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete lead');
		} finally {
			deleting = false;
		}
	}
</script>

<Sheet.Root open={lead !== null} onOpenChange={(open) => !open && (lead = null)}>
	<Sheet.Content class="w-full gap-0 overflow-y-auto p-0 sm:max-w-md">
		{#if lead}
			<Sheet.Header class="border-b px-5 py-4 pr-12">
				<Sheet.Title class="truncate">{lead.name}</Sheet.Title>
				<Sheet.Description title={formatDateTime(lead.created_at)}>
					Received {timeAgo(lead.created_at)}
				</Sheet.Description>
			</Sheet.Header>

			<div class="flex flex-col gap-6 px-5 py-5">
				<div class="flex flex-wrap gap-2">
					<Button variant="outline" size="sm" href="mailto:{lead.email}">
						<MailIcon data-icon="inline-start" />
						Email
					</Button>
					{#if lead.phone_number}
						<Button variant="outline" size="sm" href="tel:{e164(lead.phone_country_code ?? '', lead.phone_number)}">
							<PhoneIcon data-icon="inline-start" />
							Call
						</Button>
					{/if}
				</div>

				<dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
					<dt class="text-muted-foreground">Email</dt>
					<dd class="min-w-0 break-all">{lead.email}</dd>
					<dt class="text-muted-foreground">Phone</dt>
					<dd class="tabular-nums">
						{lead.phone_number ? formatPhone(lead.phone_country_code ?? '', lead.phone_number) : '–'}
					</dd>
					{#if card}
						<dt class="text-muted-foreground">Card</dt>
						<dd class="flex min-w-0 items-center gap-1.5">
							<span class="size-2 shrink-0 rounded-full" style="background: {card.color}"></span>
							<span class="truncate">{card.name}</span>
						</dd>
					{/if}
					{#if session.seesOthers}
						<dt class="text-muted-foreground">Held by</dt>
						<dd>{lead.assigned_user?.username ?? 'Organisation'}</dd>
					{/if}
					<dt class="text-muted-foreground">Came from</dt>
					<dd>{lead.source ? SOURCE_LABEL[lead.source] : '–'}</dd>
					<dt class="text-muted-foreground">Received</dt>
					<dd>{formatDateTime(lead.created_at)}</dd>
				</dl>

				<div class="flex flex-col gap-1.5">
					<h3 class="text-sm font-medium">Message</h3>
					{#if lead.notes}
						<p class="text-foreground/90 text-sm whitespace-pre-wrap">{lead.notes}</p>
					{:else}
						<p class="text-muted-foreground text-sm">No message.</p>
					{/if}
				</div>
			</div>

			{#if session.isAdmin}
				<Sheet.Footer class="border-t px-5 py-4">
					<Button variant="destructive" onclick={() => (confirming = true)}>
						<Trash2Icon data-icon="inline-start" />
						Delete lead
					</Button>
				</Sheet.Footer>
			{/if}
		{/if}
	</Sheet.Content>
</Sheet.Root>

<AlertDialog.Root open={confirming} onOpenChange={(open) => !deleting && (confirming = open)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete {lead?.name}'s details?</AlertDialog.Title>
			<AlertDialog.Description>
				The lead is removed for everyone in {session.orgName}, with its lead sync history, and is no longer included in
				exports. Copies already sent to connected tools (such as a CRM) aren't deleted there. This can't be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{#if deleting}<Spinner data-icon="inline-start" />{/if}
				Delete lead
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
