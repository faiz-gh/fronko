<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import {
		createTeam,
		TEAM_COLOR_NAMES,
		TEAM_COLORS,
		updateTeam,
		type Team,
		type TeamDetail
	} from '$lib/features/teams/api';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';
	import { teams } from '$lib/features/teams/store.svelte';

	/** Creates a team, or edits `team` when it's given. */
	let {
		open = $bindable(false),
		team = null,
		onsaved
	}: { open?: boolean; team?: Team | null; onsaved?: (team: TeamDetail) => void } = $props();

	let name = $state('');
	let description = $state('');
	let color = $state(TEAM_COLORS[0]);
	let saving = $state(false);
	let error = $state('');

	$effect(() => {
		if (!open) return;
		name = team?.name ?? '';
		description = team?.description ?? '';
		// New teams get the next colour nobody uses yet.
		const used = new Set((teams.list ?? []).map((t) => t.color));
		color = team?.color || TEAM_COLORS.find((c) => !used.has(c)) || TEAM_COLORS[0];
		error = '';
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		error = '';
		try {
			const body = { name: name.trim(), description: description.trim(), color };
			const saved = team ? await updateTeam(team.id, body) : await createTeam(body);
			teams.upsert(saved);
			teams.details[saved.id] = saved;
			onsaved?.(saved);
			open = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Could not save the team';
		} finally {
			saving = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<form onsubmit={submit} class="flex flex-col gap-6">
			<Dialog.Header>
				<Dialog.Title class="text-lg">{team ? 'Edit team' : 'New team'}</Dialog.Title>
				<Dialog.Description>
					{team
						? 'Change how the team is named and shown.'
						: 'Group people like Sales or Finance. Teams get their own files, and team leads see their teammates’ cards and leads.'}
				</Dialog.Description>
			</Dialog.Header>
			<Field.Group>
				<Field.Field>
					<Field.Label for="team-name">Name</Field.Label>
					<Input id="team-name" bind:value={name} maxlength={60} required placeholder="Sales" autocomplete="off" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="team-description">Description</Field.Label>
					<Textarea
						id="team-description"
						bind:value={description}
						maxlength={280}
						rows={2}
						placeholder="Optional: what the team does"
					/>
				</Field.Field>
				<Field.Field>
					<Field.Label id="team-colour-label">Colour</Field.Label>
					<div class="flex flex-wrap gap-2" role="radiogroup" aria-labelledby="team-colour-label">
						{#each TEAM_COLORS as c (c)}
							<button
								type="button"
								role="radio"
								aria-checked={color === c}
								aria-label={TEAM_COLOR_NAMES[c]}
								onclick={() => (color = c)}
								class="focus-visible:ring-ring/50 grid size-8 place-items-center rounded-full text-white ring-offset-2 ring-offset-background outline-none focus-visible:ring-3 aria-checked:ring-2 aria-checked:ring-foreground/60"
								style="background: {c}"
							>
								{#if color === c}<CheckIcon class="size-4" />{/if}
							</button>
						{/each}
					</div>
				</Field.Field>
				{#if error}<Field.Error>{error}</Field.Error>{/if}
			</Field.Group>
			<Dialog.Footer>
				<Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close>
				<Button type="submit" disabled={saving || !name.trim()}>
					{#if saving}<Spinner data-icon="inline-start" />{/if}
					{team ? 'Save' : 'Create team'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
