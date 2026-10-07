<script lang="ts">
	import '../app.css';
	import { Toaster } from '$lib/components/ui/sonner';
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/state';
	import { session } from '$lib/core/session.svelte';
	import { theme } from '$lib/core/theme.svelte';

	let { children } = $props();

	session.load();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Fronko</title>
</svelte:head>

{@render children()}
<Toaster position="top-center" richColors theme={page.url.pathname.startsWith('/dashboard') && theme.dark ? 'dark' : 'light'} />
