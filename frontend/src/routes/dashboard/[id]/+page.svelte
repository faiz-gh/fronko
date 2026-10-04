<script lang="ts">
	import { beforeNavigate, goto } from '$app/navigation';
	import { page } from '$app/state';
	import { fly } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import CheckIcon from '@lucide/svelte/icons/check';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import EyeIcon from '@lucide/svelte/icons/eye';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import ImagesIcon from '@lucide/svelte/icons/images';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import QrCodeIcon from '@lucide/svelte/icons/qr-code';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import XIcon from '@lucide/svelte/icons/x';
	import { ACCEPT, checkUpload, formatBytes, uploadFile, type LibraryFile, type PublicFile } from '$lib/api/files';
	import { getMyProfile, getProfileBySlug, updateProfile, type Profile } from '$lib/api/profile';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Switch } from '$lib/components/ui/switch';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Textarea } from '$lib/components/ui/textarea';
	import BrandIcon from '$lib/components/app/brand-icon.svelte';
	import CardAvatar from '$lib/components/app/card-avatar.svelte';
	import DeleteCardDialog from '$lib/components/app/delete-card-dialog.svelte';
	import FilePickerDialog from '$lib/components/app/file-picker-dialog.svelte';
	import FormSection from '$lib/components/app/form-section.svelte';
	import ImageCropDialog from '$lib/components/app/image-crop-dialog.svelte';
	import LeadsTable from '$lib/components/app/leads-table.svelte';
	import PhoneInput from '$lib/components/app/phone-input.svelte';
	import ProfileCard from '$lib/components/app/profile-card.svelte';
	import QrCode from '$lib/components/app/qr-code.svelte';
	import QrDialog from '$lib/components/app/qr-dialog.svelte';
	import {
		ACCENTS,
		coverSrc,
		detectCalendar,
		isValidSlug,
		MAX_DOCUMENTS,
		newId,
		normalizeCard,
		publicUrl,
		safeUrl,
		type AccentKey,
		type CardData
	} from '$lib/card/card';
	import { downloadQrPng, downloadQrSvg } from '$lib/card/qr';
	import { cards } from '$lib/cards.svelte';
	import { isValidPhone } from '$lib/phone';
	import { storage } from '$lib/storage.svelte';
	import { timeAgo } from '$lib/format';
	import { cn } from '$lib/utils';

	const MAX_LINKS = 12;

	const profileId = $derived(Number(page.params.id));

	let profile = $state<Profile | null>(null);
	let card = $state<CardData | null>(null);
	let slug = $state('');
	let loadError = $state('');
	let saving = $state(false);
	let saveError = $state('');
	let qrOpen = $state(false);
	let previewOpen = $state(false);
	let previewMode = $state<'card' | 'qr'>('card');
	let qrMarkup = $state('');
	let deleteTarget = $state<Profile | null>(null);
	let tab = $state(page.url.searchParams.get('tab') === 'leads' ? 'leads' : 'card');

	// The last saved state; anything different is an unsaved change.
	let snapshot = $state('');
	const serialized = $derived(JSON.stringify({ slug, card }));
	const dirty = $derived(card !== null && serialized !== snapshot);

	const slugInvalid = $derived(!isValidSlug(slug));
	// A pasted URL only matters when no library photo is set.
	const avatarInvalid = $derived(!!card && !card.avatar_file && !!card.avatar_url.trim() && !safeUrl(card.avatar_url));

	// Metadata for library files this card uses: names and sizes for the editor and preview.
	let fileMeta = $state<Record<string, PublicFile>>({});
	let photoPickerOpen = $state(false);
	let docPickerOpen = $state(false);
	let photoProgress = $state<number | null>(null);
	let photoInput: HTMLInputElement | undefined = $state();
	let coverPickerOpen = $state(false);
	let coverProgress = $state<number | null>(null);
	let coverInput: HTMLInputElement | undefined = $state();
	// A freshly picked image waiting in the crop dialog.
	let cropPhoto = $state<File | null>(null);
	let cropCover = $state<File | null>(null);
	const websiteInvalid = $derived(!!card?.website.trim() && !safeUrl(card.website));
	const emailInvalid = $derived(!!card?.email.trim() && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(card.email.trim()));
	const phoneInvalid = $derived(!!card?.phone_number && !isValidPhone(card.phone_country_code, card.phone_number));
	const calendarInvalid = $derived(!!card?.calendar_url.trim() && !safeUrl(card.calendar_url));
	const calendarProvider = $derived(card ? detectCalendar(card.calendar_url) : null);
	const cover = $derived(card ? coverSrc(card) : null);
	const invalid = $derived(
		slugInvalid || avatarInvalid || websiteInvalid || emailInvalid || phoneInvalid || calendarInvalid
	);
	const canSave = $derived(dirty && !saving && !invalid);

	// Kept current by the cards store (the single-card endpoint doesn't count leads).
	const leadCount = $derived(cards.list?.find((p) => p.id === profileId)?.lead_count ?? 0);

	const reduceMotion =
		typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

	// Reload when the sidebar switches to another card (same route, new id).
	$effect(() => {
		const id = profileId;
		profile = null;
		card = null;
		snapshot = '';
		saveError = '';
		if (Number.isInteger(id) && id > 0) load(id);
		else loadError = 'Card not found';
	});

	async function load(id: number) {
		loadError = '';
		try {
			const p = await getMyProfile(id);
			if (id !== profileId) return;
			profile = p;
			card = normalizeCard(p.data);
			slug = p.slug;
			snapshot = JSON.stringify({ slug, card });
			// The public endpoint already resolves the files a card references.
			getProfileBySlug(p.slug)
				.then((pub) => {
					if (id === profileId) for (const f of pub.files) fileMeta[f.id] = f;
				})
				.catch(() => {});
		} catch (e) {
			loadError = e instanceof Error ? e.message : 'Failed to load card';
		}
	}

	async function save() {
		if (!card || !canSave) return;
		saving = true;
		saveError = '';
		try {
			// Drop empty link rows rather than persisting them.
			card.links = card.links.filter((l) => l.url.trim());
			const p = await updateProfile(profileId, slug, $state.snapshot(card));
			profile = { ...p, lead_count: leadCount };
			cards.upsert(profile);
			slug = p.slug;
			snapshot = JSON.stringify({ slug, card });
			toast.success('Changes saved');
		} catch (e) {
			saveError = e instanceof Error ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}

	function discard() {
		const saved = JSON.parse(snapshot) as { slug: string; card: CardData };
		slug = saved.slug;
		card = saved.card;
		saveError = '';
	}

	function onKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key === 's') {
			e.preventDefault();
			save();
		}
	}

	beforeNavigate((nav) => {
		if (dirty && !confirm('You have unsaved changes. Leave anyway?')) nav.cancel();
	});

	function addLink() {
		card?.links.push({ id: newId(), label: '', url: '' });
	}

	function moveLink(index: number, delta: number) {
		if (!card) return;
		const target = index + delta;
		if (target < 0 || target >= card.links.length) return;
		const [item] = card.links.splice(index, 1);
		card.links.splice(target, 0, item);
	}

	async function copyLink() {
		if (!profile) return;
		try {
			await navigator.clipboard.writeText(publicUrl(profile.slug));
			toast.success('Link copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}

	function remember(file: LibraryFile) {
		fileMeta[file.id] = { id: file.id, kind: file.kind, name: file.title || file.name, size_bytes: file.size_bytes };
	}

	function usePhoto(file: LibraryFile) {
		if (!card) return;
		remember(file);
		card.avatar_file = file.id;
		card.avatar_url = '';
	}

	/** Checks a picked image, then opens the crop dialog for it. */
	function pickImage(file: File, target: 'photo' | 'cover') {
		const problem = checkUpload(file, 'image');
		if (problem) {
			toast.error(problem);
			return;
		}
		if (target === 'photo') cropPhoto = file;
		else cropCover = file;
	}

	async function uploadPhoto(file: File) {
		photoProgress = 0;
		try {
			usePhoto(await uploadFile(file, { onProgress: (p) => (photoProgress = p) }));
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Upload failed');
		} finally {
			photoProgress = null;
		}
	}

	function useCover(file: LibraryFile) {
		if (!card) return;
		remember(file);
		card.cover_file = file.id;
	}

	async function uploadCover(file: File) {
		coverProgress = 0;
		try {
			useCover(await uploadFile(file, { onProgress: (p) => (coverProgress = p) }));
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Upload failed');
		} finally {
			coverProgress = null;
		}
	}

	function removePhoto() {
		if (!card) return;
		card.avatar_file = '';
		card.avatar_url = '';
	}

	function addDocument(file: LibraryFile) {
		if (!card || card.documents.length >= MAX_DOCUMENTS) return;
		remember(file);
		if (card.documents.some((d) => d.file === file.id)) {
			toast.info('That brochure is already on this card');
			return;
		}
		card.documents.push({ id: newId(), file: file.id, title: file.title || file.name.replace(/\.pdf$/i, '') });
	}

	function moveDocument(index: number, delta: number) {
		if (!card) return;
		const target = index + delta;
		if (target < 0 || target >= card.documents.length) return;
		const [item] = card.documents.splice(index, 1);
		card.documents.splice(target, 0, item);
	}

	async function onDeleted() {
		card = null; // nothing left to lose, so skip the unsaved-changes prompt
		await goto('/dashboard');
	}
</script>

<svelte:window onkeydown={onKeydown} />
<svelte:head>
	<title>{card?.name || profile?.slug || 'Edit card'} · Fronko</title>
</svelte:head>

{#if loadError}
	<div class="grid min-h-[70svh] place-items-center px-4">
		<div class="flex max-w-sm flex-col items-center gap-3 text-center">
			<p class="text-lg font-semibold">{loadError}</p>
			<p class="text-muted-foreground text-sm">It may have been deleted, or it belongs to another account.</p>
			<Button variant="outline" href="/dashboard" class="mt-2">
				<ArrowLeftIcon data-icon="inline-start" />
				Back to overview
			</Button>
		</div>
	</div>
{:else if !card || !profile}
	<div class="flex flex-col gap-8 px-4 py-6 sm:px-8 lg:px-10">
		<div class="flex items-center gap-3">
			<Skeleton class="size-10 rounded-full" />
			<div class="flex flex-col gap-2">
				<Skeleton class="h-4 w-40" />
				<Skeleton class="h-3 w-28" />
			</div>
		</div>
		<div class="grid gap-10 xl:grid-cols-[minmax(0,1fr)_460px]">
			<div class="flex flex-col gap-6">
				{#each [1, 2, 3] as i (i)}
					<Skeleton class="h-40 rounded-xl" />
				{/each}
			</div>
			<Skeleton class="hidden h-[600px] rounded-3xl xl:block" />
		</div>
	</div>
{:else}
	<Tabs.Root bind:value={tab} class="gap-0">
		<header class="border-b px-4 pt-5 sm:px-8 lg:px-10 lg:pt-6">
			<div class="flex flex-wrap items-center gap-x-4 gap-y-3">
				<div class="flex min-w-0 flex-1 items-center gap-3">
					<CardAvatar {card} fallback={profile.slug} class="size-10 text-sm" />
					<div class="flex min-w-0 flex-col">
						<h1 class="truncate text-lg font-semibold tracking-tight sm:text-xl">{card.name || profile.slug}</h1>
						<div class="text-muted-foreground flex min-w-0 items-center gap-2 text-xs">
							<a
								href="/p/{profile.slug}"
								target="_blank"
								class="hover:text-foreground flex min-w-0 items-center gap-1 font-mono"
							>
								<span class="truncate">/p/{profile.slug}</span>
								<ExternalLinkIcon class="size-3 shrink-0" />
							</a>
							<span aria-hidden="true">·</span>
							{#if dirty}
								<span class="flex shrink-0 items-center gap-1.5 text-amber-600 dark:text-amber-400">
									<span class="size-1.5 rounded-full bg-current"></span>
									Unsaved changes
								</span>
							{:else}
								<span class="flex shrink-0 items-center gap-1">
									<CheckIcon class="size-3" />
									Saved {timeAgo(profile.updated_at)}
								</span>
							{/if}
						</div>
					</div>
				</div>

				<div class="flex items-center gap-2">
					<Button variant="outline" class="xl:hidden" onclick={() => (previewOpen = true)}>
						<EyeIcon data-icon="inline-start" />
						Preview
					</Button>
					<Button variant="outline" onclick={copyLink} class="max-sm:hidden">
						<CopyIcon data-icon="inline-start" />
						Copy link
					</Button>
					<DropdownMenu.Root>
						<DropdownMenu.Trigger class={buttonVariants({ variant: 'outline', size: 'icon' })} aria-label="More actions">
							<EllipsisIcon />
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end" class="w-52">
							<DropdownMenu.Group>
								<DropdownMenu.Item onSelect={() => window.open(`/p/${profile!.slug}`, '_blank')}>
									<ExternalLinkIcon />
									View public page
								</DropdownMenu.Item>
								<DropdownMenu.Item onSelect={copyLink}>
									<CopyIcon />
									Copy link
								</DropdownMenu.Item>
								<DropdownMenu.Item onSelect={() => (qrOpen = true)}>
									<QrCodeIcon />
									QR code
								</DropdownMenu.Item>
							</DropdownMenu.Group>
							<DropdownMenu.Separator />
							<DropdownMenu.Group>
								<DropdownMenu.Item variant="destructive" onSelect={() => (deleteTarget = profile)}>
									<Trash2Icon />
									Delete card
								</DropdownMenu.Item>
							</DropdownMenu.Group>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				</div>
			</div>

			<Tabs.List variant="line" class="mt-4 -mb-px h-10 gap-4 p-0">
				<Tabs.Trigger value="card" class="flex-none px-0.5">Card</Tabs.Trigger>
				<Tabs.Trigger value="leads" class="flex-none px-0.5">
					Leads
					<span class="bg-muted text-muted-foreground tabular rounded-full px-1.5 py-0.5 text-[11px] leading-none">
						{leadCount}
					</span>
				</Tabs.Trigger>
			</Tabs.List>
		</header>

		<Tabs.Content value="card">
			<div class="grid xl:grid-cols-[minmax(0,1fr)_460px] 2xl:grid-cols-[minmax(0,1fr)_540px]">
				<div class="min-w-0 px-4 pb-10 sm:px-8 lg:px-10">
					<FormSection id="profile" title="Profile" description="Who you are. Shown at the top of your card.">
						<Field.Group class="grid gap-5 sm:grid-cols-2">
							<Field.Field class="sm:col-span-2">
								<Field.Label for="name">Full name</Field.Label>
								<Input id="name" bind:value={card.name} placeholder="Jane Doe" maxlength={80} />
							</Field.Field>
							<Field.Field>
								<Field.Label for="title">Job title</Field.Label>
								<Input id="title" bind:value={card.title} placeholder="Product Designer" maxlength={80} />
							</Field.Field>
							<Field.Field>
								<Field.Label for="company">Company</Field.Label>
								<Input id="company" bind:value={card.company} placeholder="Acme Inc." maxlength={80} />
							</Field.Field>
							<Field.Field class="sm:col-span-2">
								<Field.Label for="location">Location</Field.Label>
								<Input id="location" bind:value={card.location} placeholder="Berlin, Germany" maxlength={80} />
							</Field.Field>
							<Field.Field class="sm:col-span-2">
								<div class="flex items-baseline justify-between">
									<Field.Label for="bio">Short bio</Field.Label>
									<span class="text-muted-foreground tabular text-xs">{card.bio.length}/280</span>
								</div>
								<Textarea
									id="bio"
									bind:value={card.bio}
									rows={3}
									maxlength={280}
									placeholder="A sentence or two about what you do."
								/>
							</Field.Field>
							<Field.Field class="sm:col-span-2" data-invalid={avatarInvalid || undefined}>
								<Field.Label>Photo</Field.Label>
								<div class="flex flex-wrap items-center gap-4">
									<div class="relative">
										<CardAvatar {card} fallback={profile.slug} class="size-16 text-lg" />
										{#if photoProgress !== null}
											<span class="bg-background/70 absolute inset-0 grid place-items-center rounded-full">
												<Spinner class="size-5" />
											</span>
										{/if}
									</div>
									<div class="flex flex-wrap gap-2">
										<Button
											variant="outline"
											onclick={() => photoInput?.click()}
											disabled={!storage.ready || photoProgress !== null}
										>
											<UploadIcon data-icon="inline-start" />
											{photoProgress !== null ? `Uploading ${Math.round(photoProgress * 100)}%` : 'Upload photo'}
										</Button>
										<Button variant="outline" onclick={() => (photoPickerOpen = true)} disabled={!storage.ready}>
											<ImagesIcon data-icon="inline-start" />
											From library
										</Button>
										{#if card.avatar_file || card.avatar_url}
											<Button variant="ghost" onclick={removePhoto}>Remove</Button>
										{/if}
									</div>
									<input
										bind:this={photoInput}
										type="file"
										accept={ACCEPT.image}
										class="sr-only"
										tabindex="-1"
										onchange={(e) => {
											const f = e.currentTarget.files?.[0];
											if (f) pickImage(f, 'photo');
											e.currentTarget.value = '';
										}}
									/>
								</div>
								{#if storage.status && !storage.ready}
									<Field.Description>
										<a href="/dashboard/settings" class="text-foreground underline underline-offset-4">Connect storage</a>
										to upload a photo, or paste an image URL below.
									</Field.Description>
								{/if}
								{#if !card.avatar_file}
									<Input
										id="avatar"
										type="url"
										bind:value={card.avatar_url}
										placeholder="Or paste an image URL: https://…/me.jpg"
										aria-label="Photo URL"
										aria-invalid={avatarInvalid || undefined}
									/>
									{#if avatarInvalid}
										<Field.Error>Enter a valid http(s) image URL.</Field.Error>
									{:else}
										<Field.Description>JPEG, PNG or WebP up to 5 MB. Leave empty to show your initials.</Field.Description>
									{/if}
								{/if}
							</Field.Field>
							<Field.Field class="sm:col-span-2">
								<Field.Label>Cover image</Field.Label>
								<div class="flex flex-wrap items-center gap-4">
									<div
										class="relative aspect-3/1 w-48 shrink-0 overflow-hidden rounded-lg border"
										style:background={cover ? undefined : ACCENTS[card.accent]}
									>
										{#if cover}
											<img src={cover} alt="" class="size-full object-cover" />
										{/if}
										{#if coverProgress !== null}
											<span class="bg-background/70 absolute inset-0 grid place-items-center">
												<Spinner class="size-5" />
											</span>
										{/if}
									</div>
									<div class="flex flex-wrap gap-2">
										<Button
											variant="outline"
											onclick={() => coverInput?.click()}
											disabled={!storage.ready || coverProgress !== null}
										>
											<UploadIcon data-icon="inline-start" />
											{coverProgress !== null ? `Uploading ${Math.round(coverProgress * 100)}%` : 'Upload cover'}
										</Button>
										<Button variant="outline" onclick={() => (coverPickerOpen = true)} disabled={!storage.ready}>
											<ImagesIcon data-icon="inline-start" />
											From library
										</Button>
										{#if card.cover_file}
											<Button variant="ghost" onclick={() => card && (card.cover_file = '')}>Remove</Button>
										{/if}
									</div>
									<input
										bind:this={coverInput}
										type="file"
										accept={ACCEPT.image}
										class="sr-only"
										tabindex="-1"
										onchange={(e) => {
											const f = e.currentTarget.files?.[0];
											if (f) pickImage(f, 'cover');
											e.currentTarget.value = '';
										}}
									/>
								</div>
								<Field.Description>
									A wide banner behind your photo, cropped to 3:1. Without one, the accent colour is used.
								</Field.Description>
							</Field.Field>
						</Field.Group>
					</FormSection>

					<FormSection
						id="contact"
						title="Contact"
						description="Shown as quick actions, and included when someone saves your contact."
					>
						<Field.Group class="grid gap-5 sm:grid-cols-2">
							<Field.Field data-invalid={emailInvalid || undefined}>
								<Field.Label for="email">Email</Field.Label>
								<Input
									id="email"
									type="email"
									bind:value={card.email}
									placeholder="jane@acme.com"
									aria-invalid={emailInvalid || undefined}
								/>
								{#if emailInvalid}
									<Field.Error>Enter a valid email address.</Field.Error>
								{/if}
							</Field.Field>
							<Field.Field data-invalid={phoneInvalid || undefined}>
								<Field.Label for="phone">Mobile number</Field.Label>
								<PhoneInput
									id="phone"
									bind:country={card.phone_country}
									bind:code={card.phone_country_code}
									bind:number={card.phone_number}
									invalid={phoneInvalid}
								/>
								{#if phoneInvalid}
									<Field.Error>
										{card.phone_country_code ? 'Enter a valid number for this country.' : 'Pick the country code for this number.'}
									</Field.Error>
								{/if}
							</Field.Field>
							<Field.Field class="sm:col-span-2" data-invalid={websiteInvalid || undefined}>
								<Field.Label for="website">Website</Field.Label>
								<Input
									id="website"
									bind:value={card.website}
									placeholder="acme.com"
									aria-invalid={websiteInvalid || undefined}
								/>
								{#if websiteInvalid}
									<Field.Error>Enter a valid web address.</Field.Error>
								{/if}
							</Field.Field>
							<Field.Field class="sm:col-span-2" data-invalid={calendarInvalid || undefined}>
								<Field.Label for="calendar">Booking link</Field.Label>
								<div class="flex items-center gap-2">
									<span class="bg-muted text-muted-foreground grid size-9 shrink-0 place-items-center rounded-lg">
										<BrandIcon url={card.calendar_url} kind="calendar" />
									</span>
									<Input
										id="calendar"
										bind:value={card.calendar_url}
										placeholder="calendly.com/you"
										aria-invalid={calendarInvalid || undefined}
									/>
								</div>
								{#if calendarInvalid}
									<Field.Error>Enter a valid web address.</Field.Error>
								{:else}
									<Field.Description>
										{calendarProvider
											? `${calendarProvider.name} link. Shown as a “Book a meeting” button.`
											: 'Calendly, Cal.com, Google Calendar or any booking page. Shown as a “Book a meeting” button.'}
									</Field.Description>
								{/if}
							</Field.Field>
						</Field.Group>
					</FormSection>

					<FormSection
						id="links"
						title="Links"
						description="Socials, portfolio, profiles. Known sites get their icon automatically."
					>
						<div class="flex flex-col gap-2">
							{#each card.links as link, i (link.id)}
								{@const linkInvalid = !!link.url.trim() && !safeUrl(link.url)}
								<div class="bg-card flex items-start gap-2 rounded-xl border p-2">
									<span class="bg-muted text-muted-foreground grid size-9 shrink-0 place-items-center rounded-lg">
										<BrandIcon url={link.url} />
									</span>
									<div class="grid flex-1 gap-2 sm:grid-cols-[2fr_3fr]">
										<Input
											bind:value={link.label}
											placeholder="Label (optional)"
											aria-label="Link {i + 1} label"
											class="shadow-none"
										/>
										<Input
											bind:value={link.url}
											placeholder="github.com/you"
											aria-label="Link {i + 1} URL"
											aria-invalid={linkInvalid || undefined}
											class="shadow-none"
										/>
									</div>
									<div class="flex shrink-0 max-sm:flex-col">
										<Button variant="ghost" size="icon" disabled={i === 0} onclick={() => moveLink(i, -1)} aria-label="Move up">
											<ArrowUpIcon />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											disabled={i === card.links.length - 1}
											onclick={() => moveLink(i, 1)}
											aria-label="Move down"
										>
											<ArrowDownIcon />
										</Button>
										<Button variant="ghost" size="icon" onclick={() => card?.links.splice(i, 1)} aria-label="Remove link">
											<XIcon />
										</Button>
									</div>
								</div>
							{/each}
							<button
								type="button"
								onclick={addLink}
								disabled={card.links.length >= MAX_LINKS}
								class="text-muted-foreground hover:text-foreground hover:border-foreground/30 flex h-12 items-center justify-center gap-2 rounded-xl border border-dashed text-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50"
							>
								<PlusIcon class="size-4" />
								{card.links.length >= MAX_LINKS ? `Up to ${MAX_LINKS} links` : 'Add link'}
							</button>
						</div>
					</FormSection>

					<FormSection
						id="brochures"
						title="Brochures"
						description="PDFs visitors can open from your card: price lists, portfolios, menus."
					>
						<div class="flex flex-col gap-2">
							{#each card.documents as doc, i (doc.id)}
								{@const meta = fileMeta[doc.file]}
								<div class="bg-card flex items-center gap-2 rounded-xl border p-2">
									<span class="bg-muted text-muted-foreground grid size-9 shrink-0 place-items-center rounded-lg">
										<FileTextIcon class="size-4" />
									</span>
									<div class="flex min-w-0 flex-1 flex-col gap-1">
										<Input
											bind:value={doc.title}
											placeholder={meta?.name ?? 'Brochure title'}
											aria-label="Brochure {i + 1} title"
											maxlength={120}
											class="shadow-none"
										/>
										<span class="text-muted-foreground truncate px-1 text-xs">
											{#if meta}{meta.name} · {formatBytes(meta.size_bytes)}{:else}PDF{/if}
										</span>
									</div>
									<div class="flex shrink-0 max-sm:flex-col">
										<Button variant="ghost" size="icon" disabled={i === 0} onclick={() => moveDocument(i, -1)} aria-label="Move up">
											<ArrowUpIcon />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											disabled={i === card.documents.length - 1}
											onclick={() => moveDocument(i, 1)}
											aria-label="Move down"
										>
											<ArrowDownIcon />
										</Button>
										<Button variant="ghost" size="icon" onclick={() => card?.documents.splice(i, 1)} aria-label="Remove brochure">
											<XIcon />
										</Button>
									</div>
								</div>
							{/each}
							<button
								type="button"
								onclick={() => (docPickerOpen = true)}
								disabled={!storage.ready || card.documents.length >= MAX_DOCUMENTS}
								class="text-muted-foreground hover:text-foreground hover:border-foreground/30 flex h-12 items-center justify-center gap-2 rounded-xl border border-dashed text-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50"
							>
								<PlusIcon class="size-4" />
								{card.documents.length >= MAX_DOCUMENTS ? `Up to ${MAX_DOCUMENTS} brochures` : 'Add brochure'}
							</button>
							{#if storage.status && !storage.ready}
								<p class="text-muted-foreground text-xs">
									<a href="/dashboard/settings" class="text-foreground underline underline-offset-4">Connect storage</a>
									to upload PDFs.
								</p>
							{/if}
						</div>
					</FormSection>

					<FormSection id="appearance" title="Appearance" description="How your card looks to visitors.">
						<Field.Group class="gap-6">
							<Field.Field>
								<Field.Label>Accent colour</Field.Label>
								<div class="flex flex-wrap gap-2.5" role="radiogroup" aria-label="Accent colour">
									{#each Object.entries(ACCENTS) as [key, color] (key)}
										<button
											type="button"
											role="radio"
											aria-checked={card.accent === key}
											aria-label={key}
											title={key}
											onclick={() => card && (card.accent = key as AccentKey)}
											class={cn(
												'ring-offset-background grid size-9 place-items-center rounded-full text-white ring-offset-2 transition-shadow',
												card.accent === key ? 'ring-foreground ring-2' : 'hover:ring-border hover:ring-2'
											)}
											style="background: {color}"
										>
											{#if card.accent === key}<CheckIcon class="size-4" />{/if}
										</button>
									{/each}
								</div>
								{#if card.cover_file}
									<Field.Description>With a cover image, the accent frames your photo and the banner.</Field.Description>
								{/if}
							</Field.Field>
							<Field.Field>
								<Field.Label>Card theme</Field.Label>
								<div class="grid max-w-md grid-cols-2 gap-3" role="radiogroup" aria-label="Card theme">
									{#each ['light', 'dark'] as const as theme (theme)}
										<button
											type="button"
											role="radio"
											aria-checked={card.theme === theme}
											onclick={() => card && (card.theme = theme)}
											class={cn(
												'flex flex-col gap-2 rounded-xl border p-2 text-left text-sm font-medium transition-colors',
												card.theme === theme ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
											)}
										>
											<span
												class={cn(
													'flex h-16 flex-col overflow-hidden rounded-lg ring-1 ring-black/5',
													theme === 'dark' ? 'bg-neutral-900' : 'bg-white'
												)}
												aria-hidden="true"
											>
												<span class="h-5" style="background: {ACCENTS[card.accent]}"></span>
												<span class="flex flex-col gap-1 p-2">
													<span class={cn('h-1.5 w-12 rounded-full', theme === 'dark' ? 'bg-white/70' : 'bg-neutral-800')}></span>
													<span class={cn('h-1.5 w-8 rounded-full', theme === 'dark' ? 'bg-white/30' : 'bg-neutral-300')}></span>
												</span>
											</span>
											<span class="px-1 capitalize">{theme}</span>
										</button>
									{/each}
								</div>
							</Field.Field>
						</Field.Group>
					</FormSection>

					<FormSection id="sharing" title="Sharing" description="Your public address and what visitors can do.">
						<Field.Group class="gap-6">
							<Field.Field data-invalid={slugInvalid || undefined}>
								<Field.Label for="slug">Public link</Field.Label>
								<div class="flex items-stretch">
									<span
										class="text-muted-foreground bg-muted flex max-w-[45%] items-center truncate rounded-l-lg border border-r-0 px-3 font-mono text-xs"
									>
										<span class="max-sm:hidden">{location.host}</span>/p/
									</span>
									<Input
										id="slug"
										class="rounded-l-none font-mono"
										value={slug}
										oninput={(e) => (slug = e.currentTarget.value.toLowerCase())}
										aria-invalid={slugInvalid || undefined}
									/>
								</div>
								{#if slugInvalid}
									<Field.Error>3–48 characters: lowercase letters, numbers and hyphens.</Field.Error>
								{:else if slug !== profile.slug}
									<Field.Description class="text-amber-700 dark:text-amber-400">
										Changing this breaks the old link and any QR codes or NFC cards already printed.
									</Field.Description>
								{:else}
									<Field.Description>Write this link to an NFC card, or print its QR code.</Field.Description>
								{/if}
							</Field.Field>
							<Field.Field orientation="horizontal" class="bg-card rounded-xl border p-4">
								<Field.Content>
									<Field.Label for="collect">Collect leads</Field.Label>
									<Field.Description>
										Show a “Share your contact” button so visitors can leave their details.
									</Field.Description>
								</Field.Content>
								<Switch id="collect" bind:checked={card.collect_leads} />
							</Field.Field>
						</Field.Group>
					</FormSection>

					{#if dirty}
						<div
							class="bg-card/95 sticky bottom-4 z-10 mt-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border px-4 py-3 shadow-lg backdrop-blur"
							transition:fly={{ y: 12, duration: reduceMotion ? 0 : 160 }}
							role="status"
						>
							<p class="text-sm">
								{#if saveError}
									<span class="text-destructive">{saveError}</span>
								{:else if invalid}
									<span class="text-destructive">Fix the highlighted fields to save.</span>
								{:else}
									<span class="font-medium">Unsaved changes</span>
								{/if}
							</p>
							<div class="flex items-center gap-2">
								<Button variant="ghost" onclick={discard} disabled={saving}>Discard</Button>
								<Button onclick={save} disabled={!canSave}>
									{#if saving}
										<Spinner data-icon="inline-start" />
									{/if}
									Save changes
									<kbd class="text-primary-foreground/60 ml-1 hidden font-sans text-xs sm:inline">⌘S</kbd>
								</Button>
							</div>
						</div>
					{/if}
				</div>

				<aside class="bg-muted/40 bg-dots hidden border-l xl:block" aria-label="Preview">
					<div class="sticky top-0 flex h-svh flex-col gap-5 p-8">
						<div class="flex items-center justify-between">
							<div class="bg-muted inline-flex rounded-lg p-[3px]" role="tablist" aria-label="Preview mode">
								{#each [['card', 'Card'], ['qr', 'QR code']] as const as [mode, label] (mode)}
									<button
										type="button"
										role="tab"
										aria-selected={previewMode === mode}
										onclick={() => (previewMode = mode)}
										class={cn(
											'h-8 rounded-md px-3 text-sm font-medium transition-colors',
											previewMode === mode ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
										)}
									>
										{label}
									</button>
								{/each}
							</div>
							<Button variant="ghost" size="sm" href="/p/{profile.slug}" target="_blank">
								Open
								<ExternalLinkIcon data-icon="inline-end" />
							</Button>
						</div>

						<div class="-mx-2 flex min-h-0 flex-1 justify-center overflow-y-auto px-2 py-2">
							{#if previewMode === 'card'}
								<div class="my-auto w-full max-w-[380px]">
									<ProfileCard {card} slug={profile.slug} files={fileMeta} />
								</div>
							{:else}
								<div class="my-auto flex w-full max-w-[300px] flex-col items-center gap-4">
									<QrCode url={publicUrl(profile.slug)} bind:svg={qrMarkup} class="w-full" />
									<p class="text-muted-foreground max-w-full truncate font-mono text-xs">{publicUrl(profile.slug)}</p>
									<div class="grid w-full grid-cols-2 gap-2">
										<Button variant="outline" onclick={() => downloadQrSvg(qrMarkup, profile!.slug)} disabled={!qrMarkup}>
											<DownloadIcon data-icon="inline-start" />
											SVG
										</Button>
										<Button onclick={() => downloadQrPng(publicUrl(profile!.slug), profile!.slug)} disabled={!qrMarkup}>
											<DownloadIcon data-icon="inline-start" />
											PNG
										</Button>
									</div>
									{#if slug !== profile.slug}
										<p class="text-muted-foreground text-center text-xs">
											This code uses your saved link. Save to update it.
										</p>
									{/if}
								</div>
							{/if}
						</div>
					</div>
				</aside>
			</div>
		</Tabs.Content>

		<Tabs.Content value="leads" class="px-4 py-8 sm:px-8 lg:px-10">
			{#key profileId}
				<LeadsTable profileId={profile.id} filename="{profile.slug}-leads" />
			{/key}
		</Tabs.Content>
	</Tabs.Root>

	<!-- Uses the saved slug: an unsaved edit isn't live yet, so its QR wouldn't resolve. -->
	<QrDialog bind:open={qrOpen} slug={profile.slug} name={card.name} />
	<DeleteCardDialog bind:target={deleteTarget} ondeleted={onDeleted} />
	<FilePickerDialog
		bind:open={photoPickerOpen}
		kind="image"
		title="Choose a photo"
		selected={card.avatar_file ? [card.avatar_file] : []}
		onselect={usePhoto}
	/>
	<FilePickerDialog
		bind:open={coverPickerOpen}
		kind="image"
		title="Choose a cover image"
		selected={card.cover_file ? [card.cover_file] : []}
		onselect={useCover}
	/>
	<ImageCropDialog
		bind:file={cropPhoto}
		title="Crop photo"
		aspect={1}
		shape="round"
		outputWidth={512}
		outputHeight={512}
		onconfirm={uploadPhoto}
	/>
	<ImageCropDialog
		bind:file={cropCover}
		title="Crop cover image"
		aspect={3}
		outputWidth={1500}
		outputHeight={500}
		onconfirm={uploadCover}
	/>
	<FilePickerDialog
		bind:open={docPickerOpen}
		kind="pdf"
		title="Add a brochure"
		selected={card.documents.map((d) => d.file)}
		onselect={addDocument}
	/>

	<Dialog.Root bind:open={previewOpen}>
		<Dialog.Content class="bg-muted max-h-[90svh] overflow-y-auto p-4 sm:max-w-sm">
			<Dialog.Title class="sr-only">Preview</Dialog.Title>
			<ProfileCard {card} slug={profile.slug} files={fileMeta} />
		</Dialog.Content>
	</Dialog.Root>
{/if}
