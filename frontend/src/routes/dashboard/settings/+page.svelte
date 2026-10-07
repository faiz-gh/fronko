<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import * as Tabs from '$lib/components/ui/tabs';
	import { session } from '$lib/core/session.svelte';
	import { SETTINGS_TABS } from '$lib/core/settings-tabs';

	// Settings are split into tabs; ?tab= remembers the open one so links can point at it.
	const tabs = $derived(SETTINGS_TABS.filter((t) => t.visible?.() ?? true));
	const fallback = SETTINGS_TABS[0].value;
	const tab = $derived.by(() => {
		const requested = page.url.searchParams.get('tab');
		return tabs.some((t) => t.value === requested) ? requested! : fallback;
	});

	function setTab(value: string) {
		goto(value === fallback ? '?' : `?tab=${value}`, { replace: true, reset: false });
	}
</script>

<svelte:head>
	<title>Settings · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1200px] flex-col px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<Tabs.Root value={tab} onValueChange={setTab} class="gap-0">
		<header class="flex flex-col gap-1 border-b">
			<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Settings</h1>
			<p class="text-muted-foreground text-sm">
				{session.isAdmin
					? 'Your account, your organisation and its branding, and storage for photos and brochures.'
					: 'Your account, password and storage.'}
			</p>
			<Tabs.List variant="line" class="mt-4 -mb-px h-10 gap-4 p-0">
				{#each tabs as t (t.value)}
					<Tabs.Trigger value={t.value} class="flex-none px-0.5">{t.label}</Tabs.Trigger>
				{/each}
			</Tabs.List>
		</header>
		{#each tabs as t (t.value)}
			<Tabs.Content value={t.value}>
				<t.component />
			</Tabs.Content>
		{/each}
	</Tabs.Root>
</div>
