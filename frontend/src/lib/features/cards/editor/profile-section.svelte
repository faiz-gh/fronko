<script lang="ts">
	import { toast } from 'svelte-sonner';
	import ImagesIcon from '@lucide/svelte/icons/images';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import ImageCropDialog from '$lib/components/shared/image-crop-dialog.svelte';
	import { ACCEPT, checkUpload, PURPOSES, uploadFile, type LibraryFile } from '$lib/features/files/api';
	import FilePickerDialog from '$lib/features/files/components/file-picker-dialog.svelte';
	import { storage } from '$lib/features/files/storage.svelte';
	import { ACCENTS, coverSrc, type CardData } from '../card';
	import CardAvatar from '../components/card-avatar.svelte';
	import type { CardErrors } from './validation';

	let {
		card = $bindable(),
		slug,
		errors,
		onfile
	}: {
		card: CardData;
		/** The saved link, shown in place of initials while there's no name. */
		slug: string;
		errors: CardErrors;
		/** Called with every library file the card starts using. */
		onfile: (file: LibraryFile) => void;
	} = $props();

	let photoPickerOpen = $state(false);
	let photoProgress = $state<number | null>(null);
	let photoInput: HTMLInputElement | undefined = $state();
	let coverPickerOpen = $state(false);
	let coverProgress = $state<number | null>(null);
	let coverInput: HTMLInputElement | undefined = $state();
	// A freshly picked image waiting in the crop dialog.
	let cropPhoto = $state<File | null>(null);
	let cropCover = $state<File | null>(null);

	const avatarInvalid = $derived(errors.avatar);
	const cover = $derived(coverSrc(card));

	function usePhoto(file: LibraryFile) {
		onfile(file);
		card.avatar_file = file.id;
		card.avatar_url = '';
	}

	/** Checks a picked image, then opens the crop dialog for it. */
	function pickImage(file: File, target: 'photo' | 'cover') {
		const problem = checkUpload(file, 'image');
		if (problem) {
			toast.error(problem);
			return;
		}
		if (target === 'photo') cropPhoto = file;
		else cropCover = file;
	}

	async function uploadPhoto(file: File) {
		photoProgress = 0;
		try {
			usePhoto(await uploadFile(file, { purpose: 'avatar', onProgress: (p) => (photoProgress = p) }));
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Upload failed');
		} finally {
			photoProgress = null;
		}
	}

	function useCover(file: LibraryFile) {
		onfile(file);
		card.cover_file = file.id;
	}

	async function uploadCover(file: File) {
		coverProgress = 0;
		try {
			useCover(await uploadFile(file, { purpose: 'cover', onProgress: (p) => (coverProgress = p) }));
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Upload failed');
		} finally {
			coverProgress = null;
		}
	}

	function removePhoto() {
		card.avatar_file = '';
		card.avatar_url = '';
	}
</script>

