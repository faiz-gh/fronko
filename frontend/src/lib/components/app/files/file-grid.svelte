<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import LinkIcon from '@lucide/svelte/icons/link';
	import { fileDimensions, fileTitle, formatBytes, locationLabel, PURPOSES, type LibraryFile } from '$lib/api/files';
	import { teamColor } from '$lib/api/teams';
	import FileThumb from '../file-thumb.svelte';
	import { cn } from '$lib/utils';

	let {
		files,
		selected,
		selectable = true,
		showLocation = false,
		onopen,
		ontoggle
	}: {
		files: LibraryFile[];
		/** Ids of selected files. */
		selected: Set<string>;
		selectable?: boolean;
		/** Say where each file lives (for views that mix locations). */
		showLocation?: boolean;
		onopen: (file: LibraryFile) => void;
		/** range: shift was held, to select everything since the last click. */
		ontoggle: (file: LibraryFile, range: boolean) => void;
	} = $props();

	const anySelected = $derived(selected.size > 0);
</script>

<ul class="grid grid-cols-2 gap-3 sm:grid-cols-[repeat(auto-fill,minmax(190px,1fr))] sm:gap-4">
	{#each files as file (file.id)}
		{@const isSelected = selected.has(file.id)}
		{@const dims = fileDimensions(file)}
		<li
			class={cn(
				'group bg-card relative flex flex-col overflow-hidden rounded-xl border transition-shadow',
				'hover:shadow-md focus-within:shadow-md',
				isSelected && 'ring-brand border-brand ring-2'
			)}
		>
			<button
				type="button"
				class="focus-visible:ring-ring/50 block text-left outline-none focus-visible:ring-3 focus-visible:ring-inset"
				onclick={(e) => (anySelected && selectable ? ontoggle(file, e.shiftKey) : onopen(file))}
				aria-label="Open {fileTitle(file)}"
			>
				<span class="relative block">
					<FileThumb
						id={file.id}
						kind={file.kind}
						hasThumb={file.has_thumb}
						fit={file.purpose === 'logo' || file.purpose === 'banner' ? 'contain' : 'cover'}
						class="aspect-[4/3] w-full"
					/>
					<span class="pointer-events-none absolute top-2 left-2 flex max-w-[calc(100%-3rem)] flex-wrap gap-1">
						<span class="bg-background/90 text-foreground rounded-full px-2 py-0.5 text-[11px] font-medium shadow-sm backdrop-blur">
							{PURPOSES[file.purpose].label}
						</span>
						{#if file.team && showLocation}
							<span
								class="bg-background/90 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium shadow-sm backdrop-blur"
							>
								<span class="size-1.5 rounded-full" style="background: {teamColor(file.team)}"></span>
								{file.team.name}
							</span>
						{/if}
					</span>
					{#if file.use_count > 0}
						<span
							class="bg-brand text-brand-foreground pointer-events-none absolute right-2 bottom-2 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium shadow-sm"
							title="Used in {file.use_count} {file.use_count === 1 ? 'place' : 'places'}"
						>
							<LinkIcon class="size-3" />
							{file.use_count}
						</span>
					{/if}
				</span>
				<span class="flex flex-col gap-0.5 px-3 py-2.5">
					<span class="truncate text-sm font-medium" title={fileTitle(file)}>{fileTitle(file)}</span>
					<span class="text-muted-foreground truncate text-xs">
						{formatBytes(file.size_bytes)}{dims ? ` · ${dims}` : ''}{showLocation ? ` · ${locationLabel(file)}` : ''}
					</span>
				</span>
			</button>

			{#if selectable}
				<button
					type="button"
					role="checkbox"
					aria-checked={isSelected}
					aria-label="Select {fileTitle(file)}"
					onclick={(e) => ontoggle(file, e.shiftKey)}
					class={cn(
						'absolute top-2 right-2 grid size-6 place-items-center rounded-md border shadow-sm transition-opacity',
						'focus-visible:ring-ring/50 outline-none focus-visible:opacity-100 focus-visible:ring-3',
						isSelected
							? 'bg-brand border-brand text-brand-foreground opacity-100'
							: 'bg-background/90 opacity-0 backdrop-blur group-hover:opacity-100',
						anySelected && 'opacity-100'
					)}
				>
					{#if isSelected}<CheckIcon class="size-3.5" />{/if}
				</button>
			{/if}
		</li>
	{/each}
</ul>
