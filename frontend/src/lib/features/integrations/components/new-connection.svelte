<script lang="ts">
	import { untrack } from 'svelte';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import { ApiError } from '$lib/core/api';
	import { cn } from '$lib/utils';
	import { createConnection } from '../api';
	import { fieldProblems, initialValues, settingsChange, type FormValues } from '../fields';
	import { SCOPE_HELP } from '../registry';
	import type { CatalogEntry, Connection, Scope } from '../types';
	import ConnectionForm from './connection-form.svelte';

	let {
		entry,
		scopes,
		oncreated,
		oncancel
	}: {
		entry: CatalogEntry;
		/** The scopes this user may create, in the manifest's order. */
		scopes: Scope[];
		oncreated: (c: Connection) => void;
		oncancel?: () => void;
	} = $props();

	// The form starts from the props it was opened with; the page remounts it for another provider.
	let scope = $state<Scope>(untrack(() => scopes[0]));
	let name = $state('');
	let values = $state<FormValues>(untrack(() => initialValues(entry.fields)));
	let problems = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);

	const SCOPE_CHOICE: Record<Scope, string> = { org: 'For the organisation', user: 'Just for me' };

	async function create(event: SubmitEvent) {
		event.preventDefault();
		formError = '';
		problems = fieldProblems(entry.fields, values);
		if (Object.keys(problems).length > 0) return;
		saving = true;
		try {
			const created = await createConnection({
				provider: entry.id,
				scope,
				name: name.trim(),
				...settingsChange(entry.fields, values)
			});
			oncreated(created);
		} catch (e) {
			const field = e instanceof ApiError ? e.field : undefined;
			const msg = e instanceof Error ? e.message : 'Failed to save';
			if (field && entry.fields.some((f) => f.key === field)) problems = { [field]: msg };
			else formError = msg;
		} finally {
			saving = false;
		}
	}
</script>

<form onsubmit={create} novalidate class="bg-card flex flex-col gap-5 rounded-xl border p-4 sm:p-5">
	<div class="flex flex-col gap-1">
		<h3 class="font-semibold">New {entry.name} connection</h3>
		<p class="text-muted-foreground text-sm">Secrets are encrypted before they're stored and never shown again.</p>
	</div>

	{#if scopes.length > 1}
		<Field.Field>
			<Field.Label>Who it's for</Field.Label>
			<div class="grid gap-2 sm:grid-cols-2" role="radiogroup" aria-label="Who it's for">
				{#each scopes as s (s)}
					<button
						type="button"
						role="radio"
						aria-checked={scope === s}
						onclick={() => (scope = s)}
						class={cn(
							'flex flex-col gap-0.5 rounded-lg border px-3 py-2.5 text-left text-sm transition-colors',
							scope === s ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
						)}
					>
						<span class="font-medium">{SCOPE_CHOICE[s]}</span>
						<span class="text-muted-foreground text-xs">{SCOPE_HELP[entry.category][s]}</span>
					</button>
				{/each}
			</div>
		</Field.Field>
	{:else if SCOPE_HELP[entry.category][scope]}
		<p class="text-muted-foreground -mt-2 text-sm">{SCOPE_HELP[entry.category][scope]}</p>
	{/if}

	<Field.Field>
		<Field.Label for="new-name">Name <span class="text-muted-foreground font-normal">(optional)</span></Field.Label>
		<Input id="new-name" bind:value={name} placeholder={entry.name} maxlength={80} autocomplete="off" />
		<Field.Description>Only shown here, to tell connections apart.</Field.Description>
	</Field.Field>

	<ConnectionForm fields={entry.fields} bind:values {problems} idPrefix="new" disabled={saving} />

	{#if formError}
		<Alert.Root variant="destructive">
			<CircleAlertIcon />
			<Alert.Title>{formError}</Alert.Title>
		</Alert.Root>
	{/if}

	<div class="flex flex-wrap gap-2">
		<Button type="submit" disabled={saving}>
			{#if saving}<Spinner data-icon="inline-start" />{/if}
			{entry.auth === 'oauth2' ? 'Save and continue' : 'Connect'}
		</Button>
		{#if oncancel}
			<Button type="button" variant="ghost" onclick={oncancel} disabled={saving}>Cancel</Button>
		{/if}
	</div>
</form>
