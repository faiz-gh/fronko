<script lang="ts">
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import UserRoundCogIcon from '@lucide/svelte/icons/user-round-cog';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import * as Alert from '$lib/components/ui/alert';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { ApiError } from '$lib/core/api';
	import { plural } from '$lib/core/format';
	import { session } from '$lib/core/session.svelte';
	import {
		deleteAccount,
		getDeletionInfo,
		sendDeletionCode,
		transferOwnership,
		type DeletionInfo,
		type Reauth
	} from '../api';
	import CodeInput from './code-input.svelte';

	let {
		open = $bindable(false),
		onexport,
		exporting = false
	}: {
		open?: boolean;
		/** Downloads the person's data, offered before they delete it. */
		onexport: () => void;
		exporting?: boolean;
	} = $props();

	/**
	 * leave: a member or admin deletes their account; the organisation keeps their work.
	 * delete-org: the owner deletes their account and the whole organisation.
	 * transfer: the owner first hands the organisation to someone else.
	 */
	type Mode = 'leave' | 'delete-org' | 'transfer';
	type Step = 'choose' | 'transfer' | 'confirm';

	let info = $state<DeletionInfo | null>(null);
	let loadError = $state('');
	let mode = $state<Mode>('leave');
	let step = $state<Step>('confirm');

	let understood = $state(false);
	let confirmText = $state('');
	let password = $state('');
	let code = $state('');
	let codeSent = $state(false);
	let sendingCode = $state(false);
	let newOwner = $state('');
	let busy = $state(false);
	let error = $state('');

	async function load() {
		info = null;
		loadError = '';
		try {
			const i = await getDeletionInfo();
			info = i;
			if (i.role !== 'owner') {
				mode = 'leave';
				step = 'confirm';
			} else if (i.sole_member) {
				mode = 'delete-org';
				step = 'confirm';
			} else {
				step = 'choose';
			}
		} catch (e) {
			loadError = e instanceof Error ? e.message : "Couldn't load your account";
		}
	}

	function reset() {
		understood = false;
		confirmText = password = code = newOwner = error = '';
		codeSent = false;
	}

	$effect(() => {
		if (open) {
			reset();
			load();
		}
	});

	const confirmWord = $derived(info ? (mode === 'delete-org' ? info.org_handle : info.username) : '');
	const reauth = $derived<Reauth>(info?.has_password ? { password } : { code });
	const reauthReady = $derived(info?.has_password ? password.length > 0 : code.length === 6);
	const canDelete = $derived(
		!!info &&
			step === 'confirm' &&
			understood &&
			confirmText.trim().toLowerCase() === confirmWord.toLowerCase() &&
			reauthReady &&
			!busy
	);
	const candidate = $derived(info?.transfer_candidates.find((c) => String(c.id) === newOwner));

	async function requestCode() {
		sendingCode = true;
		error = '';
		try {
			await sendDeletionCode();
			codeSent = true;
			toast.success(`We emailed a code to ${session.email ?? 'your address'}`);
		} catch (e) {
			if (e instanceof ApiError && e.status === 429) codeSent = true;
			error = e instanceof Error ? e.message : "Couldn't send the code";
		} finally {
			sendingCode = false;
		}
	}

	function choose(next: Mode) {
		mode = next;
		step = next === 'transfer' ? 'transfer' : 'confirm';
		understood = false;
		confirmText = error = '';
	}

	async function transfer(event: SubmitEvent) {
		event.preventDefault();
		if (!candidate || !reauthReady || busy) return;
		busy = true;
		error = '';
		try {
			const me = await transferOwnership(candidate.id, reauth);
			session.signIn(me);
			toast.success(`${candidate.username} is now the owner of ${info?.org_name}. You're an admin.`);
			// The emailed code was used up; a password can be reused.
			code = '';
			codeSent = false;
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : "Couldn't hand over the organisation";
		} finally {
			busy = false;
		}
	}

	async function remove(event: SubmitEvent) {
		event.preventDefault();
		if (!canDelete || !info) return;
		busy = true;
		error = '';
		try {
			await deleteAccount(confirmText.trim(), reauth, mode === 'delete-org');
			open = false;
			session.clear();
			await goto('/login?deleted=1', { replaceState: true });
		} catch (e) {
			error = e instanceof Error ? e.message : "Couldn't delete your account";
			if (e instanceof ApiError && e.code === 'ownership_transfer_required') step = 'choose';
		} finally {
			busy = false;
		}
	}
</script>

