<script lang="ts">
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import ImageOffIcon from '@lucide/svelte/icons/image-off';
	import { fileUrl, type FileKind } from '$lib/api/files';
	import { cn } from '$lib/utils';

	let { id, kind, class: className }: { id: string; kind: FileKind; class?: string } = $props();

	let failed = $state(false);
</script>

<div class={cn('bg-muted text-muted-foreground relative grid place-items-center overflow-hidden', className)}>
	{#if kind === 'image' && !failed}
		<img
			src={fileUrl(id)}
			alt=""
			loading="lazy"
			class="absolute inset-0 size-full object-cover"
			onerror={() => (failed = true)}
		/>
	{:else if kind === 'image'}
		<ImageOffIcon class="size-6" aria-label="Image unavailable" />
	{:else}
		<span class="flex flex-col items-center gap-1">
			<FileTextIcon class="size-7" />
			<span class="text-[10px] font-semibold tracking-wider">PDF</span>
		</span>
	{/if}
</div>
