<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CheckIcon from '@lucide/svelte/icons/check';
	import ImagesIcon from '@lucide/svelte/icons/images';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import XIcon from '@lucide/svelte/icons/x';
	import { ACCEPT, checkUpload, fileUrl, uploadFile, type LibraryFile } from '$lib/api/files';
	import { updateBranding, type LogoPolicy, type OrgBranding } from '$lib/api/org';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';
	import { branding } from '$lib/branding.svelte';
	import { safeUrl } from '$lib/card/card';
	import { SIGNATURE_TEMPLATES, SIGNATURE_TEMPLATE_KEYS } from '$lib/signature/templates';
	import { storage } from '$lib/storage.svelte';
	import { cn } from '$lib/utils';
	import FilePickerDialog from './file-picker-dialog.svelte';

	type Draft = Omit<OrgBranding, 'name'>;
	type Slot = 'logo' | 'banner';

	const MAX_DISCLAIMER = 1000;

	let draft = $state<Draft | null>(null);
	let snapshot = $state('');
	let saving = $state(false);
	let progress = $state<Record<Slot, number | null>>({ logo: null, banner: null });
	let pickerFor = $state<Slot | null>(null);
	let pickerOpen = $state(false);
	let inputs = $state<Record<Slot, HTMLInputElement | null>>({ logo: null, banner: null });

	function load(b: OrgBranding) {
		draft = {
			logo_file: b.logo_file,
			logo_policy: b.logo_policy,
			signature: {
				locked_template: b.signature.locked_template ?? '',
				brand_color: b.signature.brand_color ?? '',
				disclaimer: b.signature.disclaimer ?? '',
				banner_file: b.signature.banner_file ?? '',
				banner_url: b.signature.banner_url ?? ''
			}
		};
		snapshot = JSON.stringify(draft);
	}

	$effect(() => {
		if (branding.value && !draft) load(branding.value);
	});

	const dirty = $derived(!!draft && JSON.stringify(draft) !== snapshot);
	const colorInvalid = $derived(!!draft?.signature.brand_color && !/^#[0-9a-f]{6}$/i.test(draft.signature.brand_color));
	const bannerUrlInvalid = $derived(!!draft?.signature.banner_url && !safeUrl(draft.signature.banner_url));
	const canSave = $derived(dirty && !saving && !colorInvalid && !bannerUrlInvalid);

	function current(slot: Slot): string {
		if (!draft) return '';
		return (slot === 'logo' ? draft.logo_file : draft.signature.banner_file) ?? '';
	}

	function setSlot(slot: Slot, id: string) {
		if (!draft) return;
		if (slot === 'logo') draft.logo_file = id || null;
		else draft.signature.banner_file = id;
	}

	async function upload(slot: Slot, file: File | undefined) {
		if (!file) return;
		const problem = checkUpload(file, 'image');
		if (problem) {
			toast.error(problem);
			return;
		}
		progress[slot] = 0;
		try {
			const uploaded = await uploadFile(file, {
				area: 'org',
				title: slot === 'logo' ? 'Organisation logo' : 'Signature banner',
				onProgress: (p) => (progress[slot] = p)
			});
			setSlot(slot, uploaded.id);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Upload failed');
		} finally {
			progress[slot] = null;
		}
	}

	function choose(slot: Slot) {
		pickerFor = slot;
		pickerOpen = true;
	}

	function picked(file: LibraryFile) {
		if (!pickerFor) return;
		if (file.area === 'personal') {
			toast.error('Pick an organisation or shared image. Personal files can’t be used for branding.');
			return;
		}
		setSlot(pickerFor, file.id);
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (!draft || !canSave) return;
		saving = true;
		try {
			const saved = await updateBranding($state.snapshot(draft));
			branding.set(saved);
			load(saved);
			toast.success('Branding saved');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to save');
		} finally {
			saving = false;
		}
	}

	const POLICIES: { value: LogoPolicy; label: string; description: string }[] = [
		{ value: 'required', label: 'Required', description: 'Every card and email signature shows the logo.' },
		{ value: 'optional', label: 'Employee chooses', description: 'Each card can show or hide it. Shown by default.' }
	];
</script>

{#snippet imageSlot(slot: Slot, label: string, hint: string)}
	{@const id = current(slot)}
	<Field.Field>
		<Field.Label>{label}</Field.Label>
		{#if id}
			<div class="grid max-w-md grid-cols-2 overflow-hidden rounded-xl border">
				<div class="grid h-24 place-items-center bg-white p-3">
					<img src={fileUrl(id)} alt="{label} on light" class="max-h-full max-w-full object-contain" />
				</div>
				<div class="grid h-24 place-items-center bg-neutral-900 p-3">
					<img src={fileUrl(id)} alt="{label} on dark" class="max-h-full max-w-full object-contain" />
				</div>
			</div>
		{/if}
		<div class="flex flex-wrap items-center gap-2">
			<Button
				variant="outline"
				size="sm"
				onclick={() => inputs[slot]?.click()}
				disabled={!storage.ready || progress[slot] !== null}
			>
				<UploadIcon data-icon="inline-start" />
				{progress[slot] !== null ? `Uploading ${Math.round((progress[slot] ?? 0) * 100)}%` : 'Upload'}
			</Button>
			<input
				bind:this={inputs[slot]}
				type="file"
				accept={ACCEPT.image}
				class="hidden"
				onchange={(e) => {
					upload(slot, e.currentTarget.files?.[0]);
					e.currentTarget.value = '';
				}}
			/>
			<Button variant="outline" size="sm" onclick={() => choose(slot)} disabled={!storage.ready}>
				<ImagesIcon data-icon="inline-start" />
				Choose from files
			</Button>
			{#if id}
				<Button variant="ghost" size="sm" onclick={() => setSlot(slot, '')}>
					<XIcon data-icon="inline-start" />
					Remove
				</Button>
			{/if}
		</div>
		<Field.Description>
			{storage.ready ? hint : 'Connect storage below to upload images.'}
		</Field.Description>
	</Field.Field>
{/snippet}

{#if !draft}
	<Skeleton class="h-48 rounded-xl" />
{:else}
	<form onsubmit={save} class="flex flex-col gap-8">
		<Field.Group class="gap-6">
			{@render imageSlot('logo', 'Logo', 'PNG with a transparent background works best. Shown on cards and email signatures.')}

			<Field.Field>
				<Field.Label id="logo-policy-label">Logo on cards and signatures</Field.Label>
				<div class="grid gap-2 sm:grid-cols-2" role="radiogroup" aria-labelledby="logo-policy-label">
					{#each POLICIES as p (p.value)}
						<button
							type="button"
							role="radio"
							aria-checked={draft.logo_policy === p.value}
							onclick={() => draft && (draft.logo_policy = p.value)}
							class={cn(
								'flex flex-col gap-0.5 rounded-xl border p-3 text-left transition-colors',
								draft.logo_policy === p.value ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
							)}
						>
							<span class="text-sm font-medium">{p.label}</span>
							<span class="text-muted-foreground text-xs leading-snug">{p.description}</span>
						</button>
					{/each}
				</div>
				{#if !draft.logo_file}
					<Field.Description>Upload a logo for this to take effect.</Field.Description>
				{/if}
			</Field.Field>
		</Field.Group>

		<Field.Group class="gap-6">
			<Field.Field>
				<Field.Label id="sig-template-label">Email signature template</Field.Label>
				<div class="grid gap-2 sm:grid-cols-3" role="radiogroup" aria-labelledby="sig-template-label">
					{#each [{ key: '', label: 'Employees choose', description: 'Anyone can pick any template.' }, ...SIGNATURE_TEMPLATE_KEYS.map((k) => ({ key: k, ...SIGNATURE_TEMPLATES[k] }))] as t (t.key)}
						{@const active = (draft.signature.locked_template ?? '') === t.key}
						<button
							type="button"
							role="radio"
							aria-checked={active}
							onclick={() => draft && (draft.signature.locked_template = t.key)}
							class={cn(
								'flex flex-col gap-0.5 rounded-xl border p-3 text-left transition-colors',
								active ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
							)}
						>
							<span class="flex items-center gap-1.5 text-sm font-medium">
								{t.key ? `Lock to ${t.label}` : t.label}
								{#if active}<CheckIcon class="size-3.5" />{/if}
							</span>
							<span class="text-muted-foreground text-xs leading-snug">{t.description}</span>
						</button>
					{/each}
				</div>
			</Field.Field>

			<Field.Field data-invalid={colorInvalid || undefined} class="sm:max-w-sm">
				<Field.Label for="brand-color">Brand colour</Field.Label>
				<div class="flex items-center gap-2">
					<input
						type="color"
						aria-label="Pick brand colour"
						class="size-9 shrink-0 cursor-pointer rounded-lg border bg-transparent p-1"
						value={colorInvalid || !draft.signature.brand_color ? '#4f46e5' : draft.signature.brand_color}
						oninput={(e) => draft && (draft.signature.brand_color = e.currentTarget.value)}
					/>
					<Input
						id="brand-color"
						class="font-mono"
						placeholder="Each card’s accent"
						bind:value={draft.signature.brand_color}
						aria-invalid={colorInvalid || undefined}
					/>
					{#if draft.signature.brand_color}
						<Button variant="ghost" size="icon" aria-label="Clear brand colour" onclick={() => draft && (draft.signature.brand_color = '')}>
							<XIcon />
						</Button>
					{/if}
				</div>
				{#if colorInvalid}
					<Field.Error>Use a hex colour like #1a2b3c.</Field.Error>
				{:else}
					<Field.Description>Replaces each card’s accent colour in signatures. Leave empty to keep them.</Field.Description>
				{/if}
			</Field.Field>

			<Field.Field>
				<Field.Label for="disclaimer">Disclaimer</Field.Label>
				<Textarea
					id="disclaimer"
					rows={3}
					maxlength={MAX_DISCLAIMER}
					placeholder="This email and any attachments are confidential…"
					bind:value={draft.signature.disclaimer}
				/>
				<Field.Description>
					Small print under every signature. {(draft.signature.disclaimer ?? '').length}/{MAX_DISCLAIMER}
				</Field.Description>
			</Field.Field>

			{@render imageSlot('banner', 'Banner', 'An image under every signature, such as an event or a promotion. Shown up to 400 × 120 px.')}

			<Field.Field data-invalid={bannerUrlInvalid || undefined}>
				<Field.Label for="banner-url">Banner link</Field.Label>
				<Input
					id="banner-url"
					type="url"
					placeholder="https://example.com/offer"
					bind:value={draft.signature.banner_url}
					aria-invalid={bannerUrlInvalid || undefined}
				/>
				{#if bannerUrlInvalid}
					<Field.Error>Enter a full link starting with https://</Field.Error>
				{:else}
					<Field.Description>Where clicking the banner goes. Optional.</Field.Description>
				{/if}
			</Field.Field>
		</Field.Group>

		<div>
			<Button type="submit" disabled={!canSave}>
				{#if saving}<Spinner data-icon="inline-start" />{/if}
				Save branding
			</Button>
		</div>
	</form>

	<FilePickerDialog
		bind:open={pickerOpen}
		kind="image"
		title={pickerFor === 'banner' ? 'Choose a banner' : 'Choose a logo'}
		selected={pickerFor ? [current(pickerFor)].filter(Boolean) : []}
		onselect={picked}
	/>
{/if}
