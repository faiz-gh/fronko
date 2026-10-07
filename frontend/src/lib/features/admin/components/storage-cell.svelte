<script lang="ts">
	import type { OrgUsage } from '$lib/features/admin/api';
	import { formatBytes } from '$lib/features/files/api';

	let { org }: { org: OrgUsage } = $props();

	const PROVIDER: Record<string, string> = {
		r2: 'Cloudflare R2',
		b2: 'Backblaze B2',
		s3: 'Amazon S3',
		minio: 'MinIO',
		other: 'S3-compatible'
	};
</script>

{#if org.storage_connected}
	<span class="flex flex-col">
		<span class="tabular">{formatBytes(org.storage_used_bytes)}</span>
		<span class="text-muted-foreground text-xs">
			{PROVIDER[org.storage_provider ?? ''] ?? org.storage_provider}{org.storage_verified ? '' : ' · not verified'}
		</span>
	</span>
{:else}
	<span class="text-muted-foreground">Not connected</span>
{/if}
