<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import SignatureIcon from '@lucide/svelte/icons/signature';
	import SmartphoneNfcIcon from '@lucide/svelte/icons/smartphone-nfc';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Switch } from '$lib/components/ui/switch';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { session } from '$lib/core/session.svelte';
	import { cn } from '$lib/utils';
	import { TAP_ACTIONS, tapUrl, type CardData, type TapAction, type TapSource } from '../card';
	import type { CardErrors } from './validation';

	let {
		card = $bindable(),
		slug = $bindable(),
		savedSlug,
		profileId,
		errors,
		onnfc
	}: {
		card: CardData;
		/** The link being edited. */
		slug: string;
		/** The saved link, which the tap links and QR code use until saved. */
		savedSlug: string;
		profileId: number;
		errors: CardErrors;
		/** Opens the NFC writer. */
		onnfc: () => void;
	} = $props();

	const slugInvalid = $derived(errors.slug);

	async function copyTapUrl(via: TapSource) {
		try {
			await navigator.clipboard.writeText(tapUrl(session.orgHandle, savedSlug, via));
			toast.success(via === 'nfc' ? 'NFC link copied' : 'QR link copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}
</script>

<FormSection panel id="sharing" title="Sharing" description="Your public address and what visitors can do.">
	<Field.Group class="gap-6">
		<Field.Field data-invalid={slugInvalid || undefined}>
			<Field.Label for="slug">Public link</Field.Label>
			<div class="flex items-stretch">
				<span
					class="text-muted-foreground bg-muted flex max-w-[70%] shrink-0 items-center rounded-l-lg border border-r-0 px-3 font-mono text-xs"
				>
					<span class="hidden min-w-0 truncate 2xl:block">{location.host}</span>
					<span class="truncate">/p/{session.orgHandle}/</span>
				</span>
				<Input
					id="slug"
					class="min-w-0 rounded-l-none font-mono read-only:bg-muted/40 read-only:text-muted-foreground"
					value={slug}
					oninput={(e) => (slug = e.currentTarget.value.toLowerCase())}
					aria-invalid={slugInvalid || undefined}
					readonly={!session.isAdmin}
				/>
			</div>
			{#if !session.isAdmin}
				<Field.Description>
					{session.orgName} manages this link because it may already be printed on cards and QR codes.
				</Field.Description>
			{:else if slugInvalid}
				<Field.Error>3–48 characters: lowercase letters, numbers and hyphens.</Field.Error>
			{:else if slug !== savedSlug}
				<Field.Description class="text-amber-700 dark:text-amber-400">
					Changing this breaks the old link and any QR codes or NFC cards already printed.
				</Field.Description>
			{:else}
				<Field.Description>The address of your card. NFC tags and QR codes use it too.</Field.Description>
			{/if}
		</Field.Field>
		<Field.Field orientation="horizontal" class="bg-card rounded-xl border p-4">
			<Field.Content>
				<Field.Label for="collect">Collect leads</Field.Label>
				<Field.Description>Show a “Share your contact” button so visitors can leave their details.</Field.Description>
			</Field.Content>
			<Switch id="collect" bind:checked={card.collect_leads} />
		</Field.Field>
		{#each [['nfc', 'NFC tap', 'This is the link on your NFC card.'], ['qr', 'QR code scan', 'Your QR code already points here.']] as const as [via, label, hint] (via)}
			<Field.Field>
				<Field.Label id="tap-{via}-label">When someone uses your {label}</Field.Label>
				<div class="grid gap-2 sm:grid-cols-3" role="radiogroup" aria-labelledby="tap-{via}-label">
					{#each Object.entries(TAP_ACTIONS) as [action, info] (action)}
						{@const unavailable = action === 'lead_form' && !card.collect_leads}
						<button
							type="button"
							role="radio"
							aria-checked={card.tap[via] === action}
							disabled={unavailable}
							onclick={() => card && (card.tap[via] = action as TapAction)}
							class={cn(
								'flex flex-col gap-0.5 rounded-xl border p-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50',
								card.tap[via] === action ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
							)}
						>
							<span class="text-sm font-medium">{info.label}</span>
							<span class="text-muted-foreground text-xs leading-snug">
								{unavailable ? 'Turn on Collect leads to use this.' : info.description}
							</span>
						</button>
					{/each}
				</div>
				{#if card.tap[via] === 'lead_form' && !card.collect_leads}
					<Field.Error>Collect leads is off, so visitors will just see your card.</Field.Error>
				{/if}
				<div class="flex items-center gap-2">
					<code class="bg-muted text-muted-foreground min-w-0 flex-1 truncate rounded-lg px-3 py-2 font-mono text-xs">
						{tapUrl(session.orgHandle, savedSlug, via)}
					</code>
					<Button variant="outline" size="icon" onclick={() => copyTapUrl(via)} aria-label="Copy {label} link">
						<CopyIcon />
					</Button>
					<Button
						variant="outline"
						href={tapUrl(session.orgHandle, savedSlug, via)}
						target="_blank"
						title="Try it (uses the saved settings)"
					>
						Try it
						<ExternalLinkIcon data-icon="inline-end" />
					</Button>
				</div>
				<Field.Description>{hint} Changes apply as soon as you save; nothing needs re-writing.</Field.Description>
			</Field.Field>
		{/each}
		<Field.Field orientation="horizontal" class="bg-card rounded-xl border p-4">
			<Field.Content>
				<Field.Label>NFC card</Field.Label>
				<Field.Description>Put this card's NFC link on the chip inside your metal or plastic card.</Field.Description>
			</Field.Content>
			<Button variant="outline" onclick={onnfc}>
				<SmartphoneNfcIcon data-icon="inline-start" />
				Write to NFC card
			</Button>
		</Field.Field>
		<Field.Field orientation="horizontal" class="bg-card rounded-xl border p-4">
			<Field.Content>
				<Field.Label>Email signature</Field.Label>
				<Field.Description>Turn this card into a signature for Gmail, Outlook or Apple Mail.</Field.Description>
			</Field.Content>
			<Button variant="outline" href="/dashboard/signatures?card={profileId}">
				<SignatureIcon data-icon="inline-start" />
				Create signature
			</Button>
		</Field.Field>
	</Field.Group>
</FormSection>
