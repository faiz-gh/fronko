<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { reinstateOrg, suspendOrg, type OrgUsage } from '$lib/features/admin/api';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Field from '$lib/components/ui/field';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';

	let {
		org,
		open = $bindable(false),
		onchanged
	}: {
		org: OrgUsage;
		open?: boolean;
		onchanged?: (org: OrgUsage) => void;
	} = $props();

	const MAX_REASON = 500;

	let reason = $state('');
	let working = $state(false);
	let error = $state('');

	const suspending = $derived(!org.suspended_at);
	// One line: the reason goes into an email subject's neighbour and the login screen.
	const cleanReason = $derived(reason.replace(/\s+/g, ' ').trim());

	async function confirm(event: MouseEvent) {
		// Keep the dialog open until the request finishes.
		event.preventDefault();
		if (suspending && !cleanReason) return;
		working = true;
		error = '';
		try {
			const updated = suspending ? await suspendOrg(org.id, cleanReason) : await reinstateOrg(org.id);
			toast.success(suspending ? `${org.name} is suspended` : `${org.name} is reinstated`);
			onchanged?.(updated);
			open = false;
			reason = '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Something went wrong';
		} finally {
			working = false;
		}
	}
</script>

<AlertDialog.Root bind:open onOpenChange={(o) => !o && ((reason = ''), (error = ''))}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{suspending ? `Suspend ${org.name}?` : `Reinstate ${org.name}?`}</AlertDialog.Title>
			<AlertDialog.Description>
				{#if suspending}
					All {org.user_count} of its users are signed out and can't sign in. Its public cards show “unavailable” and stop
					taking leads. Nothing is deleted. We'll email the owner{org.owner_email ? ` (${org.owner_email})` : ''} the reason.
				{:else}
					Everyone in it can sign in again and its public cards come back online. We'll email the owner to let them know.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if suspending}
			<Field.Field>
				<Field.Label for="suspend-reason">Reason</Field.Label>
				<Textarea id="suspend-reason" bind:value={reason} rows={3} maxlength={MAX_REASON} placeholder="e.g. Spam cards reported by visitors" />
				<Field.Description>The owner sees this in the email and on the sign-in page.</Field.Description>
			</Field.Field>
		{/if}
		{#if error}
			<Field.Error>{error}</Field.Error>
		{/if}
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={working}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action
				variant={suspending ? 'destructive' : 'default'}
				onclick={confirm}
				disabled={working || (suspending && !cleanReason)}
			>
				{#if working}<Spinner data-icon="inline-start" />{/if}
				{suspending ? 'Suspend organisation' : 'Reinstate organisation'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
