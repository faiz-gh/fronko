<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { getFileGrants, setFileGrants } from '$lib/api/org';
	import type { LibraryFile } from '$lib/api/files';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Switch } from '$lib/components/ui/switch';
	import FileThumb from './file-thumb.svelte';
	import UserAvatar from './user-avatar.svelte';
	import { orgUsers } from '$lib/org-users.svelte';

	/** The file whose access is being managed; the dialog is open while this is set. */
	let { file = $bindable(null) }: { file?: LibraryFile | null } = $props();

	let selected = $state<Set<number> | null>(null);
	let saved = new Set<number>();
	let saving = $state(false);
	let error = $state('');

	// Admins see every file anyway, and a personal file's owner always has it,
	// so only the other members can be given access.
	const candidates = $derived(
		orgUsers.assignable.filter((u) => u.role === 'member' && !(file?.area === 'personal' && file.owner?.id === u.id))
	);

	$effect(() => {
		const f = file;
		selected = null;
		error = '';
		if (!f) return;
		getFileGrants(f.id)
			.then(({ users }) => {
				if (file?.id !== f.id) return;
				saved = new Set(users.map((u) => u.id));
				selected = new Set(saved);
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

	const changed = $derived(
		!!selected && (selected.size !== saved.size || [...selected].some((id) => !saved.has(id)))
	);

	async function save() {
		if (!file || !selected) return;
		saving = true;
		try {
			await setFileGrants(file.id, [...selected]);
			toast.success(selected.size ? `Shared with ${selected.size} ${selected.size === 1 ? 'person' : 'people'}` : 'Access removed');
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
				{:else}
					Organisation files are private to admins. Choose who else can see this one and use it on their cards.
				{/if}
			</Dialog.Description>
		</Dialog.Header>

		{#if file}
			<div class="flex items-center gap-3 rounded-lg border p-2.5">
				<FileThumb id={file.id} kind={file.kind} class="size-10 shrink-0 rounded-md" />
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
			{#if candidates.length === 0}
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
