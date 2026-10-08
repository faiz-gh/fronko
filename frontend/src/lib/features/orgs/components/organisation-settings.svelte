<script lang="ts">
	import { toast } from 'svelte-sonner';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { confirmDialog } from '$lib/core/confirm.svelte';
	import { session } from '$lib/core/session.svelte';
	import { SLUG_PATTERN } from '$lib/features/cards/card';
	import QuotaInput from '$lib/features/files/components/quota-input.svelte';
	import { getOrganization, updateOrganization, updateOrgHandle, updateOrgPrivacy, type Organization } from '../api';

	// Organisation (owner edits; admins see it)
	let org = $state<Organization | null>(null);
	let orgName = $state('');
	let defaultQuota = $state<number | null>(null);
	let savingOrg = $state(false);
	$effect(() => {
		if (!session.isAdmin) return;
		getOrganization()
			.then((o) => {
				org = o;
				orgName = o.name;
				handle = o.handle;
				defaultQuota = o.default_quota_bytes;
				privacyUrl = o.privacy_url ?? '';
				retention = o.lead_retention_days === null ? 'forever' : String(o.lead_retention_days);
			})
			.catch(() => {});
	});
	const orgChanged = $derived(
		!!org && (orgName.trim() !== org.name || defaultQuota !== org.default_quota_bytes) && !!orgName.trim()
	);
	async function saveOrg(event: SubmitEvent) {
		event.preventDefault();
		if (!orgChanged) return;
		savingOrg = true;
		try {
			org = await updateOrganization(orgName.trim(), defaultQuota);
			orgName = org.name;
			session.orgName = org.name;
			toast.success('Organisation saved');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to save');
		} finally {
			savingOrg = false;
		}
	}

	// Privacy: the notice linked from contact forms, and how long leads are kept.
	const RETENTION_OPTIONS: { value: string; label: string }[] = [
		{ value: 'forever', label: 'Until someone deletes them' },
		{ value: '90', label: '3 months' },
		{ value: '180', label: '6 months' },
		{ value: '365', label: '1 year' },
		{ value: '730', label: '2 years' },
		{ value: '1095', label: '3 years' }
	];
	let privacyUrl = $state('');
	let retention = $state('forever');
	let savingPrivacy = $state(false);
	const retentionOptions = $derived(
		RETENTION_OPTIONS.some((o) => o.value === retention)
			? RETENTION_OPTIONS
			: [...RETENTION_OPTIONS, { value: retention, label: `${retention} days` }]
	);
	const retentionDays = $derived(retention === 'forever' ? null : Number(retention));
	const privacyUrlInvalid = $derived(!!privacyUrl.trim() && !/^https?:\/\/[^\s/]+\.[^\s]+$/i.test(privacyUrl.trim()));
	const privacyChanged = $derived(
		!!org && ((privacyUrl.trim() || null) !== org.privacy_url || retentionDays !== org.lead_retention_days)
	);
	async function savePrivacy(event: SubmitEvent) {
		event.preventDefault();
		if (!privacyChanged || privacyUrlInvalid) return;
		const shorter =
			retentionDays !== null && (org?.lead_retention_days === null || retentionDays < (org?.lead_retention_days ?? 0));
		if (shorter) {
			const ok = await confirmDialog({
				title: 'Delete older leads?',
				description: `Leads older than ${retentionOptions.find((o) => o.value === retention)?.label.toLowerCase()} will be deleted automatically, starting within a few hours. Export any you want to keep first. This can't be undone.`,
				confirmLabel: 'Save and delete older leads',
				destructive: true
			});
			if (!ok) return;
		}
		savingPrivacy = true;
		try {
			org = await updateOrgPrivacy(privacyUrl.trim() || null, retentionDays);
			privacyUrl = org.privacy_url ?? '';
			toast.success('Privacy settings saved');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to save');
		} finally {
			savingPrivacy = false;
		}
	}

	// Card link handle: every card's link is /p/{handle}/{slug}. It's printed on
	// QR codes and written to NFC cards, so changing it takes a confirmation.
	let handle = $state('');
	let savingHandle = $state(false);
	let confirmHandle = $state(false);
	const handleInvalid = $derived(handle.length < 3 || handle.length > 32 || !SLUG_PATTERN.test(handle));
	const handleChanged = $derived(!!org && handle !== org.handle);
	async function saveHandle() {
		if (!handleChanged || handleInvalid) return;
		savingHandle = true;
		try {
			org = await updateOrgHandle(handle);
			handle = org.handle;
			session.orgHandle = org.handle;
			confirmHandle = false;
			toast.success('Card link handle changed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to save');
		} finally {
			savingHandle = false;
		}
	}
</script>

<FormSection
	id="organisation"
	title="Organisation"
	description={session.isOwner
		? 'Shown to your team. New people start with the default storage limit; you can change it per person.'
		: 'Only the owner can change these.'}
