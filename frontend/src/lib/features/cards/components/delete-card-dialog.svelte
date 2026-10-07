<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { deleteProfile, type Profile } from '$lib/features/cards/api';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Spinner } from '$lib/components/ui/spinner';
	import { cards } from '$lib/features/cards/store.svelte';
	import { session } from '$lib/core/session.svelte';

	let {
		target = $bindable(null),
		ondeleted
	}: {
		/** The card to delete; the dialog is open while this is set. */
		target?: Profile | null;
		ondeleted?: (profile: Profile) => void;
	} = $props();

	let deleting = $state(false);
	const leadCount = $derived(
		target ? (cards.list?.find((p) => p.id === target!.id)?.lead_count ?? target.lead_count) : 0
	);

	async function confirmDelete() {
		if (!target) return;
		const profile = target;
		deleting = true;
		try {
			await deleteProfile(profile.id);
			cards.remove(profile.id);
			toast.success(`Deleted /p/${session.orgHandle}/${profile.slug}`);
			target = null;
			ondeleted?.(profile);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete card');
		} finally {
			deleting = false;
		}
	}
</script>

<AlertDialog.Root open={target !== null} onOpenChange={(open) => !open && (target = null)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete /p/{session.orgHandle}/{target?.slug}?</AlertDialog.Title>
			<AlertDialog.Description>
				The public link stops working immediately, and its {leadCount} captured
				{leadCount === 1 ? 'lead is' : 'leads are'} deleted too. This can't be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{#if deleting}
					<Spinner data-icon="inline-start" />
				{/if}
				Delete card
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
