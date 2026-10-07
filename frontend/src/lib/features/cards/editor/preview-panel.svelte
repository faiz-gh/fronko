<script lang="ts">
	import DownloadIcon from '@lucide/svelte/icons/download';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import { Button } from '$lib/components/ui/button';
	import { session } from '$lib/core/session.svelte';
	import { branding } from '$lib/features/branding/store.svelte';
	import type { PublicFile } from '$lib/features/files/api';
	import type { Booking } from '../api';
	import { cn } from '$lib/utils';
	import { tapUrl, type CardData } from '../card';
	import ProfileCard from '../components/profile-card.svelte';
	import QrCode from '../components/qr-code.svelte';
	import { downloadQrPng, downloadQrSvg } from '../qr';

	let {
		card,
		savedSlug,
		slug,
		files,
		booking,
		mode: previewMode = $bindable('card')
	}: {
		card: CardData;
		/** The saved link, which the public page and QR code use. */
		savedSlug: string;
		/** The link being edited, to warn when the QR code doesn't match it yet. */
		slug: string;
		files: Record<string, PublicFile>;
		booking?: Booking | null;
		mode?: 'card' | 'qr';
	} = $props();

	let qrMarkup = $state('');
</script>

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
							previewMode === mode
								? 'bg-background text-foreground shadow-sm'
								: 'text-muted-foreground hover:text-foreground'
						)}
					>
						{label}
					</button>
				{/each}
			</div>
			<Button variant="ghost" size="sm" href="/p/{session.orgHandle}/{savedSlug}" target="_blank">
				Open
				<ExternalLinkIcon data-icon="inline-end" />
			</Button>
		</div>

		<div class="-mx-2 flex min-h-0 flex-1 justify-center overflow-y-auto px-2 py-2">
			{#if previewMode === 'card'}
				<div class="my-auto w-full max-w-[380px]">
					<ProfileCard {card} slug={savedSlug} {files} org={branding.value} {booking} />
				</div>
			{:else}
				<div class="my-auto flex w-full max-w-[300px] flex-col items-center gap-4">
					<QrCode url={tapUrl(session.orgHandle, savedSlug, 'qr')} style={card.qr} bind:svg={qrMarkup} class="w-full" />
					<p class="text-muted-foreground max-w-full truncate font-mono text-xs">
						{tapUrl(session.orgHandle, savedSlug, 'qr')}
					</p>
					<div class="grid w-full grid-cols-2 gap-2">
						<Button variant="outline" onclick={() => downloadQrSvg(qrMarkup, savedSlug)} disabled={!qrMarkup}>
							<DownloadIcon data-icon="inline-start" />
							SVG
						</Button>
						<Button onclick={() => downloadQrPng(qrMarkup, savedSlug)} disabled={!qrMarkup}>
							<DownloadIcon data-icon="inline-start" />
							PNG
						</Button>
					</div>
					{#if slug !== savedSlug}
						<p class="text-muted-foreground text-center text-xs">This code uses your saved link. Save to update it.</p>
					{/if}
				</div>
			{/if}
		</div>
	</div>
</aside>
