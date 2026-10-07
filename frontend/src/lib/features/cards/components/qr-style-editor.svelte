<script lang="ts">
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import ImageIcon from '@lucide/svelte/icons/image';
	import RotateCcwIcon from '@lucide/svelte/icons/rotate-ccw';
	import { PURPOSES, type LibraryFile } from '$lib/features/files/api';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Slider } from '$lib/components/ui/slider';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import FilePickerDialog from '$lib/features/files/components/file-picker-dialog.svelte';
	import FileThumb from '$lib/features/files/components/file-thumb.svelte';
	import QrCode from './qr-code.svelte';
	import { QR_IMAGE_SCALE, defaultQrStyle, type QrCorners, type QrDots, type QrImage, type QrStyle } from '$lib/features/cards/card';
	import { qrContrastIssue } from '$lib/features/cards/qr';
	import { cn } from '$lib/utils';

	let {
		style = $bindable(),
		url,
		orgLogo,
		orgName,
		brandColor
	}: {
		style: QrStyle;
		/** What the code encodes, for the inline preview. */
		url: string;
		/** The organisation's logo file, if it has one. */
		orgLogo?: string | null;
		orgName?: string;
		/** The organisation's brand colour (#rrggbb), offered as a preset. */
		brandColor?: string;
	} = $props();

	let pickerOpen = $state(false);
	const issue = $derived(qrContrastIssue(style));

	const PRESETS = $derived(
		[
			...(brandColor ? [{ fg: brandColor, bg: '#ffffff', label: `${orgName ?? 'Brand'} colour` }] : []),
			{ fg: '#0a0a0a', bg: '#ffffff', label: 'Black' },
			{ fg: '#1e3a8a', bg: '#ffffff', label: 'Navy' },
			{ fg: '#3730a3', bg: '#eef2ff', label: 'Indigo' },
			{ fg: '#065f46', bg: '#ecfdf5', label: 'Forest' },
			{ fg: '#7f1d1d', bg: '#fff7ed', label: 'Burgundy' },
			{ fg: '#374151', bg: '#f9fafb', label: 'Graphite' }
		].filter((p, i, all) => all.findIndex((q) => q.fg === p.fg && q.bg === p.bg) === i)
	);

	const DOTS: { value: QrDots; label: string }[] = [
		{ value: 'square', label: 'Square' },
		{ value: 'rounded', label: 'Rounded' },
		{ value: 'dots', label: 'Dots' }
	];
	const CORNERS: { value: QrCorners; label: string }[] = [
		{ value: 'square', label: 'Square' },
		{ value: 'rounded', label: 'Rounded' },
		{ value: 'dot', label: 'Circle' }
	];

	const HEX = /^#[0-9a-f]{6}$/i;
	// Typed colours apply once they're complete.
	let fgText = $state('');
	let bgText = $state('');
	$effect(() => {
		fgText = style.fg;
		bgText = style.bg;
	});

	function chooseImage(value: QrImage) {
		if (value === 'custom' && !style.image_file) {
			pickerOpen = true;
			return;
		}
		style.image = value;
	}

	function useImage(file: LibraryFile) {
		style.image_file = file.id;
		style.image = 'custom';
	}

	const isDefault = $derived(JSON.stringify(style) === JSON.stringify(defaultQrStyle()));
</script>

