<script lang="ts">
	import { beforeNavigate, goto } from '$app/navigation';
	import { page } from '$app/state';
	import { fly } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ArrowUpRightIcon from '@lucide/svelte/icons/arrow-up-right';
	import CheckIcon from '@lucide/svelte/icons/check';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import EyeIcon from '@lucide/svelte/icons/eye';
	import QrCodeIcon from '@lucide/svelte/icons/qr-code';
	import SmartphoneNfcIcon from '@lucide/svelte/icons/smartphone-nfc';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Tabs from '$lib/components/ui/tabs';
	import SectionRail from '$lib/components/shared/section-rail.svelte';
	import { timeAgo } from '$lib/core/format';
	import { session } from '$lib/core/session.svelte';
	import { getAnalyticsSummary, lastDays, type AnalyticsTotals } from '$lib/features/analytics/api';
	import { branding } from '$lib/features/branding/store.svelte';
	import { getMyProfile, getPublicProfile, updateProfile, type Profile } from '$lib/features/cards/api';
	import { normalizeCard, publicUrl, type CardData } from '$lib/features/cards/card';
	import CardAvatar from '$lib/features/cards/components/card-avatar.svelte';
	import DeleteCardDialog from '$lib/features/cards/components/delete-card-dialog.svelte';
	import NfcDialog from '$lib/features/cards/components/nfc-dialog.svelte';
	import ProfileCard from '$lib/features/cards/components/profile-card.svelte';
	import QrDialog from '$lib/features/cards/components/qr-dialog.svelte';
	import AppearanceSection from '$lib/features/cards/editor/appearance-section.svelte';
	import BrochuresSection from '$lib/features/cards/editor/brochures-section.svelte';
	import ContactSection from '$lib/features/cards/editor/contact-section.svelte';
	import LayoutSection from '$lib/features/cards/editor/layout-section.svelte';
	import LinksSection from '$lib/features/cards/editor/links-section.svelte';
	import PreviewPanel from '$lib/features/cards/editor/preview-panel.svelte';
	import ProfileSection from '$lib/features/cards/editor/profile-section.svelte';
	import QrSection from '$lib/features/cards/editor/qr-section.svelte';
	import { railItems, SECTIONS, sectionErrors, sectionFrom, type SectionId } from '$lib/features/cards/editor/sections';
	import SharingSection from '$lib/features/cards/editor/sharing-section.svelte';
	import { cardErrors, hasErrors } from '$lib/features/cards/editor/validation';
	import { cards } from '$lib/features/cards/store.svelte';
	import type { LibraryFile, PublicFile } from '$lib/features/files/api';
	import LeadsTable from '$lib/features/leads/components/leads-table.svelte';
	import { setCardAssignee } from '$lib/features/orgs/api';
	import UserPicker from '$lib/features/orgs/components/user-picker.svelte';
	import { orgUsers } from '$lib/features/orgs/users.svelte';

	const profileId = $derived(Number(page.params.id));

	let profile = $state<Profile | null>(null);
	let card = $state<CardData | null>(null);
	let slug = $state('');
	let loadError = $state('');
	let saving = $state(false);
	let saveError = $state('');
	let qrOpen = $state(false);
	let nfcOpen = $state(false);
	let previewOpen = $state(false);
	let previewMode = $state<'card' | 'qr'>('card');
	let deleteTarget = $state<Profile | null>(null);
	let tab = $state(page.url.searchParams.get('tab') === 'leads' ? 'leads' : 'card');

	// The last saved state; anything different is an unsaved change.
	let snapshot = $state('');
	const serialized = $derived(JSON.stringify({ slug, card }));
	const dirty = $derived(card !== null && serialized !== snapshot);

	// Metadata for library files this card uses: names and sizes for the editor and preview.
	let fileMeta = $state<Record<string, PublicFile>>({});

	const errors = $derived(card ? cardErrors(card, slug) : null);
	const invalid = $derived(!!errors && hasErrors(errors));
	const canSave = $derived(dirty && !saving && !invalid);

	// The last 30 days at a glance, linking to the full analytics.
	let recentStats = $state<AnalyticsTotals | null>(null);
	$effect(() => {
		const id = profileId;
		recentStats = null;
		getAnalyticsSummary({ ...lastDays(30), profileId: id })
			.then((s) => id === profileId && (recentStats = s.current))
			.catch(() => {});
	});

	// Kept current by the cards store (the single-card endpoint doesn't count leads).
	const leadCount = $derived(cards.list?.find((p) => p.id === profileId)?.lead_count ?? 0);

	const reduceMotion = typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

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
			getPublicProfile(session.orgHandle, p.slug)
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
		// Switching sections only changes the hash; nothing is lost.
		if (nav.to?.url.pathname === page.url.pathname) return;
		if (dirty && !confirm('You have unsaved changes. Leave anyway?')) nav.cancel();
	});

	// One section of the form shows at a time, picked from the rail. The URL
	// hash (#contact) remembers it, so links can open a given section.
	let section = $derived<SectionId>(sectionFrom(page.url.hash));
	// Styling the code is easier with the code in view.
	$effect(() => {
		if (section === 'qr') previewMode = 'qr';
	});

	function openSection(id: SectionId) {
		section = id;
		goto(`#${id}`, { shallow: true, replace: true, reset: false });
	}

	const errorsBySection = $derived(errors ? sectionErrors(errors) : null);
	const rail = $derived(
		card && errorsBySection ? railItems(card, `/p/${session.orgHandle}/${slug}`, errorsBySection) : []
	);
	// While the unsaved-changes bar floats over the bottom of the page, keep
	// focused fields (e.g. tabbing to "Add block") scrolled clear of it.
	$effect(() => {
		if (!dirty) return;
		const root = document.documentElement;
		root.style.scrollPaddingBottom = '6rem';
		return () => root.style.removeProperty('scroll-padding-bottom');
	});
	const firstInvalid = $derived(SECTIONS.find((id) => errorsBySection?.[id]) ?? null);

	async function copyLink() {
		if (!profile) return;
		try {
			await navigator.clipboard.writeText(publicUrl(session.orgHandle, profile.slug));
			toast.success('Link copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}

	function remember(file: LibraryFile) {
		fileMeta[file.id] = { id: file.id, kind: file.kind, name: file.title || file.name, size_bytes: file.size_bytes };
	}

	async function onDeleted() {
		card = null; // nothing left to lose, so skip the unsaved-changes prompt
		await goto('/dashboard/cards');
	}

	let assigning = $state(false);
	async function assign(value: number | 'none' | null) {
		if (!profile) return;
		const to = typeof value === 'number' ? value : null;
		if (to === (profile.assigned_user?.id ?? null)) return;
		assigning = true;
		try {
			const p = await setCardAssignee(profile.id, to);
			profile = { ...profile, assigned_user: p.assigned_user };
			cards.upsert(p);
			orgUsers.refresh();
			toast.success(to === null ? 'Card returned to the organisation' : `Assigned to ${p.assigned_user?.username}`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not assign the card');
		} finally {
			assigning = false;
		}
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
			<p class="text-muted-foreground text-sm">
				{session.isAdmin ? 'It may have been deleted.' : 'It may have been deleted, or it’s no longer assigned to you.'}
			</p>
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
								href="/p/{session.orgHandle}/{profile.slug}"
								target="_blank"
								class="hover:text-foreground flex min-w-0 items-center gap-1 font-mono"
							>
								<span class="truncate">/p/{session.orgHandle}/{profile.slug}</span>
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

				<div class="flex flex-wrap items-center gap-2">
					{#if session.isAdmin}
						<UserPicker
							value={profile.assigned_user?.id ?? null}
							onchange={assign}
							filter={false}
							disabled={assigning}
						/>
					{/if}
					<Button variant="outline" class="xl:hidden" onclick={() => (previewOpen = true)}>
						<EyeIcon data-icon="inline-start" />
						Preview
					</Button>
					<Button variant="outline" onclick={copyLink} class="max-sm:hidden">
						<CopyIcon data-icon="inline-start" />
						Copy link
					</Button>
					<DropdownMenu.Root>
						<DropdownMenu.Trigger
							class={buttonVariants({ variant: 'outline', size: 'icon' })}
							aria-label="More actions"
						>
							<EllipsisIcon />
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end" class="w-52">
							<DropdownMenu.Group>
								<DropdownMenu.Item onSelect={() => window.open(`/p/${session.orgHandle}/${profile!.slug}`, '_blank')}>
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
								<DropdownMenu.Item onSelect={() => (nfcOpen = true)}>
									<SmartphoneNfcIcon />
									Write to NFC card
								</DropdownMenu.Item>
							</DropdownMenu.Group>
							{#if session.isAdmin}
								<DropdownMenu.Separator />
								<DropdownMenu.Group>
									<DropdownMenu.Item variant="destructive" onSelect={() => (deleteTarget = profile)}>
										<Trash2Icon />
										Delete card
									</DropdownMenu.Item>
								</DropdownMenu.Group>
							{/if}
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				</div>
			</div>

			<Tabs.List variant="line" class="mt-4 -mb-px h-10 w-full justify-start gap-4 p-0">
				<Tabs.Trigger value="card" class="flex-none px-0.5">Card</Tabs.Trigger>
				<Tabs.Trigger value="leads" class="flex-none px-0.5">
					Leads
					<span class="bg-muted text-muted-foreground tabular rounded-full px-1.5 py-0.5 text-[11px] leading-none">
						{leadCount}
					</span>
				</Tabs.Trigger>
				<a
					href="/dashboard/analytics?card={profile.id}"
					class="text-muted-foreground hover:text-foreground ml-auto inline-flex items-center gap-1.5 self-center text-sm"
					title="Last 30 days"
				>
					{#if recentStats}
						<span class="tabular"
							>{recentStats.views} views · {recentStats.saves} saves · {recentStats.leads} leads</span
						>
					{:else}
						Analytics
					{/if}
					<ArrowUpRightIcon class="size-3.5" />
				</a>
			</Tabs.List>
		</header>

		<Tabs.Content value="card">
			<div class="grid xl:grid-cols-[minmax(0,1fr)_460px] 2xl:grid-cols-[minmax(0,1fr)_540px]">
				<div class="min-w-0 px-4 pb-10 sm:px-8 lg:px-10">
					<div class="grid gap-6 pt-6 lg:grid-cols-[220px_minmax(0,1fr)] lg:gap-10">
						<SectionRail
							label="Card sections"
							items={rail}
							active={section}
							onselect={openSection}
							class="lg:sticky lg:top-6 lg:self-start"
						/>
						<div class="min-w-0">
							{#if errors}
								{#if section === 'profile'}
									<ProfileSection bind:card slug={profile.slug} {errors} onfile={remember} />
								{:else if section === 'contact'}
									<ContactSection bind:card {errors} booking={profile.booking} holder={profile.assigned_user} />
								{:else if section === 'links'}
									<LinksSection bind:card {errors} />
								{:else if section === 'brochures'}
									<BrochuresSection bind:card files={fileMeta} onfile={remember} />
								{:else if section === 'layout'}
									<LayoutSection bind:card files={fileMeta} onfile={remember} />
								{:else if section === 'appearance'}
									<AppearanceSection bind:card />
								{:else if section === 'qr'}
									<QrSection bind:card slug={profile.slug} />
								{:else if section === 'sharing'}
									<SharingSection
										bind:card
										bind:slug
										savedSlug={profile.slug}
										profileId={profile.id}
										{errors}
										onnfc={() => (nfcOpen = true)}
									/>
								{/if}
							{/if}

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
											{#if firstInvalid && firstInvalid !== section}
												<button
													type="button"
													class="text-foreground ml-1 font-medium underline underline-offset-4"
													onclick={() => firstInvalid && openSection(firstInvalid)}
												>
													Show me
												</button>
											{/if}
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
					</div>
				</div>

				<PreviewPanel
					{card}
					savedSlug={profile.slug}
					{slug}
					files={fileMeta}
					booking={profile.booking}
					bind:mode={previewMode}
				/>
			</div>
		</Tabs.Content>

		<Tabs.Content value="leads" class="px-4 py-8 sm:px-8 lg:px-10">
			{#key profileId}
				<LeadsTable profileId={profile.id} filename="{profile.slug}-leads" />
			{/key}
		</Tabs.Content>
	</Tabs.Root>

	<!-- Uses the saved slug: an unsaved edit isn't live yet, so its QR wouldn't resolve. -->
	<QrDialog bind:open={qrOpen} slug={profile.slug} name={card.name} style={card.qr} />
	<NfcDialog bind:open={nfcOpen} profileId={profile.id} slug={profile.slug} name={card.name} />
	<DeleteCardDialog bind:target={deleteTarget} ondeleted={onDeleted} />

	<Dialog.Root bind:open={previewOpen}>
		<Dialog.Content class="bg-muted max-h-[90svh] overflow-y-auto p-4 sm:max-w-sm">
			<Dialog.Title class="sr-only">Preview</Dialog.Title>
			<ProfileCard {card} slug={profile.slug} files={fileMeta} org={branding.value} booking={profile.booking} />
		</Dialog.Content>
	</Dialog.Root>
{/if}
