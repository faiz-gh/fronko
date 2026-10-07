<script lang="ts">
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import SendIcon from '@lucide/svelte/icons/send';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import * as Alert from '$lib/components/ui/alert';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Switch } from '$lib/components/ui/switch';
	import * as Tabs from '$lib/components/ui/tabs';
	import { ApiError } from '$lib/core/api';
	import { formatDateTime, timeAgo } from '$lib/core/format';
	import { deleteConnection, getConnection, oauthStartUrl, testConnection, updateConnection } from '../api';
	import { fieldProblems, initialValues, isEmptyChange, settingsChange, type FormValues } from '../fields';
	import { SCOPE_LABEL, STATUS_LABEL, type Tone } from '../registry';
	import type { CatalogEntry, Connection, TestResult } from '../types';
	import ActivityLog from './activity-log.svelte';
	import ConnectionForm from './connection-form.svelte';
	import CopyField from './copy-field.svelte';
	import StatusBadge from './status-badge.svelte';
	import TokenPanel from './token-panel.svelte';

	let {
		entry,
		connection,
		redirectUrl,
		onchange,
		ondelete
	}: {
		entry: CatalogEntry;
		connection: Connection;
		/** The OAuth redirect URL to register, for OAuth providers. */
		redirectUrl: string;
		onchange: (c: Connection) => void;
		ondelete: (id: number) => void;
	} = $props();

	const fields = $derived(entry.fields);
	const isOAuth = $derived(entry.auth === 'oauth2');
	const isLeadSync = $derived(entry.category === 'lead_sync');
	/** What last_synced_at means for this kind of integration. */
	const SYNCED_LABEL: Record<string, string> = {
		lead_sync: 'Last lead sent',
		directory: 'Last change from the directory',
		sso: 'Last sign-in'
	};

	// The form starts from the saved settings, and again after each save.
	let values = $state<FormValues>(untrack(() => initialValues(entry.fields, connection)));
	let name = $state(untrack(() => connection.name));
	let formFor = untrack(() => `${connection.id}:${connection.updated_at}`);
	$effect(() => {
		const key = `${connection.id}:${connection.updated_at}`;
		if (key === formFor) return;
		formFor = key;
		values = initialValues(fields, connection);
		name = connection.name;
	});

	let tab = $state('settings');
	let problems = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);
	let testing = $state(false);
	let toggling = $state(false);
	let deleting = $state(false);
	let confirmDelete = $state(false);
	let testResult = $state<TestResult | null>(null);
	let activityVersion = $state(0);

	const change = $derived(settingsChange(fields, values, connection));
	const nameChanged = $derived(name.trim() !== connection.name && name.trim() !== '');
	const dirty = $derived(nameChanged || !isEmptyChange(change));
	// OAuth needs every other setting saved before the browser leaves.
	const missingBeforeAuth = $derived(connection.missing.length > 0 || dirty);

	const badge = $derived.by((): { tone: Tone; label: string } => {
		if (!connection.enabled) return { tone: 'muted', label: 'Paused' };
		const tone: Tone =
			connection.status === 'active' ? 'success' : connection.status === 'error' ? 'danger' : 'warning';
		return { tone, label: STATUS_LABEL[connection.status] };
	});

	function errorMessage(e: unknown, fallback: string): string {
		return e instanceof Error ? e.message : fallback;
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		formError = '';
		problems = fieldProblems(fields, values, connection);
		if (Object.keys(problems).length > 0) return;
		saving = true;
		try {
			const updated = await updateConnection(connection.id, {
				...(nameChanged ? { name: name.trim() } : {}),
				...change
			});
			onchange(updated);
			activityVersion++;
			toast.success('Saved');
		} catch (e) {
			const field = e instanceof ApiError ? e.field : undefined;
			if (field && fields.some((f) => f.key === field)) problems = { [field]: errorMessage(e, 'Invalid') };
			else formError = errorMessage(e, 'Failed to save');
		} finally {
			saving = false;
		}
	}

	async function setEnabled(enabled: boolean) {
		toggling = true;
		try {
			onchange(await updateConnection(connection.id, { enabled }));
			toast.success(enabled ? 'Resumed' : 'Paused');
		} catch (e) {
			toast.error(errorMessage(e, 'Failed to update'));
		} finally {
			toggling = false;
		}
	}

	async function runTest() {
		testing = true;
		testResult = null;
		try {
			testResult = await testConnection(connection.id);
			onchange(await getConnection(connection.id));
		} catch (e) {
			testResult = { ok: false, summary: errorMessage(e, 'The test couldn’t run') };
		} finally {
			testing = false;
			activityVersion++;
		}
	}

	async function remove() {
		deleting = true;
		try {
			await deleteConnection(connection.id);
			confirmDelete = false;
			ondelete(connection.id);
			toast.success('Connection deleted');
		} catch (e) {
			toast.error(errorMessage(e, 'Failed to delete'));
		} finally {
			deleting = false;
		}
	}
