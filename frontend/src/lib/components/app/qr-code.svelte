<script lang="ts">
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { branding } from '$lib/branding.svelte';
	import { defaultQrStyle, type QrStyle } from '$lib/card/card';
	import { qrImageFile, qrSvg } from '$lib/card/qr';
	import { cn } from '$lib/utils';

	let {
		url,
		style,
		svg = $bindable(''),
		class: className
	}: {
		url: string;
		/** The card's QR style; the plain black-on-white code when omitted. */
		style?: QrStyle;
		/** The generated markup, exposed so callers can offer downloads. */
		svg?: string;
		class?: string;
	} = $props();

	const resolved = $derived(style ?? defaultQrStyle());
	const imageFile = $derived(qrImageFile(resolved, branding.value?.logo_file));
	// Regenerate only when something that changes the code changes.
	const key = $derived(JSON.stringify([url, resolved, imageFile]));

	let timer: ReturnType<typeof setTimeout> | undefined;
	$effect(() => {
		const current = key;
		if (!url) return;
		const target = url;
		const opts = { style: $state.snapshot(resolved), imageFile };
		// Colour pickers fire continuously; wait for a pause.
		clearTimeout(timer);
		timer = setTimeout(() => {
			qrSvg(target, opts)
				.then((markup) => {
					if (current === key) svg = markup;
				})
				.catch(() => toast.error('Could not generate the QR code'));
		}, untrack(() => svg) ? 120 : 0);
		return () => clearTimeout(timer);
	});
</script>

<div
	class={cn('rounded-2xl p-1 shadow-sm ring-1 ring-black/5', className)}
	style="background: {resolved.bg}"
>
	{#if svg}
		<!-- Built locally from our own URL and style (colours validated as #rrggbb), so the markup is trusted. -->
		<div class="aspect-square [&>svg]:size-full" role="img" aria-label="QR code for {url}">
			{@html svg}
		</div>
	{:else}
		<Skeleton class="aspect-square w-full bg-neutral-100" />
	{/if}
</div>
