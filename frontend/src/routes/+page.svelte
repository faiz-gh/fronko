<script lang="ts">
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import CheckIcon from '@lucide/svelte/icons/check';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import LayersIcon from '@lucide/svelte/icons/layers';
	import NfcIcon from '@lucide/svelte/icons/nfc';
	import QrCodeIcon from '@lucide/svelte/icons/qr-code';
	import ServerIcon from '@lucide/svelte/icons/server';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import UsersIcon from '@lucide/svelte/icons/users';
	import { Button } from '$lib/components/ui/button';
	import BrandIcon from '$lib/components/shared/brand-icon.svelte';
	import Logo from '$lib/components/shared/logo.svelte';
	import ProfileCard from '$lib/features/cards/components/profile-card.svelte';
	import QrCode from '$lib/features/cards/components/qr-code.svelte';
	import { emptyCard, type CardData } from '$lib/features/cards/card';
	import { session } from '$lib/core/session.svelte';

	const REPO = 'https://github.com/faiz-gh/fronko';

	// Illustrative sample shown in the hero, in the logo's orange.
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
		accent: 'orange',
		links: [
			{ id: '1', label: '', url: 'https://dribbble.com/amara' },
			{ id: '2', label: 'Portfolio', url: 'https://amara.design' }
		]
	};

	const features = [
		{
			icon: NfcIcon,
			title: 'Tap, scan or send',
			body: 'Write the link to any NFC card or print the QR code. The card opens in the browser, with nothing to install.'
		},
		{
			icon: UserPlusIcon,
			title: 'One-tap save',
			body: 'Visitors download a vCard that iOS and Android import straight into their contacts.'
		},
		{
			icon: InboxIcon,
			title: 'Leads come back to you',
			body: 'People share their details from your card. Every lead lands in one inbox with search, filters and CSV export.'
		},
		{
			icon: LayersIcon,
			title: 'A card for every context',
			body: 'Separate cards for work, events or side projects, each with its own link, accent colour and theme.'
		},
		{
			icon: FileTextIcon,
			title: 'Brochures on the card',
			body: 'Attach PDFs from your file library. Visitors open them through short-lived signed links.'
		},
		{
			icon: ShieldCheckIcon,
			title: 'Your data, your bucket',
			body: 'Photos and files go to your own S3-compatible storage. Storage keys are encrypted at rest.'
		}
	];

	const steps = [
		{
			title: 'Build your card',
			body: 'Add your details, socials, a photo and a booking link. The editor shows a live preview as you type.'
		},
		{
			title: 'Share it anywhere',
			body: 'Tap an NFC card, show the QR code, or paste the link into a message or email signature.'
		},
		{
			title: 'Follow up',
			body: 'Contacts who share their details back show up in your leads inbox, ready to export.'
		}
	];

	const roles = [
		{ name: 'Priya Shah', role: 'Owner', cards: 2, tint: 'bg-orange-100 text-orange-700' },
		{ name: 'Marcus Lee', role: 'Admin', cards: 1, tint: 'bg-sky-100 text-sky-700' },
		{ name: 'Jordan Blake', role: 'Member', cards: 1, tint: 'bg-emerald-100 text-emerald-700' },
		{ name: 'Sofia Romero', role: 'Member', cards: 1, tint: 'bg-violet-100 text-violet-700' }
	];

	const teamPoints = [
		'Admins create accounts and assign cards to people',
		'Each person sees only their own cards and leads',
		'Shared, private and personal file areas with per-person limits',
		'Suspend or remove users without losing their leads'
	];

	const ctaHref = $derived(session.isAuthenticated ? '/dashboard' : '/login?mode=register');
	const initials = (name: string) =>
		name
			.split(' ')
			.map((part) => part[0])
			.join('');
</script>

<svelte:head>
	<title>Fronko · Digital business cards</title>
	<meta
		name="description"
		content="Open-source digital business cards. Share by NFC, QR code or link, let people save you in one tap, and collect leads in one inbox."
	/>
</svelte:head>

