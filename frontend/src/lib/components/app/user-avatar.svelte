<script lang="ts">
	import * as Avatar from '$lib/components/ui/avatar';
	import { initials } from '$lib/card/card';
	import { cn } from '$lib/utils';

	let { username, class: className }: { username: string; class?: string } = $props();
</script>

<Avatar.Root class={cn('size-6 text-[10px]', className)}>
	<Avatar.Fallback class="bg-muted font-semibold">{initials(username, '?')}</Avatar.Fallback>
</Avatar.Root>
