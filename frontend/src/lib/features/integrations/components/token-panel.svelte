<script lang="ts">
	import { toast } from 'svelte-sonner';
	import KeyRoundIcon from '@lucide/svelte/icons/key-round';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import * as Alert from '$lib/components/ui/alert';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import { formatDateTime, timeAgo } from '$lib/core/format';
	import { rotateToken } from '../api';
	import type { Connection } from '../types';
	import CopyField from './copy-field.svelte';

	/** The secret token a provider (a SCIM client) calls Fronko with. */
	let {
		connection,
		providerName,
		onchange
	}: { connection: Connection; providerName: string; onchange: (c: Connection) => void } = $props();

	let revealed = $state('');
	let working = $state(false);
	let confirmReplace = $state(false);

	async function generate() {
		working = true;
		try {
			const res = await rotateToken(connection.id);
			revealed = res.token;
			confirmReplace = false;
			onchange(res.connection);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to generate a token');
		} finally {
			working = false;
		}
	}
</script>

<div class="bg-muted/40 flex flex-col gap-3 rounded-lg border p-4">
	<div class="flex flex-col gap-1">
		<p class="text-sm font-medium">Secret token</p>
		<p class="text-muted-foreground text-sm">
			{#if connection.token}
				Ends in <span class="font-mono">…{connection.token.hint}</span>, made
				<time title={formatDateTime(connection.token.created_at)}>{timeAgo(connection.token.created_at)}</time>.
				{#if connection.token.last_used_at}
					Last used <time title={formatDateTime(connection.token.last_used_at)}
						>{timeAgo(connection.token.last_used_at)}</time
					>.
				{:else}
					Not used yet.
				{/if}
			{:else}
				{providerName} signs its requests with a token Fronko gives it. Generate one, then paste it into {providerName}.
			{/if}
		</p>
	</div>

	{#if revealed}
		<Alert.Root>
			<TriangleAlertIcon />
			<Alert.Title>Copy the token now</Alert.Title>
			<Alert.Description>It isn't shown again. If you lose it, generate a new one.</Alert.Description>
		</Alert.Root>
		<CopyField id="token-{connection.id}" label="Secret token" value={revealed} />
	{/if}

	<Button
		type="button"
		variant={connection.token ? 'outline' : 'default'}
		class="self-start"
		disabled={working}
		onclick={() => (connection.token ? (confirmReplace = true) : generate())}
	>
		{#if working}<Spinner data-icon="inline-start" />{:else}<KeyRoundIcon data-icon="inline-start" />{/if}
		{connection.token ? 'Replace token' : 'Generate token'}
	</Button>
</div>

<AlertDialog.Root bind:open={confirmReplace}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Replace the token?</AlertDialog.Title>
			<AlertDialog.Description>
				The current token stops working at once, so {providerName} can't sync until you paste the new one into it.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={working}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={generate} disabled={working}>
				{#if working}<Spinner data-icon="inline-start" />{/if}
				Replace token
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
