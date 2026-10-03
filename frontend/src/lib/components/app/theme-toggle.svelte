<script lang="ts">
	import MonitorIcon from '@lucide/svelte/icons/monitor';
	import MoonIcon from '@lucide/svelte/icons/moon';
	import SunIcon from '@lucide/svelte/icons/sun';
	import { theme, type ThemePreference } from '$lib/theme.svelte';
	import { cn } from '$lib/utils';

	const options: { value: ThemePreference; label: string; icon: typeof SunIcon }[] = [
		{ value: 'light', label: 'Light', icon: SunIcon },
		{ value: 'dark', label: 'Dark', icon: MoonIcon },
		{ value: 'system', label: 'System', icon: MonitorIcon }
	];
</script>

<div class="bg-sidebar-accent/70 inline-flex rounded-lg p-[3px]" role="radiogroup" aria-label="Theme">
	{#each options as option (option.value)}
		{@const selected = theme.preference === option.value}
		<button
			type="button"
			role="radio"
			aria-checked={selected}
			aria-label={option.label}
			title={option.label}
			onclick={() => theme.set(option.value)}
			class={cn(
				'focus-visible:ring-ring/50 grid h-7 w-8 place-items-center rounded-md transition-colors outline-none focus-visible:ring-3',
				selected ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
			)}
		>
			<option.icon class="size-4" />
		</button>
	{/each}
</div>