>
	{#if !org}
		<Skeleton class="h-24 rounded-xl" />
	{:else}
		<form onsubmit={saveOrg} class="flex flex-col gap-6">
			<Field.Group class="gap-5">
				<Field.Field class="sm:max-w-sm">
					<Field.Label for="org-name">Name</Field.Label>
					<Input id="org-name" bind:value={orgName} maxlength={80} disabled={!session.isOwner || savingOrg} required />
				</Field.Field>
				<Field.Field>
					<Field.Label for="org-quota">Default storage per user</Field.Label>
					<QuotaInput id="org-quota" bind:value={defaultQuota} disabled={!session.isOwner || savingOrg} />
					<Field.Description>
						For each person's own files. Shared and organisation files don't count. Changing this doesn't affect
						existing users.
					</Field.Description>
				</Field.Field>
			</Field.Group>
			{#if session.isOwner}
				<div>
					<Button type="submit" disabled={!orgChanged || savingOrg}>
						{#if savingOrg}<Spinner data-icon="inline-start" />{/if}
						Save organisation
					</Button>
				</div>
			{/if}
		</form>
	{/if}
</FormSection>
<FormSection
	id="card-links"
	title="Card links"
	description="Every card's link starts with this handle, so card names only have to be unique inside {session.orgName}."
>
	{#if !org}
		<Skeleton class="h-16 rounded-xl" />
	{:else}
		<form
			onsubmit={(e) => {
				e.preventDefault();
				if (handleChanged && !handleInvalid) confirmHandle = true;
			}}
			class="flex flex-col gap-6"
		>
			<Field.Field data-invalid={(handleChanged && handleInvalid) || undefined} class="sm:max-w-md">
				<Field.Label for="org-handle">Handle</Field.Label>
				<div class="flex items-stretch">
					<span
						class="text-muted-foreground bg-muted flex items-center rounded-l-lg border border-r-0 px-3 font-mono text-xs"
					>
						<span class="max-sm:hidden">{location.host}</span>/p/
					</span>
					<Input
						id="org-handle"
						class="rounded-l-none font-mono"
						value={handle}
						oninput={(e) => (handle = e.currentTarget.value.toLowerCase())}
						maxlength={32}
						aria-invalid={(handleChanged && handleInvalid) || undefined}
						disabled={savingHandle}
						required
					/>
				</div>
				{#if handleChanged && handleInvalid}
					<Field.Error>3–32 characters: lowercase letters, numbers and hyphens.</Field.Error>
				{:else if handleChanged}
					<Field.Description class="text-amber-700 dark:text-amber-400">
						Changing this breaks every card link already shared, on QR codes, NFC cards and email signatures. The old
						handle is released, so another organisation could take it.
					</Field.Description>
				{:else}
					<Field.Description>
						Cards look like <span class="font-mono">/p/{org.handle}/jane-doe</span>. Treat it as permanent once cards
						are printed or NFC cards are written.
					</Field.Description>
				{/if}
			</Field.Field>
			<div>
				<Button type="submit" variant="outline" disabled={!handleChanged || handleInvalid || savingHandle}>
					Change handle
				</Button>
			</div>
		</form>
	{/if}
</FormSection>

<FormSection
	id="privacy"
	title="Privacy"
	description="What visitors are told when they leave their details, and how long {session.orgName} keeps them."
>
	{#if !org}
		<Skeleton class="h-24 rounded-xl" />
	{:else}
		<form onsubmit={savePrivacy} class="flex flex-col gap-6">
			<Field.Group class="gap-5">
				<Field.Field data-invalid={privacyUrlInvalid || undefined} class="sm:max-w-md">
					<Field.Label for="privacy-url">Privacy notice</Field.Label>
					<Input
						id="privacy-url"
						type="url"
						inputmode="url"
						placeholder="https://example.com/privacy"
						bind:value={privacyUrl}
						disabled={savingPrivacy}
						aria-invalid={privacyUrlInvalid || undefined}
					/>
					{#if privacyUrlInvalid}
						<Field.Error>Enter a full link starting with https://</Field.Error>
					{:else}
						<Field.Description>
							Linked from every card's contact form and footer, so visitors can read how you use their details. Your
							organisation is responsible for the leads it collects.
						</Field.Description>
					{/if}
				</Field.Field>
				<Field.Field class="sm:max-w-md">
					<Field.Label for="lead-retention">Keep leads for</Field.Label>
					<Select.Root type="single" bind:value={retention} disabled={savingPrivacy}>
						<Select.Trigger id="lead-retention" class="w-full">
							{retentionOptions.find((o) => o.value === retention)?.label}
						</Select.Trigger>
						<Select.Content>
							{#each retentionOptions as o (o.value)}
								<Select.Item value={o.value} label={o.label}>{o.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
					<Field.Description>
						Older leads are deleted automatically, from every card. Keep them only as long as you need them.
					</Field.Description>
				</Field.Field>
			</Field.Group>
			<div>
				<Button type="submit" disabled={!privacyChanged || privacyUrlInvalid || savingPrivacy}>
					{#if savingPrivacy}<Spinner data-icon="inline-start" />{/if}
					Save privacy settings
				</Button>
			</div>
		</form>
	{/if}
</FormSection>

<AlertDialog.Root bind:open={confirmHandle}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Change every card link?</AlertDialog.Title>
			<AlertDialog.Description>
				Links move from <span class="font-mono">/p/{org?.handle}/…</span> to
				<span class="font-mono">/p/{handle}/…</span>. QR codes, NFC cards and email signatures that use the old links
				stop working until they're replaced, and
				<span class="font-mono">{org?.handle}</span> becomes free for anyone to take.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={savingHandle}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={saveHandle} disabled={savingHandle}>
				{#if savingHandle}<Spinner data-icon="inline-start" />{/if}
				Change handle
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
