<script lang="ts">
	import { toast } from 'svelte-sonner';
	import Cropper from 'svelte-easy-crop';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import ZoomInIcon from '@lucide/svelte/icons/zoom-in';
	import ZoomOutIcon from '@lucide/svelte/icons/zoom-out';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Slider } from '$lib/components/ui/slider';
	import { Spinner } from '$lib/components/ui/spinner';
	import { cropImage, rotateImage, type PixelArea } from '$lib/image';

	/**
	 * Lets the user frame a freshly picked image before it is uploaded. Setting
	 * `file` opens the dialog; confirming hands back a new, already-cropped File.
	 */
	let {
		file = $bindable(null),
		title = 'Crop image',
		aspect = 1,
		shape = 'rect',
		outputWidth,
		outputHeight,
		onconfirm
	}: {
		file?: File | null;
		title?: string;
		aspect?: number;
		shape?: 'rect' | 'round';
		outputWidth: number;
		outputHeight: number;
		onconfirm: (file: File) => void;
	} = $props();

	let src = $state('');
	let crop = $state({ x: 0, y: 0 });
	let zoom = $state(1);
	let area: PixelArea | null = null;
	let busy = $state(false);

	// A new file gets a fresh object URL and a reset view; the URL is freed when
	// the file changes or the dialog closes.
	$effect(() => {
		if (!file) return;
		const url = URL.createObjectURL(file);
		src = url;
		crop = { x: 0, y: 0 };
		zoom = 1;
		return () => {
			URL.revokeObjectURL(url);
			if (src !== url) URL.revokeObjectURL(src);
			src = '';
		};
	});

	async function rotate() {
		busy = true;
		try {
			const prev = src;
			src = await rotateImage(prev);
			if (file && prev) URL.revokeObjectURL(prev);
			crop = { x: 0, y: 0 };
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not rotate');
		} finally {
			busy = false;
		}
	}

	async function confirm() {
		if (!file || !area) return;
		busy = true;
		try {
			const cropped = await cropImage(src, area, outputWidth, outputHeight, file.name);
			file = null;
			onconfirm(cropped);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not crop');
		} finally {
			busy = false;
		}
	}
</script>

<Dialog.Root
	open={file !== null}
	onOpenChange={(o) => {
		if (!o && !busy) file = null;
	}}
>
	<Dialog.Content class="sm:max-w-lg">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			<Dialog.Description>Drag to reposition, and use the slider or pinch to zoom.</Dialog.Description>
		</Dialog.Header>

		<div class="bg-muted relative h-72 overflow-hidden rounded-lg sm:h-80">
			{#if src}
				<Cropper
					image={src}
					bind:crop
					bind:zoom
					{aspect}
					cropShape={shape}
					maxZoom={4}
					showGrid={shape === 'rect'}
					oncropcomplete={(e) => (area = e.pixels)}
				/>
			{/if}
		</div>

		<div class="flex items-center gap-3">
			<ZoomOutIcon class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
			<Slider type="single" bind:value={zoom} min={1} max={4} step={0.01} aria-label="Zoom" />
			<ZoomInIcon class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
			<Button variant="outline" size="icon" onclick={rotate} disabled={busy || !src} aria-label="Rotate 90°">
				<RotateCwIcon />
			</Button>
		</div>

		<Dialog.Footer>
			<Button variant="outline" onclick={() => (file = null)} disabled={busy}>Cancel</Button>
			<Button onclick={confirm} disabled={busy || !src}>
				{#if busy}<Spinner data-icon="inline-start" />{/if}
				Apply
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
