<script lang="ts">
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import QrCodeIcon from '@lucide/svelte/icons/qr-code';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { setCardAssignee } from '$lib/api/org';
	import type { Profile } from '$lib/api/profile';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import CardAvatar from './card-avatar.svelte';
	import UserPicker from './user-picker.svelte';
	import { ACCENTS, normalizeCard, publicUrl } from '$lib/card/card';
	import { cards } from '$lib/cards.svelte';
	import { plural } from '$lib/format';
	import { orgUsers } from '$lib/org-users.svelte';
	import { session } from '$lib/session.svelte';

	let {
		profile,
		onqr,
		ondelete
	}: {
		profile: Profile;
		onqr: (profile: Profile) => void;
		/** Admins only. */
		ondelete?: (profile: Profile) => void;
	} = $props();

	const card = $derived(normalizeCard(profile.data));
	let assigning = $state(false);

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(publicUrl(profile.slug));
			toast.success('Link copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}

	async function assign(value: number | 'none' | null) {
		const to = typeof value === 'number' ? value : null;
		if (to === (profile.assigned_user?.id ?? null)) return;
		assigning = true;
		try {
			cards.upsert(await setCardAssignee(profile.id, to));
			orgUsers.refresh();
			toast.success(to === null ? 'Card returned to the organisation' : `Assigned to ${orgUsers.byId(to)?.username}`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Could not assign the card');
		} finally {
			assigning = false;
		}
	}
</script>

<article
	class="bg-card group relative flex flex-col overflow-hidden rounded-xl border transition-shadow hover:shadow-md"
	style="--card-accent: {ACCENTS[card.accent]}"
>
	<div
		class="h-20 bg-(--card-accent)"
		style="background-image: radial-gradient(120% 160% at 100% 0%, oklch(1 0 0 / 0.28), transparent 55%), radial-gradient(90% 140% at 0% 100%, oklch(0 0 0 / 0.2), transparent 60%)"
	></div>
	<div class="flex flex-1 flex-col gap-3 px-5 pb-4">
		<div class="-mt-7 flex items-end justify-between">
			<CardAvatar {card} fallback={profile.slug} class="ring-card size-14 text-base ring-4" />
			<div
				class="relative z-10 flex gap-0.5 opacity-100 transition-opacity lg:opacity-0 lg:group-focus-within:opacity-100 lg:group-hover:opacity-100"
			>
				<Button variant="ghost" size="icon-sm" onclick={copyLink} aria-label="Copy link">
					<CopyIcon />
				</Button>
				<Button variant="ghost" size="icon-sm" onclick={() => onqr(profile)} aria-label="QR code">
					<QrCodeIcon />
				</Button>
				<DropdownMenu.Root>
					<DropdownMenu.Trigger class={buttonVariants({ variant: 'ghost', size: 'icon-sm' })} aria-label="More actions">
						<EllipsisIcon />
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-48">
						<DropdownMenu.Group>
							<DropdownMenu.Item onSelect={() => goto(`/dashboard/${profile.id}`)}>
								<PencilIcon />
								Edit
							</DropdownMenu.Item>
							<DropdownMenu.Item onSelect={() => window.open(`/p/${profile.slug}`, '_blank')}>
								<ExternalLinkIcon />
								View public page
							</DropdownMenu.Item>
							<DropdownMenu.Item onSelect={() => goto(`/dashboard/leads?card=${profile.id}`)}>
								<InboxIcon />
								View leads
							</DropdownMenu.Item>
						</DropdownMenu.Group>
						{#if ondelete}
							<DropdownMenu.Separator />
							<DropdownMenu.Group>
								<DropdownMenu.Item variant="destructive" onSelect={() => ondelete(profile)}>
									<Trash2Icon />
									Delete
								</DropdownMenu.Item>
							</DropdownMenu.Group>
						{/if}
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			</div>
		</div>
		<div class="flex min-w-0 flex-col gap-0.5">
			<h3 class="truncate font-semibold">
				<a href="/dashboard/{profile.id}" class="after:absolute after:inset-0">
					{card.name || profile.slug}
				</a>
			</h3>
			<p class="text-muted-foreground truncate text-sm">
				{[card.title, card.company].filter(Boolean).join(' · ') || 'No title yet'}
			</p>
		</div>
		<div class="text-muted-foreground mt-auto flex items-center justify-between gap-3 border-t pt-3 text-xs">
			{#if session.isAdmin}
				<!-- Above the card's full-size link so it stays clickable. -->
				<div class="relative z-10 min-w-0">
					<UserPicker
						value={profile.assigned_user?.id ?? null}
						onchange={assign}
						filter={false}
						size="sm"
						disabled={assigning}
						class="max-w-44 border-dashed"
					/>
				</div>
			{:else}
				<span class="truncate font-mono">/p/{profile.slug}</span>
			{/if}
			<span class={profile.lead_count > 0 ? 'text-foreground shrink-0 font-medium' : 'shrink-0'}>
				{plural(profile.lead_count, 'lead')}
			</span>
		</div>
	</div>
</article>
