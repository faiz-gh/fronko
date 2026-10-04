<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Dialog as DialogPrimitive } from 'bits-ui';
	import MenuIcon from '@lucide/svelte/icons/menu';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import AppSidebar from '$lib/components/app/app-sidebar.svelte';
	import CreateCardDialog from '$lib/components/app/create-card-dialog.svelte';
	import Logo from '$lib/components/app/logo.svelte';
	import { cards } from '$lib/cards.svelte';
	import { orgUsers } from '$lib/org-users.svelte';
	import { session } from '$lib/session.svelte';
	import { storage } from '$lib/storage.svelte';
	import { theme } from '$lib/theme.svelte';

	let { children } = $props();

	let drawerOpen = $state(false);

	// Guard before rendering anything so protected content never flashes.
	// While the session check is in flight (status 'unknown') we just wait.
	$effect(() => {
		const next = encodeURIComponent(page.url.pathname);
		if (session.status === 'anonymous') {
			goto(`/login?next=${next}`, { replaceState: true });
		} else if (session.isAuthenticated && !session.emailVerified) {
			goto(`/verify-email?next=${next}`, { replaceState: true });
		} else if (session.isAuthenticated && session.mustChangePassword) {
			goto(`/set-password?next=${next}`, { replaceState: true });
		}
	});

	const ready = $derived(session.ready);

	$effect(() => {
		if (ready && session.username) {
			cards.load(session.username);
			storage.load(session.username);
			if (session.isAdmin) orgUsers.load(session.username);
		}
	});

	// Theme the whole document (dialogs and menus portal to <body>), but only
	// while in the dashboard; leaving it restores the light marketing pages.
	$effect(() => {
		const root = document.documentElement;
		root.classList.toggle('dark', theme.dark);
		root.style.colorScheme = theme.dark ? 'dark' : 'light';
		return () => {
			root.classList.remove('dark');
			root.style.colorScheme = '';
		};
	});
</script>

{#if session.status === 'unknown' || (session.isAuthenticated && !ready)}
	<div class="grid min-h-svh place-items-center">
		<Spinner class="text-muted-foreground size-6" />
	</div>
{:else if ready}
	<!-- Desktop: fixed sidebar; the page scrolls beside it. -->
	<aside class="bg-sidebar text-sidebar-foreground border-sidebar-border fixed inset-y-0 left-0 z-30 hidden w-68 border-r lg:block">
		<AppSidebar />
	</aside>

	<!-- Mobile: top bar with a slide-in drawer. -->
	<header
		class="bg-background/85 sticky top-0 z-30 flex h-14 items-center justify-between border-b px-4 backdrop-blur lg:hidden"
	>
		<Logo href="/dashboard" />
		<Button variant="ghost" size="icon" onclick={() => (drawerOpen = true)} aria-label="Open menu">
			<MenuIcon />
		</Button>
	</header>
	<DialogPrimitive.Root bind:open={drawerOpen}>
		<DialogPrimitive.Portal>
			<DialogPrimitive.Overlay
				class="data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 fixed inset-0 z-50 bg-black/30 lg:hidden"
			/>
			<DialogPrimitive.Content
				class="bg-sidebar text-sidebar-foreground data-open:animate-in data-closed:animate-out data-closed:slide-out-to-left data-open:slide-in-from-left fixed inset-y-0 left-0 z-50 w-[min(18rem,85vw)] border-r shadow-xl duration-200 outline-none lg:hidden"
			>
				<DialogPrimitive.Title class="sr-only">Menu</DialogPrimitive.Title>
				<AppSidebar onnavigate={() => (drawerOpen = false)} />
			</DialogPrimitive.Content>
		</DialogPrimitive.Portal>
	</DialogPrimitive.Root>

	<div class="min-h-svh lg:pl-68">
		{@render children()}
	</div>

	<CreateCardDialog />
{/if}