<FormSection panel id="profile" title="Profile" description="Who you are. Shown at the top of your card.">
	<Field.Group class="grid gap-5 sm:grid-cols-2">
		<Field.Field class="sm:col-span-2">
			<Field.Label for="name">Full name</Field.Label>
			<Input id="name" bind:value={card.name} placeholder="Jane Doe" maxlength={80} />
		</Field.Field>
		<Field.Field>
			<Field.Label for="title">Job title</Field.Label>
			<Input id="title" bind:value={card.title} placeholder="Product Designer" maxlength={80} />
		</Field.Field>
		<Field.Field>
			<Field.Label for="company">Company</Field.Label>
			<Input id="company" bind:value={card.company} placeholder="Acme Inc." maxlength={80} />
		</Field.Field>
		<Field.Field class="sm:col-span-2">
			<Field.Label for="location">Location</Field.Label>
			<Input id="location" bind:value={card.location} placeholder="Berlin, Germany" maxlength={80} />
		</Field.Field>
		<Field.Field class="sm:col-span-2">
			<div class="flex items-baseline justify-between">
				<Field.Label for="bio">Short bio</Field.Label>
				<span class="text-muted-foreground tabular text-xs">{card.bio.length}/280</span>
			</div>
			<Textarea
				id="bio"
				bind:value={card.bio}
				rows={3}
				maxlength={280}
				placeholder="A sentence or two about what you do."
			/>
		</Field.Field>
		<Field.Field class="sm:col-span-2" data-invalid={avatarInvalid || undefined}>
			<Field.Label>Photo</Field.Label>
			<div class="flex flex-wrap items-center gap-4">
				<div class="relative">
					<CardAvatar {card} fallback={slug} class="size-16 text-lg" />
					{#if photoProgress !== null}
						<span class="bg-background/70 absolute inset-0 grid place-items-center rounded-full">
							<Spinner class="size-5" />
						</span>
					{/if}
				</div>
				<div class="flex flex-wrap gap-2">
					<Button
						variant="outline"
						onclick={() => photoInput?.click()}
						disabled={!storage.ready || photoProgress !== null}
					>
						<UploadIcon data-icon="inline-start" />
						{photoProgress !== null ? `Uploading ${Math.round(photoProgress * 100)}%` : 'Upload photo'}
					</Button>
					<Button variant="outline" onclick={() => (photoPickerOpen = true)} disabled={!storage.ready}>
						<ImagesIcon data-icon="inline-start" />
						From library
					</Button>
					{#if card.avatar_file || card.avatar_url}
						<Button variant="ghost" onclick={removePhoto}>Remove</Button>
					{/if}
				</div>
				<input
					bind:this={photoInput}
					type="file"
					accept={ACCEPT.image}
					class="sr-only"
					tabindex="-1"
					onchange={(e) => {
						const f = e.currentTarget.files?.[0];
						if (f) pickImage(f, 'photo');
						e.currentTarget.value = '';
					}}
				/>
			</div>
			{#if storage.status && !storage.ready}
				<Field.Description>
					<a href="/dashboard/settings?tab=storage" class="text-foreground underline underline-offset-4"
						>Connect storage</a
					>
					to upload a photo, or paste an image URL below.
				</Field.Description>
			{/if}
			{#if !card.avatar_file}
				<Input
					id="avatar"
					type="url"
					bind:value={card.avatar_url}
					placeholder="Or paste an image URL: https://…/me.jpg"
					aria-label="Photo URL"
					aria-invalid={avatarInvalid || undefined}
				/>
				{#if avatarInvalid}
					<Field.Error>Enter a valid http(s) image URL.</Field.Error>
				{:else}
					<Field.Description>JPEG, PNG or WebP up to 5 MB. Leave empty to show your initials.</Field.Description>
				{/if}
			{/if}
		</Field.Field>
		<Field.Field class="sm:col-span-2">
			<Field.Label>Cover image</Field.Label>
			<div class="flex flex-wrap items-center gap-4">
				<div
					class="relative aspect-3/1 w-48 shrink-0 overflow-hidden rounded-lg border"
					style:background={cover ? undefined : ACCENTS[card.accent]}
				>
					{#if cover}
						<img src={cover} alt="" class="size-full object-cover" />
					{/if}
					{#if coverProgress !== null}
						<span class="bg-background/70 absolute inset-0 grid place-items-center">
							<Spinner class="size-5" />
						</span>
					{/if}
				</div>
				<div class="flex flex-wrap gap-2">
					<Button
						variant="outline"
						onclick={() => coverInput?.click()}
						disabled={!storage.ready || coverProgress !== null}
					>
						<UploadIcon data-icon="inline-start" />
						{coverProgress !== null ? `Uploading ${Math.round(coverProgress * 100)}%` : 'Upload cover'}
					</Button>
					<Button variant="outline" onclick={() => (coverPickerOpen = true)} disabled={!storage.ready}>
						<ImagesIcon data-icon="inline-start" />
						From library
					</Button>
					{#if card.cover_file}
						<Button variant="ghost" onclick={() => card && (card.cover_file = '')}>Remove</Button>
					{/if}
				</div>
				<input
					bind:this={coverInput}
					type="file"
					accept={ACCEPT.image}
					class="sr-only"
					tabindex="-1"
					onchange={(e) => {
						const f = e.currentTarget.files?.[0];
						if (f) pickImage(f, 'cover');
						e.currentTarget.value = '';
					}}
				/>
			</div>
			<Field.Description>
				A wide banner behind your photo, cropped to 3:1. Without one, the accent colour is used.
			</Field.Description>
		</Field.Field>
	</Field.Group>
</FormSection>

<FilePickerDialog
	bind:open={photoPickerOpen}
	purpose="avatar"
	crop={PURPOSES.avatar.crop}
	title="Choose a photo"
	selected={card.avatar_file ? [card.avatar_file] : []}
	onselect={usePhoto}
/>
<FilePickerDialog
	bind:open={coverPickerOpen}
	purpose="cover"
	crop={PURPOSES.cover.crop}
	title="Choose a cover image"
	selected={card.cover_file ? [card.cover_file] : []}
	onselect={useCover}
/>
<ImageCropDialog
	bind:file={cropPhoto}
	title="Crop photo"
	aspect={1}
	shape="round"
	outputWidth={512}
	outputHeight={512}
	onconfirm={uploadPhoto}
/>
<ImageCropDialog
	bind:file={cropCover}
	title="Crop cover image"
	aspect={3}
	outputWidth={1500}
	outputHeight={500}
	onconfirm={uploadCover}
/>
