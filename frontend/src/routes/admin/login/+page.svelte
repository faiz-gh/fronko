<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import ShieldIcon from '@lucide/svelte/icons/shield';
	import { adminLogin } from '$lib/api/admin';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import Logo from '$lib/components/app/logo.svelte';
	import { adminSession } from '$lib/admin-session.svelte';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	const expired = page.url.searchParams.has('expired');

	// Only follow paths inside the admin panel.
	function nextPath(): string {
		const next = page.url.searchParams.get('next');
		return next && next.startsWith('/admin') && !next.startsWith('/admin/login') ? next : '/admin';
	}

	$effect(() => {
		if (adminSession.isAuthenticated && !loading) goto(nextPath(), { replaceState: true });
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		loading = true;
		try {
			adminSession.signIn(await adminLogin(email.trim(), password));
		} catch (e) {
			error = e instanceof Error ? e.message : 'Something went wrong';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Admin sign in · Fronko</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<div class="flex min-h-svh flex-col gap-4 p-6 md:p-10">
	<Logo />
	<div class="flex flex-1 items-center justify-center">
		<div class="flex w-full max-w-sm flex-col gap-7">
			<div class="flex flex-col gap-2">
				<span class="bg-brand-soft text-brand grid size-10 place-items-center rounded-full">
					<ShieldIcon class="size-5" />
				</span>
				<h1 class="text-3xl font-semibold tracking-tight">Platform admin</h1>
				<p class="text-muted-foreground">Sign in to see how organisations use Fronko and read feedback.</p>
			</div>

			{#if error}
				<Alert.Root variant="destructive">
					<CircleAlertIcon />
					<Alert.Title>{error}</Alert.Title>
				</Alert.Root>
			{:else if expired}
				<Alert.Root>
					<CircleAlertIcon />
					<Alert.Title>Your session expired</Alert.Title>
					<Alert.Description>Please sign in again to continue.</Alert.Description>
				</Alert.Root>
			{/if}

			<form onsubmit={handleSubmit}>
				<Field.Group>
					<Field.Field>
						<Field.Label for="admin-email">Email</Field.Label>
						<Input
							id="admin-email"
							type="email"
							autocomplete="username"
							autocapitalize="none"
							spellcheck={false}
							required
							bind:value={email}
							disabled={loading}
						/>
					</Field.Field>
					<Field.Field>
						<Field.Label for="admin-password">Password</Field.Label>
						<Input
							id="admin-password"
							type="password"
							autocomplete="current-password"
							required
							bind:value={password}
							disabled={loading}
						/>
					</Field.Field>
					<Button type="submit" size="lg" class="w-full" disabled={loading || !email || !password}>
						{#if loading}<Spinner data-icon="inline-start" />{/if}
						Sign in
					</Button>
				</Field.Group>
			</form>
		</div>
	</div>
</div>
