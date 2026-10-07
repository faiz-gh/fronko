<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import { formatDateTime, timeAgo } from '$lib/core/format';
	import { addDomain, deleteDomain, listDomains, verifyDomain } from '../api';
	import type { OrgDomain } from '../types';
	import CopyField from './copy-field.svelte';
	import StatusBadge from './status-badge.svelte';

	/**
	 * The organisation's email domains. Verified ones send people who sign in
	 * with their work email to single sign-on, and let single sign-on and
	 * directories mark their addresses as verified.
	 */
	let domains = $state<OrgDomain[] | null>(null);
	let loadError = $state('');
	let newDomain = $state('');
	let adding = $state(false);
	let addError = $state('');
	let busy = $state<number | null>(null);

	onMount(async () => {
		try {
			domains = await listDomains();
		} catch (e) {
			loadError = e instanceof Error ? e.message : 'Failed to load domains';
		}
	});

	async function add(event: SubmitEvent) {
		event.preventDefault();
		addError = '';
		adding = true;
		try {
			const d = await addDomain(newDomain.trim());
			domains = [...(domains ?? []), d].sort((a, b) => a.domain.localeCompare(b.domain));
			newDomain = '';
		} catch (e) {
			addError = e instanceof Error ? e.message : 'Failed to add the domain';
		} finally {
			adding = false;
		}
	}

	function replace(d: OrgDomain) {
		domains = domains?.map((x) => (x.id === d.id ? d : x)) ?? null;
	}

	async function verify(d: OrgDomain) {
		busy = d.id;
		try {
			replace(await verifyDomain(d.id));
			toast.success(`${d.domain} verified`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Verification failed');
		} finally {
			busy = null;
		}
	}

	async function remove(d: OrgDomain) {
		busy = d.id;
		try {
			await deleteDomain(d.id);
			domains = domains?.filter((x) => x.id !== d.id) ?? null;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to remove the domain');
		} finally {
			busy = null;
		}
	}
</script>

<section class="bg-card flex flex-col gap-4 rounded-xl border p-4 sm:p-5">
	<div class="flex flex-col gap-1">
		<h3 class="flex items-center gap-2 font-semibold"><GlobeIcon class="size-4" /> Email domains</h3>
		<p class="text-muted-foreground text-sm">
			Verify the domains of your work email addresses. People can then sign in with just their email, and single sign-on
			marks their address as confirmed.
		</p>
	</div>

	{#if loadError}
		<p class="text-destructive text-sm">{loadError}</p>
	{:else if domains === null}
		<Skeleton class="h-16 rounded-lg" />
	{:else}
		{#each domains as d (d.id)}
			<div class="flex flex-col gap-3 rounded-lg border p-3">
				<div class="flex flex-wrap items-center gap-2">
					<span class="font-medium">{d.domain}</span>
					{#if d.verified_at}
						<StatusBadge tone="success" label="Verified" />
						<time class="text-muted-foreground text-xs" title={formatDateTime(d.verified_at)}
							>{timeAgo(d.verified_at)}</time
						>
					{:else}
						<StatusBadge tone="warning" label="Not verified" />
					{/if}
					<div class="ml-auto flex gap-2">
						{#if !d.verified_at}
							<Button size="sm" onclick={() => verify(d)} disabled={busy === d.id}>
								{#if busy === d.id}<Spinner data-icon="inline-start" />{:else}<CircleCheckIcon
										data-icon="inline-start"
									/>{/if}
								Verify
							</Button>
						{/if}
						<Button
							size="sm"
							variant="ghost"
							class="text-destructive"
							onclick={() => remove(d)}
							disabled={busy === d.id}
							aria-label="Remove {d.domain}"
						>
							<Trash2Icon />
						</Button>
					</div>
				</div>
				{#if !d.verified_at}
					<p class="text-muted-foreground text-sm">
						Add this TXT record to {d.domain}'s DNS, then click Verify. It can take a while to appear.
					</p>
					<div class="grid gap-3 sm:grid-cols-[1fr_2fr]">
						<CopyField id="txt-name-{d.id}" label="Name (host)" value={d.txt_name} help="Or @, in most DNS editors" />
						<CopyField id="txt-value-{d.id}" label="Value" value={d.txt_value} />
					</div>
				{/if}
			</div>
		{/each}

		<form onsubmit={add} class="flex flex-col gap-2">
			<Field.Field data-invalid={addError ? true : undefined}>
				<Field.Label for="new-domain" class="sr-only">Domain</Field.Label>
				<div class="flex gap-2">
					<Input
						id="new-domain"
						bind:value={newDomain}
						placeholder="example.com"
						autocapitalize="none"
						spellcheck={false}
						disabled={adding}
						aria-invalid={addError ? true : undefined}
					/>
					<Button type="submit" variant="outline" disabled={adding || !newDomain.trim()}>
						{#if adding}<Spinner data-icon="inline-start" />{:else}<PlusIcon data-icon="inline-start" />{/if}
						Add domain
					</Button>
				</div>
				{#if addError}<Field.Error>{addError}</Field.Error>{/if}
			</Field.Field>
		</form>
	{/if}
</section>
