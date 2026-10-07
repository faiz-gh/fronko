<script lang="ts">
	import { cn } from '$lib/utils';
	import type { Tone } from '../registry';

	let { tone, label, class: className }: { tone: Tone; label: string; class?: string } = $props();

	const TONES: Record<Tone, string> = {
		success: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
		warning: 'bg-amber-500/10 text-amber-700 dark:text-amber-400',
		danger: 'bg-destructive/10 text-destructive',
		muted: 'bg-muted text-muted-foreground'
	};
	const DOTS: Record<Tone, string> = {
		success: 'bg-emerald-500',
		warning: 'bg-amber-500',
		danger: 'bg-destructive',
		muted: 'bg-muted-foreground/50'
	};
</script>

<span
	class={cn(
		'inline-flex h-5 shrink-0 items-center gap-1.5 rounded-full px-2 text-xs font-medium whitespace-nowrap',
		TONES[tone],
		className
	)}
>
	<span class={cn('size-1.5 rounded-full', DOTS[tone])}></span>
	{label}
</span>
