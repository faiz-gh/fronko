<script lang="ts">
	import ArrowUpRightIcon from '@lucide/svelte/icons/arrow-up-right';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import { fileUrl, formatBytes, type PublicFile } from '$lib/api/files';
	import type { CardData } from '$lib/card/card';

	let { card, files }: { card: CardData; files?: Record<string, PublicFile> } = $props();

	const documents = $derived(
		card.documents.flatMap((d) => {
			const meta = files?.[d.file];
			if (files && !meta) return [];
			const title = d.title.trim() || meta?.name.replace(/\.pdf$/i, '') || 'Brochure';
			return [{ ...d, href: fileUrl(d.file), title, size: meta ? formatBytes(meta.size_bytes) : null }];
		})
	);
</script>

{#if documents.length > 0}
	<div class="flex flex-col gap-3">
		<p class="text-muted-foreground text-xs font-medium tracking-wide uppercase">Brochures</p>
		<ul class="flex flex-col gap-2">
			{#each documents as doc (doc.id)}
				<li>
					<a
						href={doc.href}
						target="_blank"
						rel="noopener noreferrer"
						class="bg-muted/60 hover:bg-muted group flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-medium transition-colors"
					>
						<span class="text-(--card-accent)"><FileTextIcon class="size-[18px]" /></span>
						<span class="flex min-w-0 flex-1 flex-col">
							<span class="truncate">{doc.title}</span>
							<span class="text-muted-foreground text-xs font-normal">PDF{doc.size ? ` · ${doc.size}` : ''}</span>
						</span>
						<ArrowUpRightIcon
							class="text-muted-foreground size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5"
						/>
					</a>
				</li>
			{/each}
		</ul>
	</div>
{/if}
