<script lang="ts">
	import { teamColor, TEAM_ROLE_LABEL, type TeamRole } from '$lib/api/teams';
	import { Switch } from '$lib/components/ui/switch';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import { teams } from '$lib/teams.svelte';
	import { cn } from '$lib/utils';

	/** Which teams someone is in and their role in each, chosen from the organisation's teams. */
	let {
		value = $bindable([]),
		disabled = false,
		class: className
	}: {
		value?: { team_id: number; role: TeamRole }[];
		disabled?: boolean;
		class?: string;
	} = $props();

	const roleOf = (id: number) => value.find((m) => m.team_id === id)?.role ?? null;

	function toggle(id: number, on: boolean) {
		value = on ? [...value, { team_id: id, role: 'member' }] : value.filter((m) => m.team_id !== id);
	}

	function setRole(id: number, role: TeamRole) {
		value = value.map((m) => (m.team_id === id ? { ...m, role } : m));
	}
</script>

{#if !teams.list?.length}
	<p class={cn('text-muted-foreground text-sm', className)}>
		No teams yet. Create them on the <a href="/dashboard/teams" class="underline underline-offset-4">Teams</a> page.
	</p>
{:else}
	<ul class={cn('flex flex-col divide-y rounded-lg border', className)}>
		{#each teams.list as team (team.id)}
			{@const role = roleOf(team.id)}
			<li class="flex min-h-12 items-center gap-3 px-3 py-2">
				<Switch
					id="team-{team.id}"
					checked={role !== null}
					onCheckedChange={(on) => toggle(team.id, on)}
					{disabled}
					aria-label="In {team.name}"
				/>
				<label for="team-{team.id}" class="flex min-w-0 flex-1 cursor-pointer items-center gap-2 text-sm font-medium">
					<span class="size-2.5 shrink-0 rounded-full" style="background: {teamColor(team)}"></span>
					<span class="truncate">{team.name}</span>
				</label>
				{#if role}
					<ToggleGroup.Root
						type="single"
						size="sm"
						variant="outline"
						value={role}
						onValueChange={(v) => v && setRole(team.id, v as TeamRole)}
						{disabled}
						aria-label="Role in {team.name}"
					>
						{#each ['member', 'lead'] as const as r (r)}
							<ToggleGroup.Item value={r} class="px-2.5 text-xs">{TEAM_ROLE_LABEL[r]}</ToggleGroup.Item>
						{/each}
					</ToggleGroup.Root>
				{/if}
			</li>
		{/each}
	</ul>
{/if}
