<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import LockIcon from '@lucide/svelte/icons/lock';
	import PlugIcon from '@lucide/svelte/icons/plug';
	import { changePassword } from '$lib/api/auth';
	import {
		disconnectStorage,
		saveStorage,
		testStorage,
		type StorageInput,
		type StorageProvider
	} from '$lib/api/storage';
	import * as Alert from '$lib/components/ui/alert';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Switch } from '$lib/components/ui/switch';
	import FormSection from '$lib/components/app/form-section.svelte';
	import StorageProviderIcon from '$lib/components/app/storage-provider-icon.svelte';
	import { plural, timeAgo } from '$lib/format';
	import { session } from '$lib/session.svelte';
	import { storage } from '$lib/storage.svelte';
	import { cn } from '$lib/utils';

	interface Preset {
		label: string;
		endpoint: string;
		region: string;
		pathStyle: boolean;
		keysHelp: string;
	}

	const PRESETS: Record<StorageProvider, Preset> = {
		r2: {
			label: 'Cloudflare R2',
			endpoint: 'https://<ACCOUNT_ID>.r2.cloudflarestorage.com',
			region: 'auto',
			pathStyle: true,
			keysHelp:
				'Cloudflare dashboard → R2 → Manage API tokens → Create API token: permission "Object Read & Write", applied to this bucket only. Copy the Access Key ID, Secret Access Key and the S3 endpoint it shows (use the endpoint without the bucket name).'
		},
		b2: {
			label: 'Backblaze B2',
			endpoint: 'https://s3.us-west-004.backblazeb2.com',
			region: 'us-west-004',
			pathStyle: false,
			keysHelp:
				'B2 → Application Keys → Add a New Application Key with Read and Write access to this bucket only. keyID is the access key ID, applicationKey is the secret. The endpoint and region are on the bucket page (e.g. s3.us-west-004.backblazeb2.com → region us-west-004).'
		},
		s3: {
			label: 'AWS S3',
			endpoint: 'https://s3.us-east-1.amazonaws.com',
			region: 'us-east-1',
			pathStyle: false,
			keysHelp:
				'IAM → create a user with an access key and a policy allowing s3:GetObject, s3:PutObject and s3:DeleteObject on arn:aws:s3:::<bucket>/* (s3:ListBucket on the bucket is optional). Use the bucket\'s region in both the endpoint and Region.'
		},
		minio: {
			label: 'MinIO',
			endpoint: 'https://minio.example.com',
			region: 'us-east-1',
			pathStyle: true,
			keysHelp: 'MinIO Console → Access Keys → Create access key with a policy allowing read and write on this bucket.'
		},
		other: {
			label: 'Other S3-compatible',
			endpoint: 'https://',
			region: 'us-east-1',
			pathStyle: true,
			keysHelp: 'Use an access key with read and write access to this bucket only.'
		}
	};

	// Change password
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let changingPassword = $state(false);
	let passwordMessage = $state('');

	const newPasswordError = $derived(newPassword.length > 0 && newPassword.length < 8 ? 'At least 8 characters.' : '');
	const confirmPasswordError = $derived(
		confirmPassword.length > 0 && confirmPassword !== newPassword ? "Passwords don't match." : ''
	);
	const canChangePassword = $derived(
		!!currentPassword && newPassword.length >= 8 && confirmPassword === newPassword && !changingPassword
	);

	async function submitPassword(event: SubmitEvent) {
		event.preventDefault();
		if (!canChangePassword) return;
		changingPassword = true;
		passwordMessage = '';
		try {
			await changePassword(currentPassword, newPassword);
			currentPassword = newPassword = confirmPassword = '';
			toast.success('Password changed. Other sessions were signed out.');
		} catch (e) {
			passwordMessage = e instanceof Error ? e.message : 'Failed to change password';
		} finally {
			changingPassword = false;
		}
	}

	let provider = $state<StorageProvider>('r2');
	let endpoint = $state('');
	let region = $state('');
	let bucket = $state('');
	let pathStyle = $state(false);
	let accessKeyId = $state('');
	let secretAccessKey = $state('');

	let testing = $state(false);
	let saving = $state(false);
	let disconnecting = $state(false);
	let confirmDisconnect = $state(false);
	let message = $state<{ ok: boolean; text: string } | null>(null);

	const status = $derived(storage.status);
	const configured = $derived(!!status?.configured);
	const preset = $derived(PRESETS[provider]);

	// Fill the form from the saved settings once they arrive.
	let filledFrom: string | null = null;
	$effect(() => {
		const s = storage.status;
		if (!s) return;
		const key = JSON.stringify([s.provider, s.endpoint, s.region, s.bucket, s.path_style, s.verified_at]);
		if (key === filledFrom) return;
		filledFrom = key;
		if (s.configured && s.provider) {
			provider = s.provider;
			endpoint = s.endpoint ?? '';
			region = s.region ?? '';
			bucket = s.bucket ?? '';
			pathStyle = s.path_style;
		} else {
			choosePreset('r2');
		}
		accessKeyId = '';
		secretAccessKey = '';
	});

	function choosePreset(p: StorageProvider) {
		provider = p;
		const next = PRESETS[p];
		// Don't clobber an endpoint the user already typed for this provider.
		if (!configured || status?.provider !== p) {
			endpoint = next.endpoint.includes('<') || next.endpoint === 'https://' ? '' : next.endpoint;
			region = next.region;
		}
		pathStyle = next.pathStyle;
		message = null;
	}

	const input = $derived<StorageInput>({
		provider,
		endpoint: endpoint.trim(),
		region: region.trim(),
		bucket: bucket.trim(),
		path_style: pathStyle,
		access_key_id: accessKeyId.trim(),
		secret_access_key: secretAccessKey.trim()
	});

	const missingKeys = $derived(!configured && (!input.access_key_id || !input.secret_access_key));
	const canSubmit = $derived(!!input.endpoint && !!input.bucket && !missingKeys && !testing && !saving);

	async function test() {
		testing = true;
		message = null;
		try {
			await testStorage(input);
			message = { ok: true, text: 'Connection works: Fronko can read and write this bucket.' };
		} catch (e) {
			message = { ok: false, text: e instanceof Error ? e.message : 'Test failed' };
		} finally {
			testing = false;
		}
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		saving = true;
		message = null;
		try {
			const saved = await saveStorage(input);
			storage.set(saved);
			toast.success('Storage connected');
		} catch (e) {
			message = { ok: false, text: e instanceof Error ? e.message : 'Failed to save' };
		} finally {
			saving = false;
		}
	}

	async function disconnect() {
		disconnecting = true;
		try {
			await disconnectStorage();
			if (session.username) await storage.load(session.username, true);
			confirmDisconnect = false;
			toast.success('Storage disconnected');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to disconnect');
		} finally {
			disconnecting = false;
		}
	}
