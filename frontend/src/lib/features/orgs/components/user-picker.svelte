<script lang="ts">
	import { Command as CommandPrimitive } from 'bits-ui';
	import BuildingIcon from '@lucide/svelte/icons/building-2';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import UsersIcon from '@lucide/svelte/icons/users';
	import type { OrgUser } from '$lib/features/orgs/api';
	import { teamColor } from '$lib/features/teams/api';
	import { buttonVariants } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import * as Popover from '$lib/components/ui/popover';
	import UserAvatar from '$lib/components/shared/user-avatar.svelte';
	import { orgUsers } from '$lib/features/orgs/users.svelte';
	import { teams } from '$lib/features/teams/store.svelte';
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

	let open = $state(false);
	const selected = $derived(typeof value === 'number' ? orgUsers.byId(value) : undefined);

	// People grouped by team. Someone in several teams shows under each; people
	// in no team come last. Without teams it's one plain list.
	const groups = $derived.by(() => {
		const people = orgUsers.assignable;
		const list = teams.list ?? [];
		if (list.length === 0)
			return [{ key: 'all', label: '', color: '', people: people.map((u) => ({ user: u, lead: false })) }];
		const out = list
			.map((t) => ({
				key: `team-${t.id}`,
				label: t.name,
				color: teamColor(t),
				people: people.flatMap((u) => {
					const m = u.teams.find((x) => x.id === t.id);
					return m ? [{ user: u, lead: m.role === 'lead' }] : [];
				})
			}))
			.filter((g) => g.people.length > 0);
		const loose = people.filter((u) => u.teams.length === 0);
		if (loose.length)
			out.push({ key: 'none', label: 'No team', color: '', people: loose.map((u) => ({ user: u, lead: false })) });
		return out;
	});

	function pick(v: Value) {
		open = false;
		onchange(v);
	}
</script>

{#snippet person(user: OrgUser, lead: boolean, group: string)}
	<Command.Item
		value="{user.username} {user.email ?? ''} {group}"
		data-checked={value === user.id}
		onSelect={() => pick(user.id)}
	>
		<UserAvatar username={user.username} class="size-5 text-[9px]" />
		<span class="min-w-0 flex-1 truncate">{user.username}</span>
		{#if lead}
			<span class="text-muted-foreground text-[11px]">Lead</span>
		{/if}
		{#if user.suspended_at}
			<span class="text-muted-foreground text-[11px]">Suspended</span>
		{/if}
	</Command.Item>
{/snippet}

<Popover.Root bind:open>
	<Popover.Trigger
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
	</Popover.Trigger>
	<Popover.Content align="start" class="w-72 p-0">
		<Command.Root>
			<Command.Input placeholder="Search people…" />
			<Command.List class="max-h-80">
				<Command.Empty>Nobody found.</Command.Empty>
				{#if filter || allowNone}
					<Command.Group>
						{#if filter}
							<Command.Item value="__all {allLabel}" data-checked={value === null} onSelect={() => pick(null)}>
								<UsersIcon class="text-muted-foreground" />
								{allLabel}
							</Command.Item>
						{/if}
						{#if allowNone}
							<Command.Item
								value="__none {noneLabel}"
								data-checked={value === 'none' || (!filter && value === null)}
								onSelect={() => pick('none')}
							>
								<BuildingIcon class="text-muted-foreground" />
								{noneLabel}
							</Command.Item>
						{/if}
					</Command.Group>
					<Command.Separator />
				{/if}
				{#each groups as g (g.key)}
					<CommandPrimitive.Group value={g.key} class="overflow-hidden p-1">
						{#if g.label}
							<CommandPrimitive.GroupHeading
								class="text-muted-foreground flex items-center gap-1.5 px-2 py-1.5 text-xs font-medium"
							>
								{#if g.color}<span class="size-2 rounded-full" style="background: {g.color}"></span>{/if}
								{g.label}
							</CommandPrimitive.GroupHeading>
						{/if}
						<CommandPrimitive.GroupItems>
							{#each g.people as p (p.user.id)}
								{@render person(p.user, p.lead, g.label)}
							{/each}
						</CommandPrimitive.GroupItems>
					</CommandPrimitive.Group>
				{/each}
			</Command.List>
		</Command.Root>
	</Popover.Content>
</Popover.Root>
