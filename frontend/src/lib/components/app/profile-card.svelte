<script lang="ts">
	import type { Snippet } from 'svelte';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import ArrowUpRightIcon from '@lucide/svelte/icons/arrow-up-right';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import * as Avatar from '$lib/components/ui/avatar';
	import BrandIcon from './brand-icon.svelte';
	import { fileUrl, formatBytes, type PublicFile } from '$lib/api/files';
	import {
		ACCENTS,
		avatarSrc,
		coverSrc,
		detectCalendar,
		initials,
		linkLabel,
		safeUrl,
		type CardData
	} from '$lib/card/card';
	import { e164 } from '$lib/phone';
	import { cn } from '$lib/utils';

	let {
		card,
		slug,
		actions,
		files,
		class: className
	}: {
		card: CardData;
		slug: string;
		/**
		 * Metadata for the library files the card uses. When given, brochures whose
		 * file no longer exists are hidden and sizes are shown.
		 */
		files?: Record<string, PublicFile>;
		/** Primary buttons rendered under the header (save contact, exchange, …). */
		actions?: Snippet;
		class?: string;
	} = $props();

	const name = $derived(card.name.trim() || slug);
	const subtitle = $derived([card.title, card.company].map((s) => s.trim()).filter(Boolean).join(' · '));
	const website = $derived(safeUrl(card.website));
	const links = $derived(
		card.links.flatMap((l) => {
			const href = safeUrl(l.url);
			return href ? [{ ...l, href, text: linkLabel(l) }] : [];
		})
	);
	const avatar = $derived(avatarSrc(card));
	const cover = $derived(coverSrc(card));
	const calendar = $derived(safeUrl(card.calendar_url));
	const calendarName = $derived(calendar ? detectCalendar(calendar)?.name : undefined);
	const documents = $derived(
		card.documents.flatMap((d) => {
			const meta = files?.[d.file];
			if (files && !meta) return [];
			const title = d.title.trim() || meta?.name.replace(/\.pdf$/i, '') || 'Brochure';
			return [{ ...d, href: fileUrl(d.file), title, size: meta ? formatBytes(meta.size_bytes) : null }];
		})
	);

	const quick = $derived(
		[
			card.email.trim() && { label: 'Email', href: `mailto:${card.email.trim()}`, icon: MailIcon },
			card.phone_number && { label: 'Call', href: `tel:${e164(card.phone_country_code, card.phone_number)}`, icon: PhoneIcon },
			website && { label: 'Website', href: website, icon: GlobeIcon }
		].filter((q) => !!q)
	);
</script>

<!--
	The card carries its own theme: `.dark` re-scopes the design tokens for this subtree.
	It sizes itself from its container: stacked in narrow spots (phones, the editor
	preview), two columns once it has room (the public page on desktop).
-->
<article
	class={cn(card.theme === 'dark' ? 'dark' : 'light', '@container w-full', className)}
	style="--card-accent: {ACCENTS[card.accent]}"
