<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils';

	let {
		title,
		description,
		id,
		panel = false,
		class: className,
		children
	}: {
		title: string;
		description?: string;
		id?: string;
		/** Shown on its own as the tab panel of a SectionRail item with the same id. */
		panel?: boolean;
		class?: string;
		children: Snippet;
	} = $props();
</script>

{#if panel}
	<div
		id="panel-{id}"
		role="tabpanel"
		tabindex="-1"
		aria-labelledby="rail-{id}"
		class={cn('flex flex-col gap-6 pb-8 outline-none', className)}
	>
		<div class="flex flex-col gap-1">
			<h2 class="text-lg font-semibold tracking-tight">{title}</h2>
			{#if description}
				<p class="text-muted-foreground text-sm text-pretty">{description}</p>
			{/if}
		</div>
		<div class="min-w-0 max-w-3xl">
			{@render children()}
		</div>
	</div>
{:else}
	<!-- Heading beside the fields on wide screens, above them otherwise. -->
	<section
		{id}
		aria-labelledby={id ? `${id}-title` : undefined}
		class={cn('grid gap-x-12 gap-y-5 border-b py-8 last:border-b-0 2xl:grid-cols-[220px_minmax(0,1fr)]', className)}
	>
		<div class="flex flex-col gap-1">
			<h2 id={id ? `${id}-title` : undefined} class="text-base font-semibold tracking-tight">{title}</h2>
			{#if description}
				<p class="text-muted-foreground text-sm text-pretty">{description}</p>
			{/if}
		</div>
		<div class="min-w-0 max-w-3xl">
			{@render children()}
		</div>
	</section>
{/if}
