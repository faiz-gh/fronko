<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import AuthLayout from '$lib/features/auth/components/auth-layout.svelte';
	import { ssoStartUrl } from '$lib/features/auth/api';

	// A link an organisation shares: straight to its identity provider.
	const handle = $derived(page.params.handle ?? '');
	const target = $derived(
		ssoStartUrl(`/auth/sso/${encodeURIComponent(handle)}`, page.url.searchParams.get('next') ?? undefined)
	);

	onMount(() => window.location.replace(target));
</script>

<svelte:head>
	<title>Signing in · Fronko</title>
</svelte:head>

<AuthLayout>
	<div class="flex flex-col items-start gap-4">
		<div class="flex items-center gap-3">
			<Spinner />
			<h1 class="text-xl font-semibold tracking-tight">Taking you to your company's sign-in…</h1>
		</div>
		<p class="text-muted-foreground text-sm">If nothing happens, use the button below.</p>
		<Button href={target} variant="outline">Continue</Button>
	</div>
</AuthLayout>
