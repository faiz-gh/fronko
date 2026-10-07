<script lang="ts">
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { cn } from '$lib/utils';

	let {
		page = $bindable(1),
		pageSize = $bindable(25),
		total,
		pageSizes = [10, 25, 50, 100],
		disabled = false
	}: {
		/** 1-based. */
		page?: number;
		pageSize?: number;
		total: number;
		pageSizes?: number[];
		disabled?: boolean;
	} = $props();

	const pageCount = $derived(Math.max(1, Math.ceil(total / pageSize)));
	const first = $derived(total === 0 ? 0 : (page - 1) * pageSize + 1);
	const last = $derived(Math.min(page * pageSize, total));

	/** Page numbers to show, with null for a gap: 1 … 4 5 6 … 12 */
	const pages = $derived.by(() => {
		const out: (number | null)[] = [];
		for (let p = 1; p <= pageCount; p++) {
			if (p === 1 || p === pageCount || Math.abs(p - page) <= 1) out.push(p);
			else if (out.at(-1) !== null) out.push(null);
		}
		return out;
	});

	function setPageSize(size: number) {
		// Keep the first visible row on screen when the page size changes.
		const firstIndex = (page - 1) * pageSize;
		pageSize = size;
		page = Math.floor(firstIndex / size) + 1;
	}
</script>

<nav class="flex flex-wrap items-center justify-between gap-3 text-sm" aria-label="Pagination">
	<div class="text-muted-foreground flex items-center gap-3">
		<span class="tabular" aria-live="polite">
			{#if total === 0}
				No results
			{:else}
				<span class="text-foreground font-medium">{first}–{last}</span> of
				<span class="text-foreground font-medium">{total}</span>
			{/if}
		</span>
		<DropdownMenu.Root>
			<DropdownMenu.Trigger
				class={buttonVariants({ variant: 'ghost', size: 'sm', class: 'text-muted-foreground gap-1 font-normal' })}
				{disabled}
			>
				{pageSize} per page
				<ChevronDownIcon class="size-3.5" />
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start" class="w-36">
				<DropdownMenu.RadioGroup value={String(pageSize)} onValueChange={(v) => setPageSize(Number(v))}>
					{#each pageSizes as size (size)}
						<DropdownMenu.RadioItem value={String(size)}>{size} per page</DropdownMenu.RadioItem>
					{/each}
				</DropdownMenu.RadioGroup>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>

	<div class="flex items-center gap-1">
		<Button
			variant="outline"
			size="sm"
			onclick={() => (page = page - 1)}
			disabled={disabled || page <= 1}
			aria-label="Previous page"
		>
			<ChevronLeftIcon />
			<span class="max-sm:sr-only">Previous</span>
		</Button>
		<div class="hidden items-center gap-1 sm:flex">
			{#each pages as p, i (p ?? `gap-${i}`)}
				{#if p === null}
					<span class="text-muted-foreground w-6 text-center" aria-hidden="true">…</span>
				{:else}
					<Button
						variant={p === page ? 'secondary' : 'ghost'}
						size="sm"
						class={cn('tabular min-w-8 px-2', p === page && 'font-semibold')}
						onclick={() => (page = p)}
						{disabled}
						aria-label="Page {p}"
						aria-current={p === page ? 'page' : undefined}
					>
						{p}
					</Button>
				{/if}
			{/each}
		</div>
		<span class="text-muted-foreground tabular px-2 sm:hidden">{page} / {pageCount}</span>
		<Button
			variant="outline"
			size="sm"
			onclick={() => (page = page + 1)}
			disabled={disabled || page >= pageCount}
			aria-label="Next page"
		>
			<span class="max-sm:sr-only">Next</span>
			<ChevronRightIcon />
		</Button>
	</div>
</nav>
