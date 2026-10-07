<script lang="ts">
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import { teamColor, type TeamRef } from '$lib/api/teams';
	import { buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { cn } from '$lib/utils';

	/** Filters a list by team; null is every team. */
	let {
		value,
		options,
		onchange,
		allLabel = 'All teams',
		size = 'default',
		class: className
	}: {
		value: number | null;
		options: TeamRef[];
		onchange: (value: number | null) => void;
		allLabel?: string;
		size?: 'default' | 'sm';
		class?: string;
	} = $props();

	const selected = $derived(options.find((t) => t.id === value));
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger class={buttonVariants({ variant: 'outline', size, class: cn('max-w-56 justify-start gap-2', className) })}>
		{#if selected}
			<span class="size-2.5 shrink-0 rounded-full" style="background: {teamColor(selected)}"></span>
			<span class="truncate">{selected.name}</span>
		{:else}
			<UsersRoundIcon class="text-muted-foreground" />
			<span class="truncate">{allLabel}</span>
		{/if}
		<ChevronDownIcon class="text-muted-foreground ml-auto size-3.5" />
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="start" class="max-h-80 w-56">
		<DropdownMenu.RadioGroup
			value={value === null ? 'all' : String(value)}
			onValueChange={(v) => onchange(v === 'all' ? null : Number(v))}
		>
			<DropdownMenu.RadioItem value="all">{allLabel}</DropdownMenu.RadioItem>
			{#if options.length > 0}
				<DropdownMenu.Separator />
			{/if}
			{#each options as team (team.id)}
				<DropdownMenu.RadioItem value={String(team.id)}>
					<span class="flex min-w-0 items-center gap-2">
						<span class="size-2.5 shrink-0 rounded-full" style="background: {teamColor(team)}"></span>
						<span class="truncate">{team.name}</span>
					</span>
				</DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>
