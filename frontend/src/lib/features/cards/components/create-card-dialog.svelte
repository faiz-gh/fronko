<script lang="ts">
	import { goto } from '$app/navigation';
	import { createProfile } from '$lib/features/cards/api';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import UserPicker from '$lib/features/orgs/components/user-picker.svelte';
	import { emptyCard, isValidSlug, slugify } from '$lib/features/cards/card';
	import { session } from '$lib/core/session.svelte';
	import { cards } from '$lib/features/cards/store.svelte';
	import { orgUsers } from '$lib/features/orgs/users.svelte';

	let name = $state('');
	let customSlug = $state('');
	let slugTouched = $state(false);
	let assignee = $state<number | null>(null);
	let creating = $state(false);
	let error = $state('');

	// The slug follows the name until the user edits it directly.
	const slug = $derived(slugTouched ? customSlug : slugify(name));
	const slugInvalid = $derived(slug.length > 0 && !isValidSlug(slug));

	function reset() {
		name = '';
		customSlug = '';
		slugTouched = false;
		assignee = null;
		error = '';
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (!isValidSlug(slug)) return;
		creating = true;
		error = '';
		try {
			const p = await createProfile(slug, emptyCard(name.trim()), assignee);
			cards.upsert(p);
			if (assignee !== null) orgUsers.refresh();
			cards.createOpen = false;
			await goto(`/dashboard/${p.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create card';
		} finally {
			creating = false;
		}
	}
</script>

<Dialog.Root bind:open={cards.createOpen} onOpenChange={(open) => !open && reset()}>
	<Dialog.Content class="sm:max-w-md">
		<form onsubmit={handleSubmit} class="flex flex-col gap-6">
			<Dialog.Header>
				<Dialog.Title class="text-lg">New card</Dialog.Title>
				<Dialog.Description>Start with a name and a link. You can change both later.</Dialog.Description>
			</Dialog.Header>
			<Field.Group>
				<Field.Field>
					<Field.Label for="new-name">Name on the card</Field.Label>
					<Input id="new-name" bind:value={name} placeholder="Jane Doe" required />
				</Field.Field>
				<Field.Field data-invalid={slugInvalid || !!error || undefined}>
					<Field.Label for="new-slug">Link</Field.Label>
					<div class="flex items-stretch">
						<span
							class="text-muted-foreground bg-muted flex max-w-[55%] items-center rounded-l-lg border border-r-0 px-3 font-mono text-xs"
						>
							<span class="truncate">/p/{session.orgHandle}/</span>
						</span>
						<Input
							id="new-slug"
							class="rounded-l-none font-mono"
							value={slug}
							oninput={(e) => {
								slugTouched = true;
								customSlug = e.currentTarget.value.toLowerCase();
							}}
							placeholder="jane-doe"
							aria-invalid={slugInvalid || !!error || undefined}
							required
						/>
					</div>
					{#if error}
						<Field.Error>{error}</Field.Error>
					{:else if slugInvalid}
						<Field.Error>3–48 characters: lowercase letters, numbers and hyphens.</Field.Error>
					{:else}
						<Field.Description>Only needs to be unique in {session.orgName || 'your organisation'}. This is the address you'll share and write to NFC cards.</Field.Description>
					{/if}
				</Field.Field>
				<Field.Field>
					<Field.Label>Assign to</Field.Label>
					<UserPicker
						value={assignee}
						onchange={(v) => (assignee = typeof v === 'number' ? v : null)}
						filter={false}
						noneLabel="Nobody yet"
						class="max-w-none"
					/>
					<Field.Description>They can edit the card and see the leads it collects while it's theirs.</Field.Description>
				</Field.Field>
			</Field.Group>
			<Dialog.Footer>
				<Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close>
				<Button type="submit" disabled={creating || !isValidSlug(slug)}>
					{#if creating}
						<Spinner data-icon="inline-start" />
					{/if}
					Create card
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
