<script lang="ts">
	import LinkIcon from '@lucide/svelte/icons/link';
	import { fileDimensions, fileTitle, formatBytes, locationLabel, PURPOSES, type LibraryFile } from '$lib/features/files/api';
	import { teamColor } from '$lib/features/teams/api';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Table from '$lib/components/ui/table';
	import FileThumb from './file-thumb.svelte';
	import PurposeIcon from './purpose-icon.svelte';
	import { timeAgo } from '$lib/core/format';
	import { cn } from '$lib/utils';

	let {
		files,
		selected,
		selectable = true,
		onopen,
		ontoggle,
		ontoggleall
	}: {
		files: LibraryFile[];
		selected: Set<string>;
		selectable?: boolean;
		onopen: (file: LibraryFile) => void;
		ontoggle: (file: LibraryFile, range: boolean) => void;
		ontoggleall: (on: boolean) => void;
	} = $props();

	const allSelected = $derived(files.length > 0 && files.every((f) => selected.has(f.id)));
	const someSelected = $derived(!allSelected && files.some((f) => selected.has(f.id)));
	// Shift is read from the click, which fires before the checkbox changes.
	let shift = false;
</script>

<div class="bg-card overflow-hidden rounded-xl border">
	<Table.Root>
		<Table.Header>
			<Table.Row class="hover:bg-transparent">
				{#if selectable}
					<Table.Head class="w-10 pl-4">
						<Checkbox
							checked={allSelected}
							indeterminate={someSelected}
							onCheckedChange={(v) => ontoggleall(v === true)}
							aria-label="Select all on this page"
						/>
					</Table.Head>
				{/if}
				<Table.Head class={cn(!selectable && 'pl-4')}>Name</Table.Head>
				<Table.Head class="hidden md:table-cell">Purpose</Table.Head>
				<Table.Head class="hidden lg:table-cell">Location</Table.Head>
				<Table.Head class="hidden xl:table-cell">Size</Table.Head>
				<Table.Head class="hidden sm:table-cell">Used</Table.Head>
				<Table.Head class="hidden lg:table-cell">Uploaded</Table.Head>
			</Table.Row>
		</Table.Header>
		<Table.Body>
			{#each files as file (file.id)}
				{@const isSelected = selected.has(file.id)}
				{@const dims = fileDimensions(file)}
				<Table.Row data-state={isSelected ? 'selected' : undefined} class="cursor-pointer" onclick={() => onopen(file)}>
					{#if selectable}
						<Table.Cell class="pl-4" onclick={(e: MouseEvent) => e.stopPropagation()}>
							<Checkbox
								checked={isSelected}
								onclick={(e: MouseEvent) => (shift = e.shiftKey)}
								onCheckedChange={() => ontoggle(file, shift)}
								aria-label="Select {fileTitle(file)}"
							/>
						</Table.Cell>
					{/if}
					<Table.Cell class={cn('max-w-0 min-w-48', !selectable && 'pl-4')}>
						<span class="flex items-center gap-3">
							<FileThumb
								id={file.id}
								kind={file.kind}
								hasThumb={file.has_thumb}
								fit={file.purpose === 'logo' || file.purpose === 'banner' ? 'contain' : 'cover'}
								class="size-10 shrink-0 rounded-md"
							/>
							<span class="flex min-w-0 flex-col">
								<button
									type="button"
									class="focus-visible:ring-ring/50 truncate rounded-sm text-left font-medium outline-none focus-visible:ring-3"
									onclick={(e) => {
										e.stopPropagation();
										onopen(file);
									}}
								>
									{fileTitle(file)}
								</button>
								<span class="text-muted-foreground truncate text-xs">
									{file.kind === 'pdf' ? 'PDF' : file.content_type.replace('image/', '').toUpperCase()}{dims ? ` · ${dims}` : ''}
									<span class="md:hidden"> · {PURPOSES[file.purpose].label}</span>
								</span>
							</span>
						</span>
					</Table.Cell>
					<Table.Cell class="hidden md:table-cell">
						<span class="inline-flex items-center gap-1.5">
							<PurposeIcon purpose={file.purpose} class="text-muted-foreground size-3.5" />
							{PURPOSES[file.purpose].label}
						</span>
					</Table.Cell>
					<Table.Cell class="hidden lg:table-cell">
						<span class="inline-flex items-center gap-1.5">
							{#if file.team}
								<span class="size-2 rounded-full" style="background: {teamColor(file.team)}"></span>
							{/if}
							{locationLabel(file)}
						</span>
					</Table.Cell>
					<Table.Cell class="text-muted-foreground tabular hidden xl:table-cell">{formatBytes(file.size_bytes)}</Table.Cell>
					<Table.Cell class="hidden sm:table-cell">
						{#if file.use_count > 0}
							<span class="inline-flex items-center gap-1">
								<LinkIcon class="text-brand size-3.5" />
								{file.use_count}
							</span>
						{:else}
							<span class="text-muted-foreground">—</span>
						{/if}
					</Table.Cell>
					<Table.Cell class="text-muted-foreground hidden lg:table-cell">
						<span class="block truncate">{timeAgo(file.created_at)}{file.owner ? ` · ${file.owner.username}` : ''}</span>
					</Table.Cell>
				</Table.Row>
			{/each}
		</Table.Body>
	</Table.Root>
</div>