{#snippet sectionLabel(text: string)}
	<p class="flex items-center gap-2 text-sm font-medium text-orange-600">
		<span class="h-px w-5 bg-orange-600/60"></span>
		{text}
	</p>
{/snippet}

<div class="flex min-h-svh flex-col">
	<header class="bg-background/80 sticky top-0 z-30 border-b border-black/5 backdrop-blur-md">
		<div class="mx-auto flex h-16 w-full max-w-[1280px] items-center justify-between gap-6 px-5 sm:px-8 lg:h-[72px]">
			<Logo size="lg" />
			<nav class="text-muted-foreground hidden items-center gap-8 text-sm font-medium md:flex">
				<a href="#features" class="hover:text-foreground transition-colors">Features</a>
				<a href="#how" class="hover:text-foreground transition-colors">How it works</a>
				<a href="#teams" class="hover:text-foreground transition-colors">Teams</a>
				<a href={REPO} class="hover:text-foreground transition-colors">GitHub</a>
			</nav>
			<div class="flex items-center gap-2">
				{#if session.status === 'unknown'}
					<!-- Avoid flashing "Sign in" while the session check runs. -->
				{:else if session.isAuthenticated}
					<Button href="/dashboard">
						Dashboard
						<ArrowRightIcon data-icon="inline-end" />
					</Button>
				{:else}
					<Button variant="ghost" href="/login" class="hidden sm:inline-flex">Sign in</Button>
					<Button href="/login?mode=register">Get started</Button>
				{/if}
			</div>
		</div>
	</header>

	<main class="flex-1">
		<!-- Hero -->
		<section class="relative overflow-hidden">
			<div
				class="pointer-events-none absolute inset-0 bg-dots opacity-60 [mask-image:radial-gradient(60%_70%_at_75%_40%,black,transparent)]"
				aria-hidden="true"
			></div>
			<div
				class="mx-auto grid w-full max-w-[1280px] items-center gap-14 px-5 pt-14 pb-20 sm:px-8 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)] lg:gap-10 lg:pt-20 lg:pb-28"
			>
				<div class="relative flex flex-col items-start gap-7">
					<a
						href={REPO}
						class="bg-card text-muted-foreground hover:text-foreground inline-flex items-center gap-2 rounded-full border py-1 pr-3 pl-1 text-sm shadow-xs transition-colors"
					>
						<span class="rounded-full bg-orange-600/10 px-2 py-0.5 text-xs font-semibold text-orange-700"
							>Open source</span
						>
						<span>Self-host it <span class="hidden sm:inline">with Docker Compose</span></span>
						<ArrowRightIcon class="size-3.5" />
					</a>
					<h1 class="text-[2.75rem] leading-[1.02] font-semibold tracking-[-0.04em] sm:text-6xl xl:text-[4rem]">
						Your business card,
						<span class="block text-orange-600">always up to date.</span>
					</h1>
					<p class="text-muted-foreground max-w-xl text-lg leading-relaxed text-pretty sm:text-xl">
						Share your details with an NFC tap, a QR code or a link. People save you in one tap and send their details
						right back, so no contact gets lost after the event.
					</p>
					<div class="flex flex-wrap gap-3">
						<Button size="lg" href={ctaHref} class="h-12 px-6 text-[15px]">
							Create your free card
							<ArrowRightIcon data-icon="inline-end" />
						</Button>
						<Button size="lg" variant="outline" href={REPO} class="h-12 px-5 text-[15px]">
							<BrandIcon url={REPO} class="size-4" />
							Star on GitHub
						</Button>
					</div>
					<ul class="text-muted-foreground grid gap-x-6 gap-y-2 pt-1 text-sm sm:flex sm:flex-wrap">
						<li class="flex items-center gap-2"><CheckIcon class="size-4 text-orange-600" /> No app for visitors</li>
						<li class="flex items-center gap-2"><CheckIcon class="size-4 text-orange-600" /> Unlimited cards</li>
						<li class="flex items-center gap-2"><CheckIcon class="size-4 text-orange-600" /> You own every lead</li>
					</ul>
				</div>

				<!-- Product showcase: the visitor's view of a card. -->
				<div class="relative mx-auto w-full max-w-[520px] py-6 lg:py-0">
					<div
						class="absolute top-1/2 left-1/2 size-[560px] max-w-full -translate-x-1/2 -translate-y-1/2 rounded-full opacity-60 blur-3xl"
						style="background: radial-gradient(closest-side, oklch(0.7 0.19 45 / 0.45), transparent)"
						aria-hidden="true"
					></div>
					<div class="relative mx-auto w-full max-w-[350px] [&>*]:shadow-2xl [&>*]:shadow-orange-950/10">
						<ProfileCard card={demo} slug="amara">
							{#snippet actions()}
								<Button size="lg" class="h-11 w-full text-white" style="background: var(--card-accent)" tabindex={-1}>
									<UserPlusIcon data-icon="inline-start" />
									Save contact
								</Button>
							{/snippet}
						</ProfileCard>
					</div>

					<!-- Illustrative QR and lead callouts around the card. -->
					<div
						class="bg-card absolute top-16 -left-2 hidden w-32 flex-col gap-2 rounded-2xl border p-2.5 shadow-xl sm:flex lg:-left-6"
						aria-hidden="true"
					>
						<QrCode url="https://fronko.example/p/acme/amara" class="p-1.5 shadow-none ring-0" />
						<span class="text-muted-foreground flex items-center justify-center gap-1.5 text-xs font-medium">
							<QrCodeIcon class="size-3.5" /> Scan to open
						</span>
					</div>
					<div
						class="bg-card absolute -right-2 -bottom-6 hidden w-64 items-start gap-3 rounded-2xl border p-3.5 shadow-xl sm:flex lg:-right-10"
						aria-hidden="true"
					>
						<span class="grid size-9 shrink-0 place-items-center rounded-full bg-orange-100 text-orange-700">
							<InboxIcon class="size-4" />
						</span>
						<span class="flex flex-col gap-0.5">
							<span class="text-sm font-semibold">New lead from your card</span>
							<span class="text-muted-foreground text-xs">Jordan Blake shared their email</span>
						</span>
					</div>
				</div>
			</div>
		</section>

		<!-- Features -->
		<section id="features" class="bg-card scroll-mt-16 border-y">
			<div class="mx-auto w-full max-w-[1280px] px-5 py-20 sm:px-8 lg:py-28">
				<div class="flex max-w-2xl flex-col gap-4">
					{@render sectionLabel('Features')}
					<h2 class="text-3xl font-semibold tracking-[-0.03em] text-balance sm:text-[2.75rem] sm:leading-[1.1]">
						Everything a paper card does, and everything it can't.
					</h2>
					<p class="text-muted-foreground text-lg text-pretty">
						A card that updates itself, works on every phone, and tells you who you met.
					</p>
				</div>
				<div class="mt-14 grid gap-px overflow-hidden rounded-2xl border bg-border sm:grid-cols-2 lg:grid-cols-3">
					{#each features as feature (feature.title)}
						<div class="bg-card flex flex-col gap-4 p-7 lg:p-8">
							<span class="grid size-10 place-items-center rounded-xl bg-orange-600/10 text-orange-600">
								<feature.icon class="size-5" />
							</span>
							<div class="flex flex-col gap-1.5">
								<h3 class="text-base font-semibold">{feature.title}</h3>
								<p class="text-muted-foreground leading-relaxed text-pretty">{feature.body}</p>
							</div>
						</div>
					{/each}
				</div>
			</div>
		</section>

		<!-- How it works -->
		<section id="how" class="scroll-mt-16">
			<div class="mx-auto w-full max-w-[1280px] px-5 py-20 sm:px-8 lg:py-28">
				<div class="flex max-w-2xl flex-col gap-4">
					{@render sectionLabel('How it works')}
					<h2 class="text-3xl font-semibold tracking-[-0.03em] text-balance sm:text-[2.75rem] sm:leading-[1.1]">
						From handshake to saved contact in one tap.
					</h2>
				</div>
				<ol class="mt-14 grid gap-6 md:grid-cols-3">
					{#each steps as step, i (step.title)}
						<li class="bg-card relative flex flex-col gap-3 rounded-2xl border p-7 lg:p-8">
							<span
								class="grid size-9 place-items-center rounded-full bg-orange-600 text-sm font-semibold text-white tabular"
							>
								{i + 1}
							</span>
							<h3 class="mt-3 text-lg font-semibold">{step.title}</h3>
							<p class="text-muted-foreground leading-relaxed text-pretty">{step.body}</p>
						</li>
					{/each}
				</ol>
			</div>
		</section>

		<!-- Teams -->
		<section id="teams" class="bg-card scroll-mt-16 border-y">
			<div
				class="mx-auto grid w-full max-w-[1280px] items-center gap-14 px-5 py-20 sm:px-8 lg:grid-cols-2 lg:gap-20 lg:py-28"
			>
				<div class="flex flex-col gap-5">
					{@render sectionLabel('For teams')}
					<h2 class="text-3xl font-semibold tracking-[-0.03em] text-balance sm:text-[2.75rem] sm:leading-[1.1]">
						Cards for the whole team, run from one place.
					</h2>
					<p class="text-muted-foreground text-lg text-pretty">
						Every sign-up is an organisation. Invite your team, hand out cards, and keep the leads with the company when
						people move on.
					</p>
					<ul class="mt-2 flex flex-col gap-3">
						{#each teamPoints as point (point)}
							<li class="flex items-start gap-3">
								<span
									class="mt-0.5 grid size-5 shrink-0 place-items-center rounded-full bg-orange-600/10 text-orange-600"
								>
									<CheckIcon class="size-3.5" />
								</span>
								<span>{point}</span>
							</li>
						{/each}
					</ul>
				</div>

				<!-- Illustrative team list. -->
				<div class="bg-background rounded-2xl border p-2 shadow-sm" aria-hidden="true">
					<div class="flex items-center justify-between px-4 pt-3 pb-4">
						<span class="flex items-center gap-2 text-sm font-semibold">
							<UsersIcon class="text-muted-foreground size-4" /> Northwind · Users
						</span>
						<span class="bg-primary text-primary-foreground rounded-md px-2.5 py-1 text-xs font-medium">New user</span>
					</div>
					<div class="bg-card divide-y rounded-xl border">
						{#each roles as person (person.name)}
							<div class="flex items-center gap-3 px-4 py-3.5">
								<span class="grid size-9 shrink-0 place-items-center rounded-full text-xs font-semibold {person.tint}">
									{initials(person.name)}
								</span>
								<span class="flex min-w-0 flex-1 flex-col">
									<span class="truncate text-sm font-medium">{person.name}</span>
									<span class="text-muted-foreground text-xs">
										{person.cards} card{person.cards === 1 ? '' : 's'}
									</span>
								</span>
								<span class="text-muted-foreground rounded-full border px-2.5 py-0.5 text-xs font-medium">
									{person.role}
								</span>
							</div>
						{/each}
					</div>
				</div>
			</div>
		</section>

		<!-- Self-host CTA -->
		<section class="px-5 py-20 sm:px-8 lg:py-28">
			<div
				class="bg-primary text-primary-foreground relative mx-auto grid w-full max-w-[1280px] items-center gap-10 overflow-hidden rounded-3xl px-8 py-14 sm:px-12 lg:grid-cols-[minmax(0,1fr)_auto] lg:px-16 lg:py-16"
			>
				<div
					class="absolute inset-0 opacity-60"
					style="background-image: radial-gradient(oklch(1 0 0 / 0.1) 1px, transparent 1px); background-size: 18px 18px; mask-image: radial-gradient(60% 80% at 85% 50%, black, transparent)"
					aria-hidden="true"
				></div>
				<div
					class="absolute -top-40 -right-20 size-[520px] rounded-full opacity-50 blur-3xl"
					style="background: radial-gradient(closest-side, oklch(0.64 0.19 45 / 0.6), transparent)"
					aria-hidden="true"
				></div>

				<div class="relative flex flex-col gap-4">
					<span class="text-primary-foreground/60 flex items-center gap-2 text-sm font-medium">
						<ServerIcon class="size-4" /> Self-hosted · GPL-3.0
					</span>
					<h2 class="text-3xl font-semibold tracking-[-0.03em] text-balance sm:text-4xl">
						Own your cards. Own your contacts.
					</h2>
					<p class="text-primary-foreground/70 max-w-xl text-lg text-pretty">
						Run Fronko on your own server with Docker Compose, point it at your Postgres and S3 bucket, and keep every
						lead in-house.
					</p>
					<div class="mt-3 flex flex-wrap gap-3">
						<Button size="lg" variant="secondary" href={ctaHref} class="h-12 px-6 text-[15px]">
							Create your card
							<ArrowRightIcon data-icon="inline-end" />
						</Button>
						<Button
							size="lg"
							variant="ghost"
							href={REPO}
							class="text-primary-foreground hover:text-primary-foreground h-12 px-5 text-[15px] hover:bg-white/10"
						>
							<BrandIcon url={REPO} class="size-4" />
							View the source
						</Button>
					</div>
				</div>

				<pre
					class="relative hidden rounded-2xl border border-white/10 bg-black/30 p-5 font-mono text-[13px] leading-6 text-white/80 lg:block"
					aria-label="Setup commands"><span class="text-white/40">$</span> git clone {REPO}.git
<span class="text-white/40">$</span> cd fronko/deploy
<span class="text-white/40">$</span> cp .env.example .env
<span class="text-white/40">$</span> docker compose up -d --build
<span class="text-orange-400">✓</span> fronko is running</pre>
			</div>
		</section>
	</main>

	<footer class="border-t">
		<div
			class="text-muted-foreground mx-auto flex w-full max-w-[1280px] flex-col gap-6 px-5 py-10 text-sm sm:flex-row sm:items-center sm:justify-between sm:px-8"
		>
			<div class="flex flex-col gap-2">
				<Logo class="text-foreground" />
				<span>Open-source digital business cards.</span>
			</div>
			<nav class="flex flex-wrap gap-x-6 gap-y-2">
				<a href="#features" class="hover:text-foreground transition-colors">Features</a>
				<a href="#teams" class="hover:text-foreground transition-colors">Teams</a>
				<a href={REPO} class="hover:text-foreground transition-colors">GitHub</a>
				<a href="/login" class="hover:text-foreground transition-colors">Sign in</a>
			</nav>
		</div>
	</footer>
</div>