</script>

<article id="connection-{connection.id}" class="bg-card scroll-mt-6 rounded-xl border">
	<header class="flex flex-wrap items-center gap-x-3 gap-y-2 border-b px-4 py-3 sm:px-5">
		<div class="flex min-w-0 flex-1 flex-col">
			<div class="flex min-w-0 flex-wrap items-center gap-2">
				<h3 class="truncate font-semibold">{connection.name}</h3>
				<span class="text-muted-foreground rounded-md border px-1.5 text-xs">{SCOPE_LABEL[connection.scope]}</span>
				<StatusBadge tone={badge.tone} label={badge.label} />
			</div>
			<p class="text-muted-foreground text-xs">
				{#if connection.last_synced_at && SYNCED_LABEL[entry.category]}
					{SYNCED_LABEL[entry.category]}
					<time title={formatDateTime(connection.last_synced_at)}>{timeAgo(connection.last_synced_at)}</time>
				{:else}
					Added {timeAgo(connection.created_at)}{connection.created_by ? ` by ${connection.created_by.username}` : ''}
				{/if}
			</p>
		</div>
		<div class="flex items-center gap-2">
			{#if entry.testable}
				<Button
					variant="outline"
					size="sm"
					onclick={runTest}
					disabled={testing || connection.status === 'pending' || dirty}
					title={dirty ? 'Save your changes first' : undefined}
				>
					{#if testing}<Spinner data-icon="inline-start" />{:else}<SendIcon data-icon="inline-start" />{/if}
					{isLeadSync ? 'Send test lead' : 'Test'}
				</Button>
			{/if}
			<label class="text-muted-foreground flex items-center gap-2 text-sm">
				<Switch checked={connection.enabled} onCheckedChange={setEnabled} disabled={toggling} aria-label="Enabled" />
				<span class="hidden sm:inline">{connection.enabled ? 'On' : 'Off'}</span>
			</label>
		</div>
	</header>

	<div class="flex flex-col gap-4 px-4 py-4 sm:px-5">
		{#if connection.enabled && connection.status === 'error'}
			<Alert.Root variant="destructive">
				<CircleAlertIcon />
				<Alert.Title>Stopped after {connection.failure_count} failures in a row</Alert.Title>
				<Alert.Description>
					{#if connection.last_error}<p>{connection.last_error}</p>{/if}
					<p>New leads aren't sent until you fix the settings and save, or a test succeeds.</p>
				</Alert.Description>
			</Alert.Root>
		{:else if connection.status === 'pending'}
			<Alert.Root>
				<CircleAlertIcon />
				<Alert.Title>Finish setting up</Alert.Title>
				<Alert.Description>
					{#if connection.missing.length > 0}
						Fill in {connection.missing.join(', ')} and save.
					{/if}
					{#if isOAuth && !connection.authorized}
						Then authorise Fronko to use your {entry.name} account.
					{/if}
					{#if entry.auth === 'scim_token' && !connection.token}
						Generate a secret token below and enter it in {entry.name}.
					{/if}
				</Alert.Description>
			</Alert.Root>
		{/if}

		{#if testResult}
			<Alert.Root variant={testResult.ok ? 'default' : 'destructive'}>
				{#if testResult.ok}<CircleCheckIcon />{:else}<CircleAlertIcon />{/if}
				<Alert.Title>{testResult.ok ? 'Test succeeded' : 'Test failed'}</Alert.Title>
				<Alert.Description>{testResult.summary}</Alert.Description>
			</Alert.Root>
		{/if}

		<Tabs.Root bind:value={tab}>
			<Tabs.List variant="line" class="h-9 gap-4 p-0">
				<Tabs.Trigger value="settings" class="flex-none px-0.5">Settings</Tabs.Trigger>
				<Tabs.Trigger value="activity" class="flex-none px-0.5">Activity</Tabs.Trigger>
			</Tabs.List>
			<Tabs.Content value="settings" class="pt-4">
				{#if connection.endpoints.length > 0 || entry.auth === 'scim_token'}
					<div class="mb-5 flex flex-col gap-4">
						{#if connection.endpoints.length > 0}
							<div class="bg-muted/40 flex flex-col gap-4 rounded-lg border p-4">
								<div class="flex flex-col gap-1">
									<p class="text-sm font-medium">Enter these in {entry.name}</p>
									<p class="text-muted-foreground text-sm">Fronko's side of the connection.</p>
								</div>
								{#each connection.endpoints as ep (ep.key)}
									<CopyField id="conn-{connection.id}-{ep.key}" label={ep.label} value={ep.value} help={ep.help} />
								{/each}
							</div>
						{/if}
						{#if entry.auth === 'scim_token'}
							<TokenPanel {connection} providerName={entry.name} {onchange} />
						{/if}
					</div>
				{/if}
				<form onsubmit={save} novalidate class="flex flex-col gap-5">
					<Field.Field>
						<Field.Label for="conn-{connection.id}-name">Name</Field.Label>
						<Input id="conn-{connection.id}-name" bind:value={name} maxlength={80} autocomplete="off" />
						<Field.Description>Only shown here, to tell connections apart.</Field.Description>
					</Field.Field>

					<ConnectionForm
						{fields}
						bind:values
						saved={connection}
						{problems}
						idPrefix="conn-{connection.id}"
						disabled={saving}
					/>

					{#if isOAuth}
						<div class="bg-muted/40 flex flex-col gap-3 rounded-lg border p-4">
							<div class="flex flex-col gap-1">
								<p class="text-sm font-medium">Authorisation</p>
								<p class="text-muted-foreground text-sm">
									{#if connection.authorized}
										Fronko is authorised to use your {entry.name} account.
									{:else}
										Save your app's client ID and secret, then authorise Fronko in {entry.name}.
									{/if}
								</p>
							</div>
							{#if redirectUrl}
								<CopyField id="conn-{connection.id}-redirect" label="Redirect URL for your app" value={redirectUrl} />
							{/if}
							<Button
								type="button"
								variant={connection.authorized ? 'outline' : 'default'}
								class="self-start"
								href={missingBeforeAuth ? undefined : oauthStartUrl(connection.id)}
								disabled={missingBeforeAuth}
							>
								<ExternalLinkIcon data-icon="inline-start" />
								{connection.authorized ? 'Authorise again' : `Authorise with ${entry.name}`}
							</Button>
							{#if dirty}
								<p class="text-muted-foreground text-xs">Save your changes first.</p>
							{/if}
						</div>
					{/if}

					{#if formError}
						<Alert.Root variant="destructive">
							<CircleAlertIcon />
							<Alert.Title>{formError}</Alert.Title>
						</Alert.Root>
					{/if}

					<div class="flex flex-wrap items-center gap-2">
						<Button type="submit" disabled={!dirty || saving}>
							{#if saving}<Spinner data-icon="inline-start" />{/if}
							Save changes
						</Button>
						<Button
							type="button"
							variant="ghost"
							class="text-destructive ml-auto"
							onclick={() => (confirmDelete = true)}
						>
							<Trash2Icon data-icon="inline-start" />
							Delete
						</Button>
					</div>
				</form>
			</Tabs.Content>
			<Tabs.Content value="activity" class="pt-4">
				{#if tab === 'activity'}
					<ActivityLog connectionId={connection.id} version={activityVersion} />
				{/if}
			</Tabs.Content>
		</Tabs.Root>
	</div>
</article>

<AlertDialog.Root bind:open={confirmDelete}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete {connection.name}?</AlertDialog.Title>
			<AlertDialog.Description>
				Its settings, saved secrets and activity are deleted.{isLeadSync
					? ' Leads that already arrived stay in Fronko.'
					: ''}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={remove} disabled={deleting}>
				{#if deleting}<Spinner data-icon="inline-start" />{/if}
				Delete
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
