<script lang="ts">
	import BuildingIcon from '@lucide/svelte/icons/building-2';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import UsersIcon from '@lucide/svelte/icons/users';
	import { buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import UserAvatar from './user-avatar.svelte';
	import { orgUsers } from '$lib/org-users.svelte';
	import { cn } from '$lib/utils';

	/** A user id, 'none' for the organisation itself, or null for everyone (filters only). */
	type Value = number | 'none' | null;

	let {
		value,
		onchange,
		allLabel = 'All users',
		noneLabel = 'Unassigned',
		filter = true,
		allowNone = true,
		disabled = false,
		size = 'default',
		class: className
	}: {
		value: Value;
		onchange: (value: Value) => void;
		allLabel?: string;
		noneLabel?: string;
		/** Filters offer "all"; assignment pickers only a user or the organisation. */
		filter?: boolean;
		/** Offer the organisation itself ("none"). */
		allowNone?: boolean;
		disabled?: boolean;
		size?: 'default' | 'sm';
		class?: string;
	} = $props();

	const selected = $derived(typeof value === 'number' ? orgUsers.byId(value) : undefined);
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		{disabled}
		class={buttonVariants({ variant: 'outline', size, class: cn('max-w-64 justify-start gap-2', className) })}
	>
		{#if selected}
			<UserAvatar username={selected.username} class="size-5 text-[9px]" />
			<span class="truncate">{selected.username}</span>
		{:else if value === 'none' || (!filter && value === null)}
			<BuildingIcon class="text-muted-foreground" />
			<span class="truncate">{noneLabel}</span>
		{:else}
			<UsersIcon class="text-muted-foreground" />
			<span class="truncate">{allLabel}</span>
		{/if}
		<ChevronDownIcon class="text-muted-foreground ml-auto size-3.5" />
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="start" class="max-h-80 w-64">
		<DropdownMenu.RadioGroup
			value={value === null ? (filter ? 'all' : 'none') : String(value)}
			onValueChange={(v) => onchange(v === 'all' ? null : v === 'none' ? 'none' : Number(v))}
		>
			{#if filter}
				<DropdownMenu.RadioItem value="all">{allLabel}</DropdownMenu.RadioItem>
			{/if}
			{#if allowNone}
				<DropdownMenu.RadioItem value="none">
					<span class="flex items-center gap-2">
						<BuildingIcon class="text-muted-foreground size-4" />
						{noneLabel}
					</span>
				</DropdownMenu.RadioItem>
			{/if}
			{#if orgUsers.assignable.length > 0}
				<DropdownMenu.Separator />
			{/if}
			{#each orgUsers.assignable as user (user.id)}
				<DropdownMenu.RadioItem value={String(user.id)}>
					<span class="flex min-w-0 flex-1 items-center gap-2">
						<UserAvatar username={user.username} class="size-5 text-[9px]" />
						<span class="truncate">{user.username}</span>
						{#if user.suspended_at}
							<span class="text-muted-foreground ml-auto text-xs">Suspended</span>
						{/if}
					</span>
				</DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>