{#snippet reauthField(idPrefix: string)}
	{#if info?.has_password}
		<Field.Field>
			<Field.Label for="{idPrefix}-password">Your password</Field.Label>
			<Input
				id="{idPrefix}-password"
				type="password"
				autocomplete="current-password"
				bind:value={password}
				disabled={busy}
			/>
		</Field.Field>
	{:else}
		<Field.Field>
			<Field.Label for="{idPrefix}-code">Code from your email</Field.Label>
			<Field.Description>
				You sign in with single sign-on, so we confirm it's you with a code sent to {session.email}.
			</Field.Description>
			{#if codeSent}
				<CodeInput id="{idPrefix}-code" bind:value={code} disabled={busy} />
			{/if}
			<div>
				<Button type="button" variant="outline" size="sm" onclick={requestCode} disabled={sendingCode || busy}>
					{#if sendingCode}<Spinner data-icon="inline-start" />{/if}
					{codeSent ? 'Send a new code' : 'Email me a code'}
				</Button>
			</div>
		</Field.Field>
	{/if}
{/snippet}

<Dialog.Root bind:open>
	<Dialog.Content class="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-lg">
		{#if loadError}
			<Dialog.Header>
				<Dialog.Title>Delete account</Dialog.Title>
			</Dialog.Header>
			<Alert.Root variant="destructive">
				<CircleAlertIcon />
				<Alert.Title>{loadError}</Alert.Title>
			</Alert.Root>
		{:else if !info}
			<div class="flex flex-col gap-3" aria-busy="true">
				<Skeleton class="h-6 w-48" />
				<Skeleton class="h-24 w-full" />
				<Skeleton class="h-10 w-full" />
			</div>
		{:else if info.managed}
			<Dialog.Header>
				<Dialog.Title>Your account is managed by {info.org_name}</Dialog.Title>
				<Dialog.Description>
					{info.org_name} adds and removes people through its identity provider, so deleting your account here would only
					bring it back. Ask your IT team to remove you; that deletes your Fronko account too.
				</Dialog.Description>
			</Dialog.Header>
			<Dialog.Footer>
				<Dialog.Close class={buttonVariants({ variant: 'outline' })}>Close</Dialog.Close>
			</Dialog.Footer>
		{:else if step === 'choose'}
			<Dialog.Header>
				<Dialog.Title>You own {info.org_name}</Dialog.Title>
				<Dialog.Description>
					An organisation always needs an owner, and {plural(
						info.summary.org_members - 1,
						'other person',
						'other people'
					)}
					still work in it. Choose what happens to it before your account goes.
				</Dialog.Description>
			</Dialog.Header>
			{#if error}
				<Alert.Root variant="destructive">
					<CircleAlertIcon />
					<Alert.Title>{error}</Alert.Title>
				</Alert.Root>
			{/if}
			<div class="flex flex-col gap-3">
				<button
					type="button"
					class="hover:bg-muted/60 flex items-start gap-3 rounded-lg border p-4 text-left transition-colors disabled:opacity-50"
					onclick={() => choose('transfer')}
					disabled={info.transfer_candidates.length === 0}
				>
					<UserRoundCogIcon class="text-muted-foreground mt-0.5 size-5 shrink-0" />
					<span class="flex flex-col gap-1">
						<span class="font-medium">Hand it over, then delete my account</span>
						<span class="text-muted-foreground text-sm">
							{#if info.transfer_candidates.length === 0}
								Nobody can take over yet: the new owner must be active and have verified their email.
							{:else}
								Someone else becomes the owner. The organisation, its cards and its leads carry on without you.
							{/if}
						</span>
					</span>
				</button>
				<button
					type="button"
					class="border-destructive/40 hover:bg-destructive/5 flex items-start gap-3 rounded-lg border p-4 text-left transition-colors"
					onclick={() => choose('delete-org')}
				>
					<Trash2Icon class="text-destructive mt-0.5 size-5 shrink-0" />
					<span class="flex flex-col gap-1">
						<span class="text-destructive font-medium">Delete {info.org_name} with my account</span>
						<span class="text-muted-foreground text-sm">
							Everything goes, for everyone: {plural(info.summary.org_members, 'person', 'people')},
							{plural(info.summary.org_cards, 'card')} and {plural(info.summary.org_leads, 'lead')}.
						</span>
					</span>
				</button>
			</div>
			<Dialog.Footer>
				<Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close>
			</Dialog.Footer>
		{:else if step === 'transfer'}
			<Dialog.Header>
				<Dialog.Title>Hand over {info.org_name}</Dialog.Title>
				<Dialog.Description>
					The new owner gets full control, including storage settings and everyone's accounts. You become an admin, and
					can then delete your account. Both of you are signed out on other devices.
				</Dialog.Description>
			</Dialog.Header>
			<form onsubmit={transfer} class="flex flex-col gap-5">
				<Field.Field>
					<Field.Label for="new-owner">New owner</Field.Label>
					<Select.Root type="single" bind:value={newOwner} disabled={busy}>
						<Select.Trigger id="new-owner" class="w-full">
							{candidate?.username ?? 'Choose someone'}
						</Select.Trigger>
						<Select.Content>
							{#each info.transfer_candidates as c (c.id)}
								<Select.Item value={String(c.id)} label={c.username}>{c.username}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</Field.Field>
				{@render reauthField('transfer')}
				{#if error}
					<Alert.Root variant="destructive">
						<CircleAlertIcon />
						<Alert.Title>{error}</Alert.Title>
					</Alert.Root>
				{/if}
				<Dialog.Footer>
					<Button type="button" variant="outline" onclick={() => (step = 'choose')} disabled={busy}>Back</Button>
					<Button type="submit" disabled={!candidate || !reauthReady || busy}>
						{#if busy}<Spinner data-icon="inline-start" />{/if}
						Make {candidate?.username ?? 'them'} the owner
					</Button>
				</Dialog.Footer>
			</form>
		{:else}
			<Dialog.Header>
				<Dialog.Title class="flex items-center gap-2">
					<TriangleAlertIcon class="text-destructive size-5" />
					{mode === 'delete-org' ? `Delete your account and ${info.org_name}` : 'Delete your account'}
				</Dialog.Title>
				<Dialog.Description
					>This is permanent. There is no undo, no grace period and no way for us to restore it.</Dialog.Description
				>
			</Dialog.Header>

			<form onsubmit={remove} class="flex flex-col gap-5">
				<Alert.Root variant="destructive">
					<TriangleAlertIcon />
					<Alert.Title>What happens straight away</Alert.Title>
					<Alert.Description>
						<ul class="mt-1 list-disc space-y-1 pl-4">
							{#if mode === 'delete-org'}
								<li>
									<strong>{info.org_name}</strong> is deleted, with
									{plural(info.summary.org_members, 'person', 'people')}, {plural(info.summary.org_teams, 'team')},
									{plural(info.summary.org_cards, 'card')}, {plural(info.summary.org_leads, 'lead')}, all analytics and
									integrations.
								</li>
								<li>
									{plural(info.summary.org_files, 'file')} are deleted from your storage bucket. Anything else in the bucket
									stays; it's yours to empty.
								</li>
								<li>Every card link, QR code and NFC tag stops working. Links already printed can't be fixed.</li>
								{#if !info.sole_member}
									<li>Everyone in {info.org_name} loses access immediately, without warning.</li>
								{/if}
							{:else}
								<li>You're signed out everywhere and can't sign in again.</li>
								<li>
									{#if info.summary.cards_held > 0}
										{plural(info.summary.cards_held, 'card')} you hold go back to {info.org_name}, and their
										{plural(info.summary.leads, 'lead')} stay with it.
									{:else}
										Leads and cards stay with {info.org_name}.
									{/if}
								</li>
								{#if info.summary.files > 0}
									<li>
										Your {plural(info.summary.files, 'file')} pass to {info.owner_username ?? 'the owner'}, so cards
										using them keep working.
									</li>
								{/if}
								<li>Your teams, personal integrations and email are removed.</li>
							{/if}
						</ul>
					</Alert.Description>
				</Alert.Root>

				<div class="bg-muted/50 flex flex-wrap items-center justify-between gap-3 rounded-lg p-3 text-sm">
					<span class="text-muted-foreground">Want a copy first? Download everything we hold about you.</span>
					<Button type="button" variant="outline" size="sm" onclick={onexport} disabled={exporting}>
						{#if exporting}<Spinner data-icon="inline-start" />{:else}<DownloadIcon data-icon="inline-start" />{/if}
						Download my data
					</Button>
				</div>

				<div class="flex items-start gap-3">
					<Checkbox id="delete-understood" bind:checked={understood} disabled={busy} class="mt-0.5" />
					<Label for="delete-understood" class="leading-snug font-normal">
						I understand that {mode === 'delete-org' ? `${info.org_name} and everything in it` : 'my account'} will be permanently
						deleted and can't be recovered.
					</Label>
				</div>

				<Field.Field>
					<Field.Label for="delete-confirm">
						Type <span class="font-mono font-semibold">{confirmWord}</span> to confirm
					</Field.Label>
					<Input
						id="delete-confirm"
						bind:value={confirmText}
						autocomplete="off"
						autocapitalize="off"
						spellcheck={false}
						disabled={busy}
					/>
				</Field.Field>

				{@render reauthField('delete')}

				{#if error}
					<Alert.Root variant="destructive">
						<CircleAlertIcon />
						<Alert.Title>{error}</Alert.Title>
					</Alert.Root>
				{/if}

				<Dialog.Footer>
					{#if info.role === 'owner' && !info.sole_member}
						<Button type="button" variant="outline" onclick={() => (step = 'choose')} disabled={busy}>Back</Button>
					{:else}
						<Dialog.Close class={buttonVariants({ variant: 'outline' })} disabled={busy}>Cancel</Dialog.Close>
					{/if}
					<Button type="submit" variant="destructive" disabled={!canDelete}>
						{#if busy}<Spinner data-icon="inline-start" />{/if}
						{mode === 'delete-org' ? 'Delete account and organisation' : 'Delete my account'}
					</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>
