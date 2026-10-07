<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import CheckIcon from '@lucide/svelte/icons/check';
	import CodeIcon from '@lucide/svelte/icons/code';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import LockIcon from '@lucide/svelte/icons/lock';
	import MoonIcon from '@lucide/svelte/icons/moon';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import SignatureIcon from '@lucide/svelte/icons/signature';
	import SunIcon from '@lucide/svelte/icons/sun';
	import { fileUrl } from '$lib/features/files/api';
	import { updateProfile } from '$lib/features/cards/api';
	import { Button } from '$lib/components/ui/button';
	import * as Empty from '$lib/components/ui/empty';
	import * as Field from '$lib/components/ui/field';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Switch } from '$lib/components/ui/switch';
	import * as Tabs from '$lib/components/ui/tabs';
	import CardAvatar from '$lib/features/cards/components/card-avatar.svelte';
	import { branding } from '$lib/features/branding/store.svelte';
	import { downloadBlob, normalizeCard, publicUrl, type CardData } from '$lib/features/cards/card';
	import { session } from '$lib/core/session.svelte';
	import { cards } from '$lib/features/cards/store.svelte';
	import {
		darkModeHtml,
		measureImage,
		renderSignature,
		signatureDocument,
		type ImageSize
	} from '$lib/features/signatures/render';
	import {
		SIGNATURE_TEMPLATES,
		SIGNATURE_TEMPLATE_KEYS,
		type SignatureSettings,
		type SignatureTemplateKey
	} from '$lib/features/signatures/templates';
	import { cn } from '$lib/utils';

	const selectedId = $derived(Number(page.url.searchParams.get('card')) || cards.list?.[0]?.id || null);
	const profile = $derived(cards.list?.find((p) => p.id === selectedId) ?? null);

	// The card's saved data, with the signature settings being edited swapped in.
	let settings = $state<SignatureSettings | null>(null);
	let saved = $state('');
	let saving = $state(false);
	$effect(() => {
		// Reset whenever another card is picked (or the saved card changes).
		const s = profile ? normalizeCard(profile.data).signature : null;
		settings = s;
		saved = JSON.stringify(s);
	});

	const card = $derived<CardData | null>(
		profile && settings ? { ...normalizeCard(profile.data), signature: settings } : null
	);
	const dirty = $derived(!!settings && JSON.stringify(settings) !== saved);

	const org = $derived(branding.value);
	const locked = $derived(!!org?.signature.locked_template);
	const logoRequired = $derived(!!org?.logo_file && org.logo_policy === 'required');

	// Exact image sizes let the HTML carry width and height, which Outlook needs.
	let logoSize = $state<ImageSize | null>(null);
	let bannerSize = $state<ImageSize | null>(null);
	$effect(() => {
		const id = org?.logo_file;
		logoSize = null;
		if (id) measureImage(fileUrl(id)).then((s) => (logoSize = s));
	});
	$effect(() => {
		const id = org?.signature.banner_file;
		bannerSize = null;
		if (id) measureImage(fileUrl(id)).then((s) => (bannerSize = s));
	});

	const rendered = $derived(
		card && profile
			? renderSignature(card, publicUrl(session.orgHandle, profile.slug), org, {
					logoSize,
					bannerSize,
					bookingUrl: profile.booking?.url
				})
			: null
	);
	const template = $derived(rendered?.template ?? 'classic');
	const templates = $derived<SignatureTemplateKey[]>(
		locked && rendered ? [rendered.template] : SIGNATURE_TEMPLATE_KEYS
	);

	let darkPreview = $state(false);
	let copied = $state<'rich' | 'html' | null>(null);

	function select(id: number) {
		if (dirty && !confirm('You have unsaved signature changes. Switch card anyway?')) return;
		goto(`?card=${id}`, { replace: true, reset: false });
	}

	async function save() {
		if (!profile || !card) return;
		saving = true;
		try {
			const updated = await updateProfile(profile.id, profile.slug, $state.snapshot(card));
			cards.upsert(updated);
			toast.success('Signature saved');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to save');
		} finally {
			saving = false;
		}
	}

	function flash(kind: 'rich' | 'html') {
		copied = kind;
		setTimeout(() => (copied = copied === kind ? null : copied), 2000);
	}

	async function copyRich() {
		if (!rendered) return;
		try {
			await navigator.clipboard.write([
				new ClipboardItem({
					'text/html': new Blob([rendered.html], { type: 'text/html' }),
					'text/plain': new Blob([rendered.text], { type: 'text/plain' })
				})
			]);
			flash('rich');
			toast.success('Signature copied. Paste it into your email signature settings.');
		} catch {
			toast.error('Your browser blocked copying. Use “Download .html” instead.');
		}
	}

	async function copyHtml() {
		if (!rendered) return;
		try {
			await navigator.clipboard.writeText(rendered.html);
			flash('html');
		} catch {
			toast.error('Your browser blocked copying.');
		}
	}

	function download() {
		if (!rendered || !profile) return;
		downloadBlob(new Blob([signatureDocument(rendered.html)], { type: 'text/html' }), `${profile.slug}-signature.html`);
	}

	const FIELDS: { key: Exclude<keyof SignatureSettings, 'template'>; label: string }[] = [
		{ key: 'show_phone', label: 'Phone' },
		{ key: 'show_email', label: 'Email' },
		{ key: 'show_website', label: 'Website' },
		{ key: 'show_location', label: 'Location' },
		{ key: 'show_booking', label: 'Booking link' },
		{ key: 'show_socials', label: 'Social links' },
		{ key: 'show_card_link', label: 'Link to my card' }
	];

	const CLIENTS = [
		{
			key: 'gmail',
			label: 'Gmail',
			steps: [
				'Click “Copy signature” above.',
				'In Gmail, open Settings (gear icon) → See all settings → General.',
				'Under Signature, create a new signature and paste into the box.',
				'Choose it under “Signature defaults”, then click “Save Changes” at the bottom.'
			]
		},
		{
			key: 'outlook',
			label: 'Outlook',
			steps: [
				'Click “Copy signature” above.',
				'Outlook on the web or new Outlook: Settings → Account → Signatures. Classic Outlook for Windows: File → Options → Mail → Signatures.',
				'Create a new signature and paste into the editor.',
				'Set it as the default for new messages and replies, then save.'
			]
		},
		{
			key: 'apple',
			label: 'Apple Mail',
			steps: [
				'Click “Copy signature” above.',
				'In Mail, open Settings → Signatures and add a signature with +.',
				'Untick “Always match my default message font”, then paste into the right-hand pane.',
				'Images may show as blue boxes in the settings window; they appear correctly in sent mail.'
			]
		}
	] as const;

	/** Grows the preview to its content. Scripts stay off in the frame; same-origin only lets us measure it. */
	function fitFrame(frame: HTMLIFrameElement) {
		const body = frame.contentDocument?.body;
		if (body) frame.style.height = `${body.scrollHeight}px`;
	}

	/** Gallery thumbnails: the real signature, drawn with each template. */
	function preview(key: SignatureTemplateKey): string {
		if (!card || !profile) return '';
		return renderSignature(card, publicUrl(session.orgHandle, profile.slug), org, {
			template: key,
			logoSize,
			bannerSize,
			bookingUrl: profile.booking?.url
		}).html;
	}
