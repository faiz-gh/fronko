<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Dialog as DialogPrimitive } from 'bits-ui';
	import MenuIcon from '@lucide/svelte/icons/menu';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import AdminSidebar from '$lib/features/admin/components/admin-sidebar.svelte';
	import Logo from '$lib/components/shared/logo.svelte';
	import { adminNav } from '$lib/features/admin/nav.svelte';
	import { adminSession } from '$lib/features/admin/session.svelte';
	import { theme } from '$lib/core/theme.svelte';

	let { children } = $props();

	let drawerOpen = $state(false);

	const onLogin = $derived(page.route.id === '/admin/login');

	adminSession.load();

	// The real check is on the server; this only keeps the panel from flashing for signed-out visitors.
	$effect(() => {
		if (!onLogin && adminSession.status === 'anonymous') {
			goto(`/admin/login?next=${encodeURIComponent(page.url.pathname)}`, { replaceState: true });
		}
	});

	$effect(() => {
		if (adminSession.isAuthenticated) adminNav.refresh();
	});

	// Same theme handling as the dashboard.
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

{#if onLogin}
	{@render children()}
{:else if !adminSession.isAuthenticated}
	<div class="grid min-h-svh place-items-center">
		<Spinner class="text-muted-foreground size-6" />
	</div>
{:else}
	<aside class="bg-sidebar text-sidebar-foreground border-sidebar-border fixed inset-y-0 left-0 z-30 hidden w-64 border-r lg:block">
		<AdminSidebar />
	</aside>

	<header
		class="bg-background/85 sticky top-0 z-30 flex h-14 items-center justify-between border-b px-4 backdrop-blur lg:hidden"
	>
		<Logo href="/admin" />
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
				<AdminSidebar onnavigate={() => (drawerOpen = false)} />
			</DialogPrimitive.Content>
		</DialogPrimitive.Portal>
	</DialogPrimitive.Root>

	<div class="min-h-svh lg:pl-64">
		{@render children()}
	</div>
{/if}
