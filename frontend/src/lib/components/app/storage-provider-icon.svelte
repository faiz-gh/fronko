<script lang="ts">
	import DatabaseIcon from '@lucide/svelte/icons/database';
	import ServerIcon from '@lucide/svelte/icons/server';
	import { siBackblaze, siCloudflare, siMinio, type SimpleIcon } from 'simple-icons';
	import type { StorageProvider } from '$lib/api/storage';

	let { provider, class: className = 'size-4' }: { provider: StorageProvider; class?: string } = $props();

	// simple-icons has no AWS logo (trademark removal), so S3 and "other" use generic icons.
	const BRAND_ICONS: Partial<Record<StorageProvider, SimpleIcon>> = {
		r2: siCloudflare,
		b2: siBackblaze,
		minio: siMinio
	};

	const icon = $derived(BRAND_ICONS[provider]);
</script>

{#if icon}
	<svg viewBox="0 0 24 24" fill="#{icon.hex}" class={className} aria-hidden="true">
		<path d={icon.path} />
	</svg>
{:else if provider === 's3'}
	<DatabaseIcon class={className} aria-hidden="true" />
{:else}
	<ServerIcon class={className} aria-hidden="true" />
{/if}
