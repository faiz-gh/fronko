<script lang="ts">
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import Share2Icon from '@lucide/svelte/icons/share-2';
	import QrCodeIcon from '@lucide/svelte/icons/qr-code';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import SendIcon from '@lucide/svelte/icons/send';
	import { getProfileBySlug, type PublicProfile } from '$lib/api/profile';
	import { submitLead } from '$lib/api/lead';
	import { ApiError } from '$lib/api/client';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Empty from '$lib/components/ui/empty';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';
	import ProfileCard from '$lib/components/app/profile-card.svelte';
	import QrDialog from '$lib/components/app/qr-dialog.svelte';
	import { ACCENTS, downloadVCard, normalizeCard, publicUrl } from '$lib/card/card';
	import { cn } from '$lib/utils';

	const slug = page.params.slug ?? '';

	let profile = $state<PublicProfile | null>(null);
	let notFound = $state(false);
	let loadError = $state('');

	const card = $derived(profile ? normalizeCard(profile.data) : null);
	const files = $derived(Object.fromEntries((profile?.files ?? []).map((f) => [f.id, f])));

	async function load() {
		loadError = '';
		try {
			profile = await getProfileBySlug(slug);
		} catch (e) {
			if (e instanceof ApiError && e.status === 404) notFound = true;
			else loadError = e instanceof Error ? e.message : 'Failed to load';
		}
	}
	load();

	let qrOpen = $state(false);

	// ---- Lead form -------------------------------------------------------------
	let open = $state(false);
	let leadName = $state('');
	let leadEmail = $state('');
	let leadNotes = $state('');
	let submitting = $state(false);
	let submitError = $state('');
	let submitted = $state(false);

	async function handleLeadSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (!profile) return;
		submitError = '';
		submitting = true;
		try {
			await submitLead(profile.id, leadName, leadEmail, leadNotes);
			submitted = true;
		} catch (e) {
			submitError = e instanceof Error ? e.message : 'Failed to send';
		} finally {
			submitting = false;
		}
	}

	function onOpenChange(next: boolean) {
		// Reset after a successful send so the form is fresh next time.
		if (!next && submitted) {
			submitted = false;
			leadName = '';
			leadEmail = '';
			leadNotes = '';
		}
	}

	async function share() {
		const url = publicUrl(slug);
		if (navigator.share) {
			try {
				await navigator.share({ title: card?.name || slug, url });
			} catch {
				// User dismissed the share sheet.
			}
			return;
		}
		try {
			await navigator.clipboard.writeText(url);
			toast.success('Link copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}
</script>

<svelte:head>
	<title>{card ? `${card.name || slug}${card.title ? ` · ${card.title}` : ''}` : 'Fronko'}</title>
	{#if card?.bio}
		<meta name="description" content={card.bio} />
	{/if}
</svelte:head>

<div class={cn(card?.theme === 'dark' && 'dark')} style={card ? `--card-accent: ${ACCENTS[card.accent]}` : undefined}>
	<!-- A faint wash of the card's accent so the page feels like the card's own. -->
	<div
		class="bg-background text-foreground flex min-h-svh flex-col items-center px-4 py-8 sm:py-14 lg:justify-center lg:py-16"
		style={card
			? 'background-image: radial-gradient(80% 60% at 50% 0%, color-mix(in oklch, var(--card-accent) 14%, transparent), transparent 70%)'
			: undefined}
	>
		<main class={cn('flex w-full flex-1 flex-col lg:flex-none', card ? 'max-w-sm lg:max-w-4xl' : 'max-w-sm')}>
			{#if notFound || loadError}
				<Empty.Root class="flex-1">
					<Empty.Header>
						<Empty.Title>{notFound ? 'Card not found' : "Couldn't load this card"}</Empty.Title>
						<Empty.Description>
							{notFound ? `There's no card at /p/${slug}. Check the link and try again.` : loadError}
						</Empty.Description>
					</Empty.Header>
					<Empty.Content>
						{#if loadError}
							<Button variant="outline" onclick={load}>Try again</Button>
						{:else}
							<Button variant="outline" href="/">Go to Fronko</Button>
						{/if}
					</Empty.Content>
				</Empty.Root>
			{:else if !card || !profile}
				<Skeleton class="h-[520px] w-full rounded-3xl" />
			{:else}
				<ProfileCard {card} slug={profile.slug} {files}>
					{#snippet actions()}
						<Button
							size="lg"
							class="h-11 w-full text-white hover:opacity-90"
							style="background: var(--card-accent)"
							onclick={() => card && downloadVCard(card, publicUrl(profile!.slug), profile!.slug)}
						>
							<UserPlusIcon data-icon="inline-start" />
							Save contact
						</Button>
						<div class="flex gap-2">
							{#if card.collect_leads}
								<Button size="lg" variant="outline" class="h-11 flex-1" onclick={() => (open = true)}>
									<SendIcon data-icon="inline-start" />
									Share your contact
								</Button>
							{/if}
							<Button size="lg" variant="outline" class="h-11 px-3.5" onclick={() => (qrOpen = true)} aria-label="Show QR code">
								<QrCodeIcon />
							</Button>
							<Button
								size="lg"
								variant="outline"
								class={cn('h-11', card.collect_leads ? 'px-3.5' : 'flex-1')}
								onclick={share}
								aria-label="Share this card"
							>
								<Share2Icon data-icon={card.collect_leads ? undefined : 'inline-start'} />
								{#if !card.collect_leads}Share{/if}
							</Button>
						</div>
					{/snippet}
				</ProfileCard>
			{/if}
		</main>

		<footer class="text-muted-foreground mt-8 text-xs">
			<a href="/" class="hover:text-foreground inline-flex items-center gap-1.5">
				Made with <span class="text-foreground font-medium">Fronko</span>
			</a>
		</footer>
	</div>
</div>

{#if profile}
	<QrDialog bind:open={qrOpen} slug={profile.slug} name={card?.name ?? ''} dark={card?.theme === 'dark'} />
{/if}

<Dialog.Root bind:open {onOpenChange}>
	<Dialog.Content class={cn('sm:max-w-md', card?.theme === 'dark' && 'dark')}>
		{#if submitted}
			<div class="flex flex-col items-center gap-3 py-6 text-center">
				<CircleCheckIcon class="text-primary size-10" />
				<Dialog.Title>Sent!</Dialog.Title>
				<Dialog.Description>
					{card?.name || slug} now has your details and can get back to you.
				</Dialog.Description>
				<Dialog.Close>
					{#snippet child({ props })}
						<Button variant="outline" class="mt-2" {...props}>Done</Button>
					{/snippet}
				</Dialog.Close>
			</div>
		{:else}
			<form onsubmit={handleLeadSubmit} class="flex flex-col gap-4">
				<Dialog.Header>
					<Dialog.Title>Share your contact</Dialog.Title>
					<Dialog.Description>
						Leave your details for {card?.name || slug}. They'll only be visible to them.
					</Dialog.Description>
				</Dialog.Header>
				<Field.Group>
					<Field.Field>
						<Field.Label for="lead-name">Name</Field.Label>
						<Input id="lead-name" autocomplete="name" bind:value={leadName} required maxlength={120} />
					</Field.Field>
					<Field.Field>
						<Field.Label for="lead-email">Email</Field.Label>
						<Input
							id="lead-email"
							type="email"
							autocomplete="email"
							bind:value={leadEmail}
							required
							maxlength={254}
						/>
					</Field.Field>
					<Field.Field data-invalid={!!submitError || undefined}>
						<Field.Label for="lead-notes">Message <span class="text-muted-foreground font-normal">(optional)</span></Field.Label>
						<Textarea
							id="lead-notes"
							bind:value={leadNotes}
							rows={3}
							maxlength={2000}
							placeholder="Great meeting you at…"
						/>
						{#if submitError}
							<Field.Error>{submitError}</Field.Error>
						{/if}
					</Field.Field>
				</Field.Group>
				<Dialog.Footer>
					<Button type="submit" class="w-full sm:w-auto" disabled={submitting || !leadName.trim() || !leadEmail.trim()}>
						{#if submitting}
							<Spinner data-icon="inline-start" />
						{/if}
						Send
					</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>