{#snippet swatch(color: string)}
	<span class="ring-foreground/15 inline-block size-4 shrink-0 rounded-full ring-1" style="background: {color}"></span>
{/snippet}

<Field.Group>
	<!-- The aside shows a big preview on wide screens; this one is for everything else. -->
	<div class="mx-auto w-full max-w-52 xl:hidden">
		<QrCode {url} {style} class="w-full" />
	</div>

	<Field.Field>
		<Field.Label>Centre image</Field.Label>
		<ToggleGroup.Root
			type="single"
			variant="outline"
			value={style.image}
			onValueChange={(v) => v && chooseImage(v as QrImage)}
			class="w-full"
			aria-label="Centre image"
		>
			<ToggleGroup.Item value="none" class="flex-1">None</ToggleGroup.Item>
			<ToggleGroup.Item value="org_logo" class="flex-1" disabled={!orgLogo}>Organisation logo</ToggleGroup.Item>
			<ToggleGroup.Item value="custom" class="flex-1">Image</ToggleGroup.Item>
		</ToggleGroup.Root>
		{#if style.image === 'custom' && style.image_file}
			<div class="flex items-center gap-3 rounded-xl border p-2">
				<FileThumb id={style.image_file} kind="image" fit="contain" class="size-12 rounded-lg" />
				<span class="text-muted-foreground flex-1 text-sm">Your image, cropped square.</span>
				<Button variant="outline" size="sm" onclick={() => (pickerOpen = true)}>
					<ImageIcon data-icon="inline-start" />
					Change
				</Button>
			</div>
		{/if}
		<Field.Description>
			{#if !orgLogo}
				{orgName ?? 'Your organisation'} has no logo yet, so only your own image is available.
			{:else}
				Error correction goes up automatically when there's an image, so the code still scans.
			{/if}
		</Field.Description>
	</Field.Field>

	{#if style.image !== 'none'}
		<Field.Field>
			<div class="flex items-center justify-between">
				<Field.Label>Image size</Field.Label>
				<span class="text-muted-foreground text-xs tabular">{Math.round(style.image_scale * 100)}%</span>
			</div>
			<Slider
				type="single"
				bind:value={style.image_scale}
				min={QR_IMAGE_SCALE.min}
				max={QR_IMAGE_SCALE.max}
				step={0.01}
				aria-label="Image size"
			/>
			<Field.Description>Smaller images scan more reliably.</Field.Description>
		</Field.Field>
	{/if}

	<Field.Field>
		<Field.Label>Colours</Field.Label>
		<div class="flex flex-wrap gap-2">
			{#each PRESETS as p (p.label)}
				<button
					type="button"
					onclick={() => {
						style.fg = p.fg;
						style.bg = p.bg;
					}}
					class={cn(
						'flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors',
						style.fg === p.fg && style.bg === p.bg ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
					)}
				>
					<span class="flex -space-x-1">{@render swatch(p.bg)}{@render swatch(p.fg)}</span>
					{p.label}
				</button>
			{/each}
		</div>
		<div class="grid gap-3 sm:grid-cols-2">
			{#each [['fg', 'Code', fgText], ['bg', 'Background', bgText]] as const as [key, label, text] (key)}
				<div class="flex flex-col gap-1.5">
					<span class="text-muted-foreground text-xs">{label}</span>
					<div class="flex items-center gap-2">
						<input
							type="color"
							value={style[key]}
							oninput={(e) => (style[key] = e.currentTarget.value)}
							class="ring-foreground/15 size-9 shrink-0 cursor-pointer rounded-lg bg-transparent p-0 ring-1"
							aria-label="{label} colour"
						/>
						<Input
							value={text}
							maxlength={7}
							class="h-9 min-w-0 flex-1 font-mono text-xs uppercase"
							aria-label="{label} colour, hex"
							oninput={(e) => {
								const v = e.currentTarget.value.trim();
								if (key === 'fg') fgText = v;
								else bgText = v;
								if (HEX.test(v)) style[key] = v.toLowerCase();
							}}
						/>
					</div>
				</div>
			{/each}
		</div>
		{#if issue}
			<p class={cn('flex items-start gap-2 text-sm', issue.blocking ? 'text-destructive' : 'text-amber-700 dark:text-amber-400')}>
				<CircleAlertIcon class="mt-0.5 size-4 shrink-0" />
				{issue.message}
			</p>
		{/if}
	</Field.Field>

	<div class="grid gap-6 2xl:grid-cols-2">
		<Field.Field>
			<Field.Label>Dots</Field.Label>
			<ToggleGroup.Root type="single" variant="outline" value={style.dots} onValueChange={(v) => v && (style.dots = v as QrDots)} class="w-full" aria-label="Dots">
				{#each DOTS as d (d.value)}<ToggleGroup.Item value={d.value} class="flex-1">{d.label}</ToggleGroup.Item>{/each}
			</ToggleGroup.Root>
		</Field.Field>
		<Field.Field>
			<Field.Label>Corners</Field.Label>
			<ToggleGroup.Root type="single" variant="outline" value={style.corners} onValueChange={(v) => v && (style.corners = v as QrCorners)} class="w-full" aria-label="Corners">
				{#each CORNERS as c (c.value)}<ToggleGroup.Item value={c.value} class="flex-1">{c.label}</ToggleGroup.Item>{/each}
			</ToggleGroup.Root>
		</Field.Field>
	</div>

	{#if !isDefault}
		<div>
			<Button variant="ghost" size="sm" onclick={() => (style = defaultQrStyle())}>
				<RotateCcwIcon data-icon="inline-start" />
				Back to plain black and white
			</Button>
		</div>
	{/if}
	<Field.Description>
		Always scan a test print before ordering cards. Codes you've already printed keep working: the style only changes how
		new downloads look.
	</Field.Description>
</Field.Group>

<FilePickerDialog
	bind:open={pickerOpen}
	purpose="logo"
	kind="image"
	crop={PURPOSES.logo.crop}
	title="Choose an image for the QR code"
	selected={style.image_file ? [style.image_file] : []}
	onselect={useImage}
/>
