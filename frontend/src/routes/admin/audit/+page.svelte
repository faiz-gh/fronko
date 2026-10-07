<script lang="ts">
	import { listAudit, type AuditEntry } from '$lib/features/admin/api';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import Pagination from '$lib/components/shared/pagination.svelte';
	import { formatDateTime } from '$lib/core/format';

	let pageNum = $state(1);
	let pageSize = $state(50);
	let entries = $state<AuditEntry[] | null>(null);
	let total = $state(0);
	let loading = $state(false);
	let error = $state('');

	let requestId = 0;
	async function load() {
		const id = ++requestId;
		loading = true;
		error = '';
		try {
			const res = await listAudit({ page: pageNum, pageSize });
			if (id !== requestId) return;
			entries = res.items;
			total = res.total;
		} catch (e) {
			if (id === requestId) error = e instanceof Error ? e.message : 'Failed to load the audit log';
		} finally {
			if (id === requestId) loading = false;
		}
	}

	$effect(() => {
		void [pageNum, pageSize];
		load();
	});

	const ACTION: Record<string, string> = {
		'admin.login': 'Signed in',
		'org.suspend': 'Suspended organisation',
		'org.reinstate': 'Reinstated organisation',
		'feedback.reply': 'Replied to feedback',
		'feedback.status': 'Changed feedback status'
	};

	/** Where the entry points, and a short description of what changed. */
	function describe(e: AuditEntry): { href?: string; text: string } {
		const d = e.detail;
		const sent = d.email_sent === false ? ' · email not sent' : '';
		switch (e.action) {
			case 'org.suspend':
				return { href: `/admin/orgs/${e.target_id}`, text: `${d.org_name}: “${d.reason}”${sent}` };
			case 'org.reinstate':
				return { href: `/admin/orgs/${e.target_id}`, text: `${d.org_name}${sent}` };
			case 'feedback.reply':
				return { href: `/admin/feedback/${e.target_id}`, text: `Feedback #${e.target_id}${sent}` };
			case 'feedback.status':
				return { href: `/admin/feedback/${e.target_id}`, text: `Feedback #${e.target_id}: ${d.from} → ${d.to}` };
			default:
				return { text: '' };
		}
	}
</script>

<svelte:head>
	<title>Audit log · Fronko admin</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Audit log</h1>
		<p class="text-muted-foreground text-sm">Everything platform admins did, newest first.</p>
	</header>

	{#if error && !entries}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load the audit log</p>
			<p class="text-muted-foreground text-sm">{error}</p>
			<Button variant="outline" onclick={load}>Try again</Button>
		</div>
	{:else if entries === null}
		<Skeleton class="h-64 rounded-xl" />
	{:else}
		<div class="bg-card overflow-hidden rounded-xl border">
			<Table.Root>
				<Table.Header>
					<Table.Row class="bg-muted/40 hover:bg-muted/40">
						<Table.Head class="h-10 pl-5">When</Table.Head>
						<Table.Head class="h-10">Admin</Table.Head>
						<Table.Head class="h-10">Action</Table.Head>
						<Table.Head class="hidden h-10 pr-5 md:table-cell">Details</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body class={loading ? 'opacity-60 transition-opacity' : ''}>
					{#each entries as e (e.id)}
						{@const info = describe(e)}
						<Table.Row>
							<Table.Cell class="text-muted-foreground py-3 pl-5 text-xs whitespace-nowrap">{formatDateTime(e.created_at)}</Table.Cell>
							<Table.Cell class="py-3 text-sm">{e.admin_email}</Table.Cell>
							<Table.Cell class="py-3 text-sm">{ACTION[e.action] ?? e.action}</Table.Cell>
							<Table.Cell class="hidden max-w-md truncate py-3 pr-5 text-sm md:table-cell">
								{#if info.href}
									<a href={info.href} class="hover:underline">{info.text}</a>
								{:else}
									<span class="text-muted-foreground">{info.text}</span>
								{/if}
							</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row class="hover:bg-transparent">
							<Table.Cell colspan={4} class="text-muted-foreground h-32 text-center">Nothing yet.</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
		<Pagination bind:page={pageNum} bind:pageSize {total} pageSizes={[25, 50, 100, 200]} disabled={loading} />
	{/if}
</div>