>
	<div class="bg-card text-card-foreground ring-foreground/10 overflow-hidden rounded-3xl shadow-xl ring-1">
		{#if cover}
			<!-- With a cover photo, the accent becomes a frame: a stripe under the banner and a ring around the avatar. -->
			<div class="bg-muted aspect-3/1 border-b-4 border-(--card-accent) @2xl:aspect-auto @2xl:h-56">
				<img src={cover} alt="" class="size-full object-cover" />
			</div>
		{:else}
			<div
				class="h-28 bg-(--card-accent) @2xl:h-40"
				style="background-image: radial-gradient(120% 140% at 100% 0%, oklch(1 0 0 / 0.28), transparent 55%), radial-gradient(90% 120% at 0% 100%, oklch(0 0 0 / 0.22), transparent 60%)"
			></div>
		{/if}

		<div
			class={cn(
				'flex flex-col gap-5 px-6 pb-6 @2xl:grid @2xl:gap-x-12 @2xl:px-10 @2xl:pb-10',
				(links.length > 0 || documents.length > 0) && '@2xl:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]'
			)}
		>
			<div class="flex flex-col gap-5">
				<div class="-mt-12 flex flex-col items-center gap-3 text-center @2xl:-mt-16 @2xl:items-start @2xl:gap-4 @2xl:text-left">
					<Avatar.Root
						class={cn(
							'size-24 text-2xl ring-4 @2xl:size-32 @2xl:text-3xl',
							cover ? 'bg-card ring-(--card-accent)' : 'ring-card'
						)}
					>
						{#if avatar}
							<Avatar.Image src={avatar} alt={name} class="object-cover" />
						{/if}
						<Avatar.Fallback class="bg-(--card-accent) font-semibold text-white">
							{initials(card.name, slug)}
						</Avatar.Fallback>
					</Avatar.Root>

					<div class="flex flex-col gap-1">
						<h1 class="text-2xl font-semibold tracking-tight text-balance @2xl:text-4xl">{name}</h1>
						{#if subtitle}
							<p class="text-muted-foreground text-sm font-medium @2xl:text-base">{subtitle}</p>
						{/if}
						{#if card.location.trim()}
							<p class="text-muted-foreground flex items-center justify-center gap-1 text-xs @2xl:justify-start @2xl:text-sm">
								<MapPinIcon class="size-3.5" />
								{card.location}
							</p>
						{/if}
					</div>

					{#if card.bio.trim()}
						<p class="text-foreground/80 max-w-prose text-sm leading-relaxed whitespace-pre-line @2xl:text-base">
							{card.bio}
						</p>
					{/if}
				</div>

				{#if quick.length > 0}
					<div class="flex justify-center gap-3 @2xl:justify-start">
						{#each quick as q (q.label)}
							<a
								href={q.href}
								target={q.label === 'Website' ? '_blank' : undefined}
								rel="noopener noreferrer"
								class="group flex w-16 flex-col items-center gap-1.5"
							>
								<span
									class="bg-muted text-foreground grid size-11 place-items-center rounded-full transition-colors group-hover:bg-(--card-accent) group-hover:text-white"
								>
									<q.icon class="size-[18px]" />
								</span>
								<span class="text-muted-foreground text-xs">{q.label}</span>
							</a>
						{/each}
					</div>
				{/if}

				{#if calendar}
					<a
						href={calendar}
						target="_blank"
						rel="noopener noreferrer"
						class="group flex h-12 items-center gap-3 rounded-xl border-2 border-(--card-accent) px-4 text-sm font-semibold transition-colors hover:bg-(--card-accent) hover:text-white"
					>
						<span class="text-(--card-accent) transition-colors group-hover:text-white">
							<BrandIcon url={calendar} kind="calendar" class="size-[18px]" />
						</span>
						<span class="flex-1">Book a meeting</span>
						{#if calendarName}
							<span class="text-muted-foreground text-xs font-normal transition-colors group-hover:text-white/80">
								{calendarName}
							</span>
						{/if}
						<ChevronRightIcon class="size-4 transition-transform group-hover:translate-x-0.5" />
					</a>
				{/if}

				{#if actions}
					<div class="flex flex-col gap-2">
						{@render actions()}
					</div>
				{/if}
			</div>

			{#if links.length > 0 || documents.length > 0}
				<div class="flex flex-col gap-5 @2xl:pt-8">
					{#if links.length > 0}
						<div class="flex flex-col gap-3">
							<p class="text-muted-foreground hidden text-xs font-medium tracking-wide uppercase @2xl:block">Links</p>
							<ul class="flex flex-col gap-2">
								{#each links as link (link.id)}
									<li>
										<a
											href={link.href}
											target="_blank"
											rel="noopener noreferrer"
											class="bg-muted/60 hover:bg-muted group flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-medium transition-colors @2xl:py-3.5"
										>
											<span class="text-(--card-accent)">
												<BrandIcon url={link.href} class="size-[18px]" />
											</span>
											<span class="flex-1 truncate">{link.text}</span>
											<ArrowUpRightIcon
												class="text-muted-foreground size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5"
											/>
										</a>
									</li>
								{/each}
							</ul>
						</div>
					{/if}

					{#if documents.length > 0}
						<div class="flex flex-col gap-3">
							<p class="text-muted-foreground text-xs font-medium tracking-wide uppercase">Brochures</p>
							<ul class="flex flex-col gap-2">
								{#each documents as doc (doc.id)}
									<li>
										<a
											href={doc.href}
											target="_blank"
											rel="noopener noreferrer"
											class="bg-muted/60 hover:bg-muted group flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-medium transition-colors"
										>
											<span class="text-(--card-accent)"><FileTextIcon class="size-[18px]" /></span>
											<span class="flex min-w-0 flex-1 flex-col">
												<span class="truncate">{doc.title}</span>
												<span class="text-muted-foreground text-xs font-normal">PDF{doc.size ? ` · ${doc.size}` : ''}</span>
											</span>
											<ArrowUpRightIcon
												class="text-muted-foreground size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5"
											/>
										</a>
									</li>
								{/each}
							</ul>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
</article>
