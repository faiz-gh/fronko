<script lang="ts">
	import { toast } from 'svelte-sonner';
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { formatBytes, type LibraryFile, type PublicFile } from '$lib/features/files/api';
	import FilePickerDialog from '$lib/features/files/components/file-picker-dialog.svelte';
	import { storage } from '$lib/features/files/storage.svelte';
	import { MAX_DOCUMENTS, newId, type CardData } from '../card';

	let {
		card = $bindable(),
		files,
		onfile
	}: {
		card: CardData;
		/** Names and sizes of the library files the card uses. */
		files: Record<string, PublicFile>;
		onfile: (file: LibraryFile) => void;
	} = $props();

	let docPickerOpen = $state(false);

	function addDocument(file: LibraryFile) {
		if (card.documents.length >= MAX_DOCUMENTS) return;
		onfile(file);
		if (card.documents.some((d) => d.file === file.id)) {
			toast.info('That brochure is already on this card');
			return;
		}
		card.documents.push({ id: newId(), file: file.id, title: file.title || file.name.replace(/\.pdf$/i, '') });
	}

	function moveDocument(index: number, delta: number) {
		const target = index + delta;
		if (target < 0 || target >= card.documents.length) return;
		const [item] = card.documents.splice(index, 1);
		card.documents.splice(target, 0, item);
	}
</script>

<FormSection
	panel
	id="brochures"
	title="Brochures"
	description="PDFs visitors can open from your card: price lists, portfolios, menus."
>
	<div class="flex flex-col gap-2">
		{#each card.documents as doc, i (doc.id)}
			{@const meta = files[doc.file]}
			<div class="bg-card flex items-center gap-2 rounded-xl border p-2">
				<span class="bg-muted text-muted-foreground grid size-9 shrink-0 place-items-center rounded-lg">
					<FileTextIcon class="size-4" />
				</span>
				<div class="flex min-w-0 flex-1 flex-col gap-1">
					<Input
						bind:value={doc.title}
						placeholder={meta?.name ?? 'Brochure title'}
						aria-label="Brochure {i + 1} title"
						maxlength={120}
						class="shadow-none"
					/>
					<span class="text-muted-foreground truncate px-1 text-xs">
						{#if meta}{meta.name} · {formatBytes(meta.size_bytes)}{:else}PDF{/if}
					</span>
				</div>
				<div class="flex shrink-0 max-sm:flex-col">
					<Button
						variant="ghost"
						size="icon"
						disabled={i === 0}
						onclick={() => moveDocument(i, -1)}
						aria-label="Move up"
					>
						<ArrowUpIcon />
					</Button>
					<Button
						variant="ghost"
						size="icon"
						disabled={i === card.documents.length - 1}
						onclick={() => moveDocument(i, 1)}
						aria-label="Move down"
					>
						<ArrowDownIcon />
					</Button>
					<Button variant="ghost" size="icon" onclick={() => card?.documents.splice(i, 1)} aria-label="Remove brochure">
						<XIcon />
					</Button>
				</div>
			</div>
		{/each}
		<button
			type="button"
			onclick={() => (docPickerOpen = true)}
			disabled={!storage.ready || card.documents.length >= MAX_DOCUMENTS}
			class="text-muted-foreground hover:text-foreground hover:border-foreground/30 flex h-12 items-center justify-center gap-2 rounded-xl border border-dashed text-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50"
		>
			<PlusIcon class="size-4" />
			{card.documents.length >= MAX_DOCUMENTS ? `Up to ${MAX_DOCUMENTS} brochures` : 'Add brochure'}
		</button>
		{#if storage.status && !storage.ready}
			<p class="text-muted-foreground text-xs">
				<a href="/dashboard/settings?tab=storage" class="text-foreground underline underline-offset-4"
					>Connect storage</a
				>
				to upload PDFs.
			</p>
		{/if}
	</div>
</FormSection>

<FilePickerDialog
	bind:open={docPickerOpen}
	purpose="brochure"
	title="Add a brochure"
	selected={card.documents.map((d) => d.file)}
	onselect={addDocument}
/>