</script>

<svelte:head>
	<title>Settings · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1200px] flex-col px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-col gap-1 pb-2">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Settings</h1>
		<p class="text-muted-foreground text-sm">Your account, password, and storage for photos and brochures.</p>
	</header>

	<FormSection id="account" title="Account" description="How you sign in. Either works on the sign-in page.">
		<dl class="grid gap-5 text-sm sm:grid-cols-2">
			<div class="flex flex-col gap-1.5">
				<dt class="text-muted-foreground">Username</dt>
				<dd class="font-medium">{session.username}</dd>
			</div>
			<div class="flex flex-col gap-1.5">
				<dt class="text-muted-foreground">Email</dt>
				<dd class="flex flex-wrap items-center gap-2 font-medium">
					<span class="break-all">{session.email}</span>
					{#if session.emailVerified}
						<Badge variant="secondary" class="gap-1">
							<CircleCheckIcon class="size-3" />
							Verified
						</Badge>
					{/if}
				</dd>
			</div>
		</dl>
	</FormSection>

	<FormSection
		id="password"
		title="Password"
		description="Changing it signs you out on every other device. You stay signed in here."
	>
		<form onsubmit={submitPassword} class="flex flex-col gap-6">
			<Field.Group class="grid gap-5 sm:grid-cols-2">
				<Field.Field class="sm:col-span-2 sm:max-w-[calc(50%-0.625rem)]">
					<Field.Label for="current-password">Current password</Field.Label>
					<Input
						id="current-password"
						type="password"
						autocomplete="current-password"
						bind:value={currentPassword}
						disabled={changingPassword}
					/>
				</Field.Field>
				<Field.Field data-invalid={!!newPasswordError || undefined}>
					<Field.Label for="new-password">New password</Field.Label>
					<Input
						id="new-password"
						type="password"
						autocomplete="new-password"
						bind:value={newPassword}
						disabled={changingPassword}
						aria-invalid={!!newPasswordError || undefined}
					/>
					{#if newPasswordError}
						<Field.Error>{newPasswordError}</Field.Error>
					{:else}
						<Field.Description>At least 8 characters.</Field.Description>
					{/if}
				</Field.Field>
				<Field.Field data-invalid={!!confirmPasswordError || undefined}>
					<Field.Label for="confirm-password">Confirm new password</Field.Label>
					<Input
						id="confirm-password"
						type="password"
						autocomplete="new-password"
						bind:value={confirmPassword}
						disabled={changingPassword}
						aria-invalid={!!confirmPasswordError || undefined}
					/>
					{#if confirmPasswordError}
						<Field.Error>{confirmPasswordError}</Field.Error>
					{/if}
				</Field.Field>
			</Field.Group>

			{#if passwordMessage}
				<Alert.Root variant="destructive">
					<CircleAlertIcon />
					<Alert.Title>{passwordMessage}</Alert.Title>
				</Alert.Root>
			{/if}

			<div>
				<Button type="submit" disabled={!canChangePassword}>
					{#if changingPassword}<Spinner data-icon="inline-start" />{/if}
					Change password
				</Button>
			</div>
		</form>
	</FormSection>

	{#if storage.error}
		<Alert.Root variant="destructive" class="mt-6">
			<CircleAlertIcon />
			<Alert.Title>{storage.error}</Alert.Title>
		</Alert.Root>
	{:else if !status}
		<div class="mt-8 flex flex-col gap-4">
			<Skeleton class="h-8 w-56" />
			<Skeleton class="h-72 rounded-xl" />
		</div>
	{:else if !status.enabled}
		<div class="bg-card mt-8 flex flex-col gap-2 rounded-xl border p-6">
			<p class="font-medium">File storage isn't enabled on this server</p>
			<p class="text-muted-foreground max-w-2xl text-sm">
				Whoever runs this Fronko server needs to set <code class="bg-muted rounded px-1 py-0.5 text-xs">SECRETS_KEY</code>
				(a 32-byte key, e.g. <code class="bg-muted rounded px-1 py-0.5 text-xs">openssl rand -base64 32</code>). It's used to
				encrypt your storage keys.
			</p>
		</div>
	{:else}
		<FormSection
			id="storage"
			title="Storage"
			description="Photos and PDF brochures are stored in your own S3-compatible bucket. The bucket can stay private: visitors get short-lived links."
		>
			<form onsubmit={save} class="flex flex-col gap-6">
				<div
					class={cn(
						'flex items-center gap-3 rounded-xl border px-4 py-3 text-sm',
						configured ? 'bg-card' : 'bg-muted/40 border-dashed'
					)}
				>
					{#if configured}
						<span class="grid size-8 place-items-center rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400">
							<CircleCheckIcon class="size-4" />
						</span>
						<span class="flex min-w-0 flex-col">
							<span class="flex items-center gap-1.5 font-medium">
								<StorageProviderIcon provider={status.provider ?? 'other'} class="size-3.5 shrink-0" />
								Connected to {PRESETS[status.provider ?? 'other'].label}
							</span>
							<span class="text-muted-foreground truncate text-xs">
								{status.bucket} · {plural(status.file_count, 'file')}
								{#if status.verified_at}· verified {timeAgo(status.verified_at)}{/if}
							</span>
						</span>
					{:else}
						<span class="bg-muted text-muted-foreground grid size-8 place-items-center rounded-full">
							<PlugIcon class="size-4" />
						</span>
						<span class="flex flex-col">
							<span class="font-medium">Not connected</span>
							<span class="text-muted-foreground text-xs">Uploads are off until you connect a bucket.</span>
						</span>
					{/if}
				</div>

				<Field.Field>
					<Field.Label>Provider</Field.Label>
					<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-5" role="radiogroup" aria-label="Provider">
						{#each Object.entries(PRESETS) as [key, p] (key)}
							<button
								type="button"
								role="radio"
								aria-checked={provider === key}
								onclick={() => choosePreset(key as StorageProvider)}
								class={cn(
									'flex items-center gap-2 rounded-lg border px-3 py-2.5 text-left text-sm font-medium transition-colors',
									provider === key ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
								)}
							>
								<StorageProviderIcon provider={key as StorageProvider} class="size-4 shrink-0" />
								{p.label}
							</button>
						{/each}
					</div>
				</Field.Field>

				<Field.Group class="grid gap-5 sm:grid-cols-2">
					<Field.Field class="sm:col-span-2">
						<Field.Label for="endpoint">Endpoint</Field.Label>
						<Input id="endpoint" bind:value={endpoint} placeholder={preset.endpoint} class="font-mono text-sm" autocomplete="off" />
						<Field.Description>The S3 API URL only, without the bucket name or a path.</Field.Description>
					</Field.Field>
					<Field.Field>
						<Field.Label for="bucket">Bucket</Field.Label>
						<Input id="bucket" bind:value={bucket} placeholder="my-fronko-files" autocomplete="off" />
					</Field.Field>
					<Field.Field>
						<Field.Label for="region">Region</Field.Label>
						<Input id="region" bind:value={region} placeholder={preset.region} autocomplete="off" />
					</Field.Field>
					<Field.Field>
						<Field.Label for="access-key">Access key ID</Field.Label>
						<Input
							id="access-key"
							bind:value={accessKeyId}
							placeholder={configured ? `Saved · ends in ${status.access_key_hint}` : ''}
							autocomplete="off"
							spellcheck={false}
							class="font-mono text-sm"
						/>
					</Field.Field>
					<Field.Field>
						<Field.Label for="secret-key">Secret access key</Field.Label>
						<Input
							id="secret-key"
							type="password"
							bind:value={secretAccessKey}
							placeholder={configured ? 'Saved · leave blank to keep' : ''}
							autocomplete="new-password"
							class="font-mono text-sm"
						/>
					</Field.Field>
					<Field.Description class="sm:col-span-2">{preset.keysHelp}</Field.Description>
					<Field.Field orientation="horizontal" class="sm:col-span-2">
						<Field.Content>
							<Field.Label for="path-style">Path-style URLs</Field.Label>
							<Field.Description>On for Cloudflare R2, MinIO and most self-hosted providers. Off for AWS S3 and Backblaze B2.</Field.Description>
						</Field.Content>
						<Switch id="path-style" bind:checked={pathStyle} />
					</Field.Field>
				</Field.Group>

				<p class="text-muted-foreground flex items-start gap-2 text-xs">
					<LockIcon class="mt-px size-3.5 shrink-0" />
					Keys are encrypted before they're stored and are never shown again, not even to you. Use a key that can only
					access this bucket.
				</p>

				{#if message}
					<Alert.Root variant={message.ok ? 'default' : 'destructive'}>
						{#if message.ok}<CircleCheckIcon />{:else}<CircleAlertIcon />{/if}
						<Alert.Title>{message.text}</Alert.Title>
					</Alert.Root>
				{/if}

				<div class="flex flex-wrap items-center gap-2">
					<Button type="submit" disabled={!canSubmit}>
						{#if saving}<Spinner data-icon="inline-start" />{/if}
						{configured ? 'Save changes' : 'Connect storage'}
					</Button>
					<Button type="button" variant="outline" onclick={test} disabled={!canSubmit}>
						{#if testing}<Spinner data-icon="inline-start" />{/if}
						Test connection
					</Button>
					{#if configured}
						<Button type="button" variant="ghost" class="text-destructive ml-auto" onclick={() => (confirmDisconnect = true)}>
							Disconnect
						</Button>
					{/if}
				</div>
				{#if configured && (input.bucket !== status.bucket || input.endpoint !== status.endpoint)}
					<p class="text-sm text-amber-700 dark:text-amber-400">
						Files you've already uploaded stay in {status.bucket}. They'll keep loading only if these keys can still read
						that bucket.
					</p>
				{/if}
			</form>
		</FormSection>
	{/if}
</div>

<AlertDialog.Root bind:open={confirmDisconnect}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Disconnect storage?</AlertDialog.Title>
			<AlertDialog.Description>
				Your keys are deleted from Fronko. Nothing is removed from your bucket, but your
				{plural(status?.file_count ?? 0, 'file')} won't show on your cards until you connect again.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={disconnecting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={disconnect} disabled={disconnecting}>
				{#if disconnecting}<Spinner data-icon="inline-start" />{/if}
				Disconnect
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
