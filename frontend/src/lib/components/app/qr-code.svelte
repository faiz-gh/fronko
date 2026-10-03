<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { qrSvg } from '$lib/card/qr';
	import { cn } from '$lib/utils';

	let {
		url,
		svg = $bindable(''),
		class: className
	}: {
		url: string;
		/** The generated markup, exposed so callers can offer an SVG download. */
		svg?: string;
		class?: string;
	} = $props();

	$effect(() => {
		if (!url) return;
		const target = url;
		svg = '';
		qrSvg(target)
			.then((markup) => {
				if (target === url) svg = markup;
			})
			.catch(() => toast.error('Could not generate the QR code'));
	});
</script>

<div class={cn('rounded-2xl bg-white p-3 shadow-sm ring-1 ring-black/5', className)}>
	{#if svg}
		<!-- Generated locally by `qrcode` from our own URL, so the markup is trusted. -->
		<div class="aspect-square [&>svg]:size-full" role="img" aria-label="QR code for {url}">
			{@html svg}
		</div>
	{:else}
		<Skeleton class="aspect-square w-full bg-neutral-100" />
	{/if}
</div>
