<script lang="ts">
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import NfcIcon from '@lucide/svelte/icons/nfc';
	import QrCodeIcon from '@lucide/svelte/icons/qr-code';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import { Button } from '$lib/components/ui/button';
	import Logo from '$lib/components/app/logo.svelte';
	import ProfileCard from '$lib/components/app/profile-card.svelte';
	import { emptyCard, type CardData } from '$lib/card/card';
	import { session } from '$lib/session.svelte';

	// Illustrative sample shown in the hero.
	const demo: CardData = {
		...emptyCard('Amara Okafor'),
		title: 'Product Designer',
		company: 'Northwind',
		location: 'Lisbon, Portugal',
		bio: 'Designing calm software for busy teams. Say hi at a conference near you.',
		email: 'amara@example.com',
		phone_country: 'PT',
		phone_country_code: '+351',
		phone_number: '900000000',
		website: 'northwind.example',
		accent: 'violet',
		links: [
			{ id: '1', label: '', url: 'https://dribbble.com/amara' },
			{ id: '2', label: 'Portfolio', url: 'https://amara.design' }
		]
	};

	const steps = [
		{
			title: 'Build your card',
			body: 'Add your details, socials and a photo. Keep separate cards for work, events or side projects.'
		},
		{
			title: 'Share it anywhere',
			body: 'Write the link to any NFC card, print the QR code, or send it in a message. No app needed to open it.'
		},
		{
			title: 'Get contacts back',
			body: 'Visitors save you in one tap and can leave their own details, which land in your leads inbox.'
		}
	];

	const ctaHref = $derived(session.isAuthenticated ? '/dashboard' : '/login?mode=register');
</script>

<svelte:head>
	<title>Fronko · Digital business cards</title>
</svelte:head>

<div class="flex min-h-svh flex-col">
	<header class="mx-auto flex w-full max-w-[1400px] items-center justify-between px-5 py-5 sm:px-8 lg:px-12">
		<Logo />
		<nav class="flex items-center gap-2">
			{#if session.status === 'unknown'}
				<!-- Avoid flashing "Sign in" while the session check runs. -->
			{:else if session.isAuthenticated}
				<Button href="/dashboard">
					Dashboard
					<ArrowRightIcon data-icon="inline-end" />
				</Button>
			{:else}
				<Button variant="ghost" href="/login">Sign in</Button>
				<Button href="/login?mode=register">Get started</Button>
			{/if}
		</nav>
	</header>

	<main class="flex-1">
		<section
			class="mx-auto grid w-full max-w-[1400px] items-center gap-12 px-5 pt-8 pb-16 sm:px-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] lg:gap-16 lg:px-12 lg:pt-12 lg:pb-24"
		>
			<div class="flex flex-col gap-7">
				<p class="text-muted-foreground flex items-center gap-2 text-sm font-medium">
					<span class="bg-brand size-1.5 rounded-full"></span>
					Open-source digital business cards
				</p>
				<h1 class="text-[2.75rem] leading-[1.02] font-semibold tracking-[-0.035em] text-balance sm:text-6xl xl:text-7xl">
					Your business card, always up to date.
				</h1>
				<p class="text-muted-foreground max-w-xl text-lg text-pretty sm:text-xl">
					One link, QR code or NFC tap. People save your details in a second and can send theirs right
					back, so no contact gets lost after the event.
				</p>
				<div class="flex flex-wrap gap-3">
					<Button size="lg" href={ctaHref} class="h-11 px-5">
						Create your card
						<ArrowRightIcon data-icon="inline-end" />
					</Button>
				</div>
				<ul class="text-muted-foreground flex flex-wrap gap-x-6 gap-y-2 pt-2 text-sm">
					<li class="flex items-center gap-2"><NfcIcon class="size-4" /> Works with any NFC card</li>
					<li class="flex items-center gap-2"><QrCodeIcon class="size-4" /> Print-ready QR codes</li>
					<li class="flex items-center gap-2"><UserPlusIcon class="size-4" /> One-tap save to contacts</li>
				</ul>
			</div>

			<!-- Product showcase: the visitor's view of a card, on a canvas. -->
			<div class="bg-muted/50 bg-dots relative grid place-items-center overflow-hidden rounded-3xl border px-6 py-10 sm:px-10 sm:py-14">
				<div
					class="absolute inset-x-10 top-10 bottom-10 -z-0 rounded-full opacity-40 blur-3xl"
					style="background: radial-gradient(closest-side, oklch(0.53 0.24 293 / 0.45), transparent)"
					aria-hidden="true"
				></div>
				<div class="relative w-full max-w-[360px]">
					<ProfileCard card={demo} slug="amara">
						{#snippet actions()}
							<Button size="lg" class="h-11 w-full text-white" style="background: var(--card-accent)" tabindex={-1}>
								<UserPlusIcon data-icon="inline-start" />
								Save contact
							</Button>
						{/snippet}
					</ProfileCard>
					<!-- Illustrative lead notification. -->
					<div
						class="bg-card absolute -right-4 -bottom-7 hidden w-60 items-start gap-3 rounded-xl border p-3 shadow-xl sm:-right-16 sm:flex"
						aria-hidden="true"
					>
						<span class="bg-brand-soft text-brand grid size-8 shrink-0 place-items-center rounded-full">
							<UserPlusIcon class="size-4" />
						</span>
						<span class="flex flex-col gap-0.5">
							<span class="text-sm font-medium">New lead from your card</span>
							<span class="text-muted-foreground text-xs">Jordan shared their email</span>
						</span>
					</div>
				</div>
			</div>
		</section>

		<section id="how" class="border-t">
			<div class="mx-auto w-full max-w-[1400px] px-5 py-16 sm:px-8 lg:px-12 lg:py-24">
				<h2 class="max-w-xl text-3xl font-semibold tracking-tight text-balance sm:text-4xl">
					From handshake to saved contact in one tap.
				</h2>
				<ol class="mt-12 grid gap-10 md:grid-cols-3 md:gap-8">
					{#each steps as step, i (step.title)}
						<li class="flex flex-col gap-3 border-t pt-6">
							<span class="text-brand tabular font-mono text-sm">0{i + 1}</span>
							<h3 class="text-lg font-semibold">{step.title}</h3>
							<p class="text-muted-foreground leading-relaxed text-pretty">{step.body}</p>
						</li>
					{/each}
				</ol>
			</div>
		</section>

		<section class="px-5 pb-16 sm:px-8 lg:px-12 lg:pb-24">
			<div
				class="bg-primary text-primary-foreground mx-auto flex w-full max-w-[1400px] flex-col items-start justify-between gap-6 rounded-3xl px-8 py-12 sm:px-12 lg:flex-row lg:items-center"
			>
				<div class="flex flex-col gap-2">
					<h2 class="text-2xl font-semibold tracking-tight sm:text-3xl">Self-host it, own your contacts.</h2>
					<p class="text-primary-foreground/70 max-w-xl">
						Fronko is open source and runs on your own server with Docker Compose.
					</p>
				</div>
				<Button size="lg" variant="secondary" href={ctaHref} class="h-11 px-5">
					Create your card
					<ArrowRightIcon data-icon="inline-end" />
				</Button>
			</div>
		</section>
	</main>

	<footer class="border-t">
		<div class="text-muted-foreground mx-auto flex w-full max-w-[1400px] items-center justify-between px-5 py-6 text-sm sm:px-8 lg:px-12">
			<Logo class="text-foreground text-sm" />
			<span>Open source and self-hostable.</span>
		</div>
	</footer>
</div>
