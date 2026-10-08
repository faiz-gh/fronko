<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { confirmState } from '$lib/core/confirm.svelte';

	const current = $derived(confirmState.current);
</script>

<!-- Shows whatever confirmDialog() asks; mounted once in the root layout. -->
<AlertDialog.Root open={!!current} onOpenChange={(open) => !open && confirmState.settle(false)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{current?.title}</AlertDialog.Title>
			{#if current?.description}
				<AlertDialog.Description>{current.description}</AlertDialog.Description>
			{/if}
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel onclick={() => confirmState.settle(false)}>
				{current?.cancelLabel ?? 'Cancel'}
			</AlertDialog.Cancel>
			<AlertDialog.Action
				variant={current?.destructive ? 'destructive' : 'default'}
				onclick={() => confirmState.settle(true)}
			>
				{current?.confirmLabel ?? 'Continue'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
