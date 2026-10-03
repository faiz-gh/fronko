<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import { listFiles, formatBytes, type FileKind, type LibraryFile } from '$lib/api/files';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import FileDropzone from './file-dropzone.svelte';
	import FileThumb from './file-thumb.svelte';
	import Pagination from './pagination.svelte';
	import { cn } from '$lib/utils';

	let {
		open = $bindable(false),
		kind,
		title,
		selected = [],
		onselect
	}: {
		open?: boolean;
		kind: FileKind;
		title: string;
		/** Ids already in use, marked in the grid. */
		selected?: string[];
		onselect: (file: LibraryFile) => void;
	} = $props();

	let files = $state<LibraryFile[] | null>(null);
	let total = $state(0);
	let page = $state(1);
	let pageSize = $state(12);
	let error = $state('');

	$effect(() => {
		if (!open) return;
		const query = { kind, page, pageSize };
		error = '';
		listFiles(query)
			.then((res) => {
				files = res.files;
				total = res.total;
			})
			.catch((e) => (error = e instanceof Error ? e.message : 'Failed to load files'));
	});

	function choose(file: LibraryFile) {
		onselect(file);
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="gap-5 sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			<Dialog.Description>
				Pick from your library or upload a new {kind === 'image' ? 'photo' : 'PDF'}. Uploads are saved to your library.
			</Dialog.Description>
		</Dialog.Header>

		<FileDropzone {kind} compact multiple={false} onuploaded={choose} />

		{#if error}
			<p class="text-destructive text-sm">{error}</p>
		{:else if files === null}
			<div class="grid grid-cols-3 gap-3 sm:grid-cols-4">
				{#each [1, 2, 3, 4] as i (i)}
					<Skeleton class="aspect-square rounded-lg" />
				{/each}
			</div>
		{:else if files.length === 0}
			<p class="text-muted-foreground py-6 text-center text-sm">
				No {kind === 'image' ? 'photos' : 'PDFs'} in your library yet.
			</p>
		{:else}
			<ul class="grid max-h-[50svh] grid-cols-3 gap-3 overflow-y-auto p-0.5 sm:grid-cols-4">
				{#each files as file (file.id)}
					{@const inUse = selected.includes(file.id)}
					<li>
						<button
							type="button"
							onclick={() => choose(file)}
							class={cn(
								'group focus-visible:ring-ring/50 flex w-full flex-col gap-1.5 rounded-lg text-left outline-none focus-visible:ring-3'
							)}
						>
							<span class="relative block">
								<FileThumb
									id={file.id}
									kind={file.kind}
									class={cn(
										'aspect-square w-full rounded-lg ring-1 ring-black/5 transition-shadow group-hover:ring-2 group-hover:ring-foreground/40',
										inUse && 'ring-brand ring-2'
									)}
								/>
								{#if inUse}
									<span class="bg-brand text-brand-foreground absolute top-1.5 right-1.5 grid size-5 place-items-center rounded-full">
										<CheckIcon class="size-3" />
									</span>
								{/if}
							</span>
							<span class="truncate text-xs font-medium">{file.title || file.name}</span>
							<span class="text-muted-foreground -mt-1 text-[11px]">{formatBytes(file.size_bytes)}</span>
						</button>
					</li>
				{/each}
			</ul>
			{#if total > pageSize}
				<Pagination bind:page bind:pageSize {total} pageSizes={[12, 24, 48]} />
			{/if}
		{/if}

		<Dialog.Footer>
			<Dialog.Close>
				{#snippet child({ props })}
					<Button variant="outline" {...props}>Cancel</Button>
				{/snippet}
			</Dialog.Close>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
