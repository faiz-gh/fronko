<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import QrCode from './qr-code.svelte';
	import { tapUrl, type QrStyle } from '$lib/card/card';
	import { downloadQrPng, downloadQrSvg } from '$lib/card/qr';
	import { cn } from '$lib/utils';

	let {
		open = $bindable(false),
		slug,
		name,
		style,
		dark = false
	}: {
		open?: boolean;
		slug: string;
		name: string;
		/** The card's QR style. */
		style?: QrStyle;
		/** Match a dark-themed card when shown on the public page. */
		dark?: boolean;
	} = $props();

	// Marked as a QR visit, so the card's QR tap behaviour applies.
	const url = $derived(slug ? tapUrl(slug, 'qr') : '');
	let svg = $state('');

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(url);
			toast.success('Link copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class={cn('sm:max-w-sm', dark && 'dark')}>
		<Dialog.Header>
			<Dialog.Title>QR code</Dialog.Title>
			<Dialog.Description>Scan to open {name || slug}'s card.</Dialog.Description>
		</Dialog.Header>

		{#if open && url}
			<QrCode {url} {style} bind:svg class="mx-auto w-full max-w-64" />
		{/if}

		<button
			type="button"
			onclick={copyLink}
			class="text-muted-foreground hover:text-foreground mx-auto flex max-w-full items-center gap-1.5 truncate font-mono text-xs"
		>
			<span class="truncate">{url}</span>
			<CopyIcon class="size-3.5 shrink-0" />
		</button>

		<Dialog.Footer class="grid grid-cols-2 gap-2 sm:flex">
			<Button variant="outline" onclick={() => downloadQrSvg(svg, slug)} disabled={!svg}>
				<DownloadIcon data-icon="inline-start" />
				SVG
			</Button>
			<Button onclick={() => downloadQrPng(svg, slug)} disabled={!svg}>
				<DownloadIcon data-icon="inline-start" />
				PNG
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