</script>

<svelte:head>
	<title>Email signatures · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Email signatures</h1>
		<p class="text-muted-foreground text-sm">
			Turn a card into a signature for Gmail, Outlook or Apple Mail. It uses the card’s details, so update the card to
			change them.
		</p>
	</header>

	{#if cards.list === null}
		<Skeleton class="h-96 rounded-xl" />
	{:else if cards.list.length === 0}
		<Empty.Root class="border">
			<Empty.Header>
				<Empty.Media variant="icon"><SignatureIcon /></Empty.Media>
				<Empty.Title>No cards yet</Empty.Title>
				<Empty.Description>A signature is made from a card. Once you have one, it’ll show up here.</Empty.Description>
			</Empty.Header>
		</Empty.Root>
	{:else}
		<div class="grid gap-8 lg:grid-cols-[240px_minmax(0,1fr)]">
			<!-- Card picker -->
			<nav aria-label="Cards" class="flex gap-2 overflow-x-auto pb-1 lg:flex-col lg:overflow-visible">
				{#each cards.list as p (p.id)}
					{@const data = normalizeCard(p.data)}
					<button
						type="button"
						onclick={() => select(p.id)}
						aria-current={p.id === selectedId ? 'true' : undefined}
						class={cn(
							'flex min-w-48 items-center gap-2.5 rounded-lg border px-3 py-2 text-left transition-colors lg:min-w-0',
							p.id === selectedId ? 'border-foreground bg-muted/50' : 'hover:bg-muted/40'
						)}
					>
						<CardAvatar card={data} fallback={p.slug} />
						<span class="flex min-w-0 flex-col">
							<span class="truncate text-sm font-medium">{data.name || p.slug}</span>
							<span class="text-muted-foreground truncate text-xs">/p/{session.orgHandle}/{p.slug}</span>
						</span>
					</button>
				{/each}
			</nav>

			{#if card && profile && settings && rendered}
				<div class="flex min-w-0 flex-col gap-8">
					<!-- Preview and actions -->
					<section class="flex flex-col gap-3" aria-labelledby="preview-title">
						<div class="flex flex-wrap items-center justify-between gap-2">
							<h2 id="preview-title" class="text-base font-semibold tracking-tight">Preview</h2>
							<div class="flex items-center gap-1">
								<Button
									variant="ghost"
									size="icon-sm"
									aria-label={darkPreview ? 'Preview on light background' : 'Preview on dark background'}
									title={darkPreview ? 'Light background' : 'Dark background'}
									onclick={() => (darkPreview = !darkPreview)}
								>
									{#if darkPreview}<SunIcon />{:else}<MoonIcon />{/if}
								</Button>
								<Button variant="ghost" size="sm" href="/dashboard/{profile.id}">
									<PencilIcon data-icon="inline-start" />
									Edit card
								</Button>
							</div>
						</div>
						<div class={cn('overflow-hidden rounded-xl border', darkPreview ? 'bg-neutral-900' : 'bg-white')}>
							<div
								class="border-b px-4 py-2 text-xs {darkPreview ? 'border-white/10 text-white/50' : 'text-neutral-400'}"
							>
								Best regards,
							</div>
							<iframe
								title="Signature preview"
								sandbox="allow-popups allow-same-origin"
								class="block min-h-32 w-full"
								onload={(e) => fitFrame(e.currentTarget as HTMLIFrameElement)}
								srcdoc={darkPreview
									? signatureDocument(darkModeHtml(rendered.html), '#171717')
									: signatureDocument(rendered.html)}
							></iframe>
						</div>
						{#if darkPreview}
							<p class="text-muted-foreground text-xs">
								Dark text is lightened here, as mail apps in dark mode usually do; logos with a transparent background
								are shown as-is.
							</p>
						{/if}
						<div class="flex flex-wrap gap-2">
							<Button onclick={copyRich} disabled={dirty}>
								{#if copied === 'rich'}<CheckIcon data-icon="inline-start" />{:else}<CopyIcon
										data-icon="inline-start"
									/>{/if}
								Copy signature
							</Button>
							<Button variant="outline" onclick={copyHtml} disabled={dirty}>
								{#if copied === 'html'}<CheckIcon data-icon="inline-start" />{:else}<CodeIcon
										data-icon="inline-start"
									/>{/if}
								Copy HTML
							</Button>
							<Button variant="outline" onclick={download} disabled={dirty}>
								<DownloadIcon data-icon="inline-start" />
								Download .html
							</Button>
						</div>
						{#if dirty}
							<p class="text-muted-foreground text-xs">
								Save your changes to copy the signature.
								<button
									type="button"
									class="text-foreground font-medium underline underline-offset-4"
									onclick={save}
									disabled={saving}>Save now</button
								>
							</p>
						{/if}
					</section>

					<!-- Template -->
					<section class="flex flex-col gap-3" aria-labelledby="template-title">
						<div class="flex flex-col gap-1">
							<h2 id="template-title" class="text-base font-semibold tracking-tight">Template</h2>
							{#if locked}
								<p class="text-muted-foreground flex items-center gap-1.5 text-sm">
									<LockIcon class="size-3.5" />
									{org?.name} uses the {SIGNATURE_TEMPLATES[template].label} template for everyone.
								</p>
							{/if}
						</div>
						<div class="grid gap-3 sm:grid-cols-2 2xl:grid-cols-3" role="radiogroup" aria-label="Signature template">
							{#each templates as key (key)}
								{@const active = template === key}
								<button
									type="button"
									role="radio"
									aria-checked={active}
									aria-labelledby="template-{key}-label"
									aria-describedby="template-{key}-description"
									disabled={locked}
									onclick={() => settings && (settings.template = key)}
									class={cn(
										'flex flex-col gap-2 rounded-xl border p-2 text-left transition-colors disabled:cursor-default',
										active ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
									)}
								>
									<span
										class="pointer-events-none relative h-32 overflow-hidden rounded-lg bg-white ring-1 ring-black/5"
										aria-hidden="true"
									>
										<span class="absolute top-3 left-3 block w-[200%] origin-top-left scale-50">
											<!-- Our own renderer's output: every user-supplied value in it is escaped. -->
											<!-- eslint-disable-next-line svelte/no-at-html-tags -->
											{@html preview(key)}
										</span>
									</span>
									<span class="flex flex-col gap-0.5 px-1 pb-1">
										<span id="template-{key}-label" class="flex items-center gap-1.5 text-sm font-medium">
											{SIGNATURE_TEMPLATES[key].label}
											{#if active}<CheckIcon class="size-3.5" />{/if}
										</span>
										<span id="template-{key}-description" class="text-muted-foreground text-xs leading-snug"
											>{SIGNATURE_TEMPLATES[key].description}</span
										>
									</span>
								</button>
							{/each}
						</div>
					</section>

					<!-- What to include -->
					<section class="flex flex-col gap-3" aria-labelledby="include-title">
						<h2 id="include-title" class="text-base font-semibold tracking-tight">Include</h2>
						<Field.Group class="grid gap-2 sm:grid-cols-2">
							{#if SIGNATURE_TEMPLATES[template].photo}
								<Field.Field orientation="horizontal" class="bg-card rounded-xl border px-4 py-3">
									<Field.Label for="sig-photo" class="font-normal">Photo</Field.Label>
									<Switch id="sig-photo" bind:checked={settings.show_photo} />
								</Field.Field>
							{/if}
							{#if org?.logo_file}
								<Field.Field orientation="horizontal" class="bg-card rounded-xl border px-4 py-3">
									<Field.Content>
										<Field.Label for="sig-logo" class="font-normal">{org.name} logo</Field.Label>
										{#if logoRequired}
											<Field.Description class="text-xs">Required by {org.name}.</Field.Description>
										{/if}
									</Field.Content>
									{#if logoRequired}
										<Switch id="sig-logo" checked disabled />
									{:else}
										<Switch id="sig-logo" bind:checked={settings.show_logo} />
									{/if}
								</Field.Field>
							{/if}
							{#each FIELDS as f (f.key)}
								<Field.Field orientation="horizontal" class="bg-card rounded-xl border px-4 py-3">
									<Field.Label for="sig-{f.key}" class="font-normal">{f.label}</Field.Label>
									<Switch id="sig-{f.key}" bind:checked={settings[f.key]} />
								</Field.Field>
							{/each}
						</Field.Group>
						{#if org?.signature.disclaimer || org?.signature.banner_file}
							<p class="text-muted-foreground text-xs">
								{org.name} adds {[org.signature.banner_file && 'a banner', org.signature.disclaimer && 'a disclaimer']
									.filter(Boolean)
									.join(' and ')} to every signature.
							</p>
						{/if}
					</section>

					<!-- Install -->
					<section class="flex flex-col gap-3" aria-labelledby="install-title">
						<h2 id="install-title" class="text-base font-semibold tracking-tight">Add it to your email</h2>
						<Tabs.Root value="gmail">
							<Tabs.List>
								{#each CLIENTS as c (c.key)}
									<Tabs.Trigger value={c.key}>{c.label}</Tabs.Trigger>
								{/each}
							</Tabs.List>
							{#each CLIENTS as c (c.key)}
								<Tabs.Content value={c.key}>
									<ol class="text-muted-foreground mt-2 flex list-decimal flex-col gap-1.5 pl-5 text-sm">
										{#each c.steps as step (step)}<li>{step}</li>{/each}
									</ol>
								</Tabs.Content>
							{/each}
						</Tabs.Root>
					</section>

					{#if dirty}
						<!-- Stays in view while choosing a template or what to include, like the card editor's. -->
						<div
							class="bg-card/95 sticky bottom-4 z-10 flex flex-wrap items-center justify-between gap-3 rounded-xl border px-4 py-3 shadow-lg backdrop-blur"
							role="status"
						>
							<p class="text-sm font-medium">Unsaved changes</p>
							<div class="flex items-center gap-2">
								<Button variant="ghost" onclick={() => (settings = JSON.parse(saved))} disabled={saving}>Discard</Button
								>
								<Button onclick={save} disabled={saving}>
									{#if saving}<Spinner data-icon="inline-start" />{/if}
									Save signature
								</Button>
							</div>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	{/if}
</div>
