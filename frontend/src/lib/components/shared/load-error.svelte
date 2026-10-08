<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils';

	let {
		what,
		message,
		onretry,
		class: className
	}: {
		/** What failed to load, for the title: "Couldn't load {what}". */
		what: string;
		message?: string;
		/** Shows a "Try again" button. */
		onretry?: () => void;
		class?: string;
	} = $props();
</script>

<div role="alert" class={cn('bg-card flex flex-col items-start gap-3 rounded-xl border p-6', className)}>
	<p class="font-medium">Couldn't load {what}</p>
	{#if message}
		<p class="text-muted-foreground text-sm">{message}</p>
	{/if}
	{#if onretry}
		<Button variant="outline" onclick={onretry}>Try again</Button>
	{/if}
</div>
