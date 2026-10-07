<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { getFileGrants, setFileGrants } from '$lib/features/orgs/api';
	import type { LibraryFile } from '$lib/features/files/api';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Switch } from '$lib/components/ui/switch';
	import * as Tabs from '$lib/components/ui/tabs';
	import FileThumb from './file-thumb.svelte';
	import UserAvatar from '$lib/components/shared/user-avatar.svelte';
	import { teamColor } from '$lib/features/teams/api';
	import { orgUsers } from '$lib/features/orgs/users.svelte';
	import { teams } from '$lib/features/teams/store.svelte';

	/** The file whose access is being managed; the dialog is open while this is set. */
	let { file = $bindable(null) }: { file?: LibraryFile | null } = $props();

	let selected = $state<Set<number> | null>(null);
	let saved = new Set<number>();
	let selectedTeams = $state<Set<number>>(new Set());
	let savedTeams = new Set<number>();
	let tab = $state<'people' | 'teams'>('people');
	let saving = $state(false);
	let error = $state('');

	// Admins see every file anyway, and a personal file's owner always has it,
	// so only the other members can be given access.
	const candidates = $derived(
		orgUsers.assignable.filter((u) => u.role === 'member' && !(file?.area === 'personal' && file.owner?.id === u.id))
	);
	// A team file's own team already has it.
	const teamCandidates = $derived((teams.list ?? []).filter((t) => !(file?.area === 'team' && file.team?.id === t.id)));

	$effect(() => {
		const f = file;
		selected = null;
		error = '';
		if (!f) return;
		getFileGrants(f.id)
			.then(({ users, teams: grantedTeams }) => {
				if (file?.id !== f.id) return;
				saved = new Set(users.map((u) => u.id));
				savedTeams = new Set(grantedTeams.map((t) => t.id));
				selected = new Set(saved);
				selectedTeams = new Set(savedTeams);
			})
			.catch((e) => (error = e instanceof Error ? e.message : 'Failed to load access'));
	});

	function toggle(id: number, on: boolean) {
		if (!selected) return;
		const next = new Set(selected);
		if (on) next.add(id);
		else next.delete(id);
		selected = next;
	}

	function toggleTeam(id: number, on: boolean) {
		const next = new Set(selectedTeams);
		if (on) next.add(id);
		else next.delete(id);
		selectedTeams = next;
	}

	const differs = (a: Set<number>, b: Set<number>) => a.size !== b.size || [...a].some((id) => !b.has(id));
	const changed = $derived(!!selected && (differs(selected, saved) || differs(selectedTeams, savedTeams)));

	async function save() {
		if (!file || !selected) return;
		saving = true;
		try {
			await setFileGrants(file.id, { userIds: [...selected], teamIds: [...selectedTeams] });
			const parts = [
				selected.size ? `${selected.size} ${selected.size === 1 ? 'person' : 'people'}` : '',
				selectedTeams.size ? `${selectedTeams.size} ${selectedTeams.size === 1 ? 'team' : 'teams'}` : ''
			].filter(Boolean);
			toast.success(parts.length ? `Shared with ${parts.join(' and ')}` : 'Access removed');
			file = null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Could not save access';
		} finally {
			saving = false;
		}
	}
</script>

<Dialog.Root open={file !== null} onOpenChange={(open) => !open && (file = null)}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title class="text-lg">Who can use this file</Dialog.Title>
			<Dialog.Description>
				{#if file?.area === 'personal'}
					It's {file.owner?.username}'s file. Choose who else can see it and use it on their cards.
				{:else if file?.area === 'team'}
					Everyone in {file.team?.name} can use it. Choose who else can see it and use it on their cards.
				{:else}
					Organisation files are private to admins. Choose who else can see this one and use it on their cards.
				{/if}
			</Dialog.Description>
		</Dialog.Header>

		{#if file}
			<div class="flex items-center gap-3 rounded-lg border p-2.5">
				<FileThumb id={file.id} kind={file.kind} hasThumb={file.has_thumb} class="size-10 shrink-0 rounded-md" />
				<span class="min-w-0 truncate text-sm font-medium">{file.title || file.name}</span>
			</div>
		{/if}

		{#if selected === null && !error}
			<div class="flex flex-col gap-3">
				{#each [1, 2, 3] as i (i)}
					<Skeleton class="h-9" />
				{/each}
			</div>
		{:else if selected}
			<Tabs.Root bind:value={tab}>
				<Tabs.List class="w-full">
					<Tabs.Trigger value="people">People{selected.size ? ` · ${selected.size}` : ''}</Tabs.Trigger>
					<Tabs.Trigger value="teams">Teams{selectedTeams.size ? ` · ${selectedTeams.size}` : ''}</Tabs.Trigger>
				</Tabs.List>
			</Tabs.Root>
			{#if tab === 'teams'}
				{#if teamCandidates.length === 0}
					<p class="text-muted-foreground py-4 text-center text-sm">
						No teams yet. Create them on the <a href="/dashboard/teams" class="underline underline-offset-4">Teams</a> page.
					</p>
				{:else}
					<ul class="-mx-2 flex max-h-72 flex-col overflow-y-auto">
						{#each teamCandidates as team (team.id)}
							<li>
								<label class="hover:bg-muted/60 flex cursor-pointer items-center gap-3 rounded-md px-2 py-2">
									<span
										class="grid size-7 shrink-0 place-items-center rounded-full text-[11px] font-semibold text-white"
										style="background: {teamColor(team)}">{team.name.slice(0, 1).toUpperCase()}</span
									>
									<span class="flex min-w-0 flex-1 flex-col">
										<span class="truncate text-sm font-medium">{team.name}</span>
										<span class="text-muted-foreground truncate text-xs">
											{team.member_count}
											{team.member_count === 1 ? 'person' : 'people'}
										</span>
									</span>
									<Switch checked={selectedTeams.has(team.id)} onCheckedChange={(on) => toggleTeam(team.id, on)} />
								</label>
							</li>
						{/each}
					</ul>
				{/if}
			{:else if candidates.length === 0}
				<p class="text-muted-foreground py-4 text-center text-sm">No other members to share with yet.</p>
			{:else}
				<ul class="-mx-2 flex max-h-72 flex-col overflow-y-auto">
					{#each candidates as user (user.id)}
						<li>
							<label class="hover:bg-muted/60 flex cursor-pointer items-center gap-3 rounded-md px-2 py-2">
								<UserAvatar username={user.username} class="size-7 text-[10px]" />
								<span class="flex min-w-0 flex-1 flex-col">
									<span class="truncate text-sm font-medium">{user.username}</span>
									<span class="text-muted-foreground truncate text-xs">{user.email}</span>
								</span>
								<Switch checked={selected.has(user.id)} onCheckedChange={(on) => toggle(user.id, on)} />
							</label>
						</li>
					{/each}
				</ul>
			{/if}
			<p class="text-muted-foreground text-xs">Admins and the owner can always see every file.</p>
		{/if}

		{#if error}
			<p class="text-destructive text-sm">{error}</p>
		{/if}

		<Dialog.Footer>
			<Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close>
			<Button onclick={save} disabled={!changed || saving}>
				{#if saving}<Spinner data-icon="inline-start" />{/if}
				Save
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
