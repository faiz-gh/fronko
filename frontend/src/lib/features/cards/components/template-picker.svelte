<script lang="ts">
	import { TEMPLATES, type TemplateKey } from '$lib/features/cards/blocks';
	import { cn } from '$lib/utils';

	let {
		value,
		accent,
		logo = false,
		onselect
	}: {
		/** The template the card's layout currently matches, if any. */
		value: TemplateKey | null;
		accent: string;
		/** Show where the organisation's logo goes. */
		logo?: boolean;
		onselect: (key: TemplateKey) => void;
	} = $props();
</script>

<!-- The organisation's logo emblem on the photo's lower right. -->
{#snippet emblem()}
	{#if logo}
		<span class="absolute -right-0.5 -bottom-0.5 size-2 rounded-full bg-white ring-1 ring-black/25"></span>
	{/if}
{/snippet}

<!-- A tiny wireframe of each template: enough to tell them apart at a glance. -->
{#snippet wireframe(key: TemplateKey)}
	<span
		class="flex h-24 flex-col overflow-hidden rounded-lg bg-white ring-1 ring-black/5 dark:bg-neutral-900"
		aria-hidden="true"
	>
		{#if key === 'event'}
			<span class="flex flex-col items-center gap-1 py-2" style="background: {accent}">
				<span class="h-0.5 w-3 rounded-full bg-black/30"></span>
				<span class="relative size-4 rounded-full bg-white/90">{@render emblem()}</span>
				<span class="h-1.5 w-12 rounded-full bg-white"></span>
			</span>
			<span class="mx-2 mt-1.5 h-2 rounded-sm" style="background: {accent}; opacity: 0.6"></span>
			<span class="mx-2 mt-1 h-2 rounded-sm bg-neutral-200 dark:bg-white/15"></span>
		{:else if key === 'minimal'}
			<span class="flex items-center gap-1.5 px-2 pt-3">
				<span class="relative size-5 shrink-0 rounded-full" style="background: {accent}">{@render emblem()}</span>
				<span class="flex flex-col gap-1">
					<span class="h-1.5 w-10 rounded-full bg-neutral-800 dark:bg-white/70"></span>
					<span class="h-1 w-7 rounded-full bg-neutral-300 dark:bg-white/30"></span>
				</span>
			</span>
			<span class="mt-2.5 flex gap-1 px-2">
				{#each [1, 2, 3] as i (i)}<span class="size-2.5 rounded-full bg-neutral-200 dark:bg-white/15"></span>{/each}
			</span>
			<span class="mx-2 mt-2 h-2 rounded-sm bg-neutral-200 dark:bg-white/15"></span>
			<span class="mx-2 mt-1 h-2 rounded-sm bg-neutral-200 dark:bg-white/15"></span>
		{:else}
			<span class="h-5 shrink-0" style="background: {accent}"></span>
			<span class="-mt-2.5 flex flex-col items-center gap-1">
				<span class="relative size-5 rounded-full bg-neutral-300 ring-2 ring-white dark:ring-neutral-900"
					>{@render emblem()}</span
				>
				<span class="h-1.5 w-10 rounded-full bg-neutral-800 dark:bg-white/70"></span>
			</span>
			{#if key === 'portfolio'}
				<span class="mx-2 mt-1.5 grid grid-cols-3 gap-0.5">
					{#each [1, 2, 3, 4, 5, 6] as i (i)}<span class="aspect-square rounded-[2px] bg-neutral-200 dark:bg-white/15"
						></span>{/each}
				</span>
			{:else}
				<span class="mt-1.5 flex justify-center gap-1">
					{#each [1, 2, 3] as i (i)}<span class="size-2.5 rounded-full bg-neutral-200 dark:bg-white/15"></span>{/each}
				</span>
				<span class="mx-2 mt-1.5 h-2 rounded-sm" style="background: {accent}"></span>
				<span class="mx-2 mt-1 h-2 rounded-sm bg-neutral-200 dark:bg-white/15"></span>
			{/if}
		{/if}
	</span>
{/snippet}

<div class="grid grid-cols-2 gap-3 2xl:grid-cols-4" role="radiogroup" aria-label="Template">
	{#each Object.entries(TEMPLATES) as [key, t] (key)}
		<button
			type="button"
			role="radio"
			aria-checked={value === key}
			onclick={() => onselect(key as TemplateKey)}
			class={cn(
				'flex flex-col gap-2 rounded-xl border p-2 text-left transition-colors',
				value === key ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
			)}
		>
			{@render wireframe(key as TemplateKey)}
			<span class="flex flex-col gap-0.5 px-1 pb-0.5">
				<span class="text-sm font-medium">{t.label}</span>
				<span class="text-muted-foreground text-xs leading-snug">{t.description}</span>
			</span>
		</button>
	{/each}
</div>
