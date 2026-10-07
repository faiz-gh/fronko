<script lang="ts">
	import { onDestroy } from 'svelte';
	import { toast } from 'svelte-sonner';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import LightbulbIcon from '@lucide/svelte/icons/lightbulb';
	import SmartphoneNfcIcon from '@lucide/svelte/icons/smartphone-nfc';
	import { Button } from '$lib/components/ui/button';
	import QrCode from './qr-code.svelte';
	import { nfcErrorMessage, nfcSupport, writeNfcUrl } from '$lib/features/cards/nfc';

	let {
		url,
		writerUrl,
		name
	}: {
		/** What goes on the chip: the card's link marked as an NFC tap. */
		url: string;
		/** This writer on its own page, for the desktop's "open on your phone" QR code. */
		writerUrl: string;
		/** The card's name, for messages. */
		name: string;
	} = $props();

	const support = nfcSupport();
	// Long enough to find the spot on a metal card, short enough not to leave NFC on.
	const TIMEOUT_MS = 60_000;

	let status = $state<'idle' | 'writing' | 'done' | 'error'>('idle');
	let error = $state('');
	let controller: AbortController | null = null;

	async function write() {
		controller?.abort();
		const current = new AbortController();
		controller = current;
		let timedOut = false;
		const timer = setTimeout(() => {
			timedOut = true;
			current.abort();
		}, TIMEOUT_MS);
		status = 'writing';
		error = '';
		try {
			await writeNfcUrl(url, current.signal);
			if (controller !== current) return;
			status = 'done';
			navigator.vibrate?.(80);
		} catch (e) {
			if (controller !== current) return;
			// Cancelling isn't a failure worth a red message.
			if (e instanceof DOMException && e.name === 'AbortError' && !timedOut) {
				status = 'idle';
				return;
			}
			status = 'error';
			error = nfcErrorMessage(e, timedOut);
		} finally {
			clearTimeout(timer);
		}
	}

	function cancel() {
		controller?.abort();
	}

	onDestroy(() => controller?.abort());

	async function copy() {
		try {
			await navigator.clipboard.writeText(url);
			toast.success('NFC link copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}
</script>

{#snippet metalTip()}
	<div class="bg-muted/50 flex gap-3 rounded-xl border p-3 text-sm">
		<LightbulbIcon class="text-muted-foreground mt-0.5 size-4 shrink-0" />
		<p class="text-muted-foreground leading-snug">
			<span class="text-foreground font-medium">Metal card?</span> The chip only reads through a small window, usually near
			the logo or one edge. Slide the card slowly over the back of your phone near the camera, and hold it still for a second
			or two once it's found.
		</p>
	</div>
{/snippet}

<div class="flex flex-col gap-4">
	{#if support === 'web-nfc'}
		{#if status === 'writing'}
			<div class="flex flex-col items-center gap-3 py-4 text-center" role="status">
				<span class="relative flex size-16 items-center justify-center">
					<span class="bg-primary/20 absolute inset-0 animate-ping rounded-full motion-reduce:animate-none"></span>
					<span class="bg-primary/10 relative flex size-16 items-center justify-center rounded-full">
						<SmartphoneNfcIcon class="text-primary size-7" />
					</span>
				</span>
				<p class="font-medium">Hold your card to the back of your phone</p>
				<p class="text-muted-foreground text-sm">Keep it still until it's done.</p>
				<Button variant="outline" onclick={cancel}>Cancel</Button>
			</div>
		{:else if status === 'done'}
			<div class="flex flex-col items-center gap-2 py-4 text-center" role="status">
				<CircleCheckIcon class="size-10 text-emerald-600 dark:text-emerald-400" />
				<p class="font-medium">Card written</p>
				<p class="text-muted-foreground text-sm">Tap it on your phone to check it opens {name || 'the card'}.</p>
				<Button variant="outline" onclick={write}>Write another card</Button>
			</div>
		{:else}
			{#if status === 'error'}
				<div class="border-destructive/30 bg-destructive/5 flex gap-3 rounded-xl border p-3 text-sm" role="alert">
					<CircleAlertIcon class="text-destructive mt-0.5 size-4 shrink-0" />
					<p class="leading-snug">{error}</p>
				</div>
			{/if}
			<Button size="lg" onclick={write}>
				<SmartphoneNfcIcon data-icon="inline-start" />
				{status === 'error' ? 'Try again' : 'Write to NFC card'}
			</Button>
			<p class="text-muted-foreground text-center text-xs">
				Chrome asks once for permission to use NFC. Anything already on the card is replaced.
			</p>
		{/if}
		{@render metalTip()}
	{:else}
		<Button size="lg" onclick={copy}>
			<CopyIcon data-icon="inline-start" />
			Copy NFC link
		</Button>
		<div class="flex flex-col gap-2 text-sm">
			<p class="font-medium">
				{support === 'desktop'
					? 'Write it with the free NFC Tools app on your phone'
					: 'Write it with the free NFC Tools app'}
			</p>
			<ol class="text-muted-foreground list-decimal space-y-1 pl-5 leading-snug">
				{#if support === 'desktop'}
					<li>Send yourself this link, or scan the QR code below to copy it on your phone.</li>
				{/if}
				<li>
					Install <span class="text-foreground">NFC Tools</span> from the {support === 'ios'
						? 'App Store'
						: support === 'android'
							? 'Play Store'
							: 'App Store or Play Store'}.
				</li>
				<li>
					Open <span class="text-foreground">Write</span> → <span class="text-foreground">Add a record</span> →
					<span class="text-foreground">URL / URI</span>, paste the link and tap OK.
				</li>
				<li>
					Tap <span class="text-foreground">Write</span> and hold the card to {support === 'ios'
						? 'the top edge of your iPhone'
						: 'the back of your phone'} until it's done.
				</li>
			</ol>
		</div>
		{@render metalTip()}
		{#if support === 'desktop'}
			<div class="flex items-center gap-4 rounded-xl border p-3">
				<QrCode url={writerUrl} class="size-28 shrink-0" />
				<div class="flex min-w-0 flex-col gap-1 text-sm">
					<p class="font-medium">Have an Android phone?</p>
					<p class="text-muted-foreground leading-snug">
						Scan this with it and open the page in Chrome to write the card directly, no app needed.
					</p>
				</div>
			</div>
		{/if}
		<code class="bg-muted text-muted-foreground truncate rounded-lg px-3 py-2 font-mono text-xs">{url}</code>
	{/if}
</div>
