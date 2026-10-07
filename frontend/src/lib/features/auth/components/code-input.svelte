<script lang="ts">
	import * as InputOTP from '$lib/components/ui/input-otp';

	let {
		value = $bindable(''),
		id,
		disabled = false,
		invalid = false,
		oncomplete
	}: {
		value?: string;
		id?: string;
		disabled?: boolean;
		invalid?: boolean;
		/** Called once all six digits are in. */
		oncomplete?: (code: string) => void;
	} = $props();
</script>

<!-- Six-digit emailed code. Pasting the whole code fills every slot. -->
<InputOTP.Root
	{id}
	class="w-full"
	bind:value
	maxlength={6}
	pattern="^[0-9]*$"
	inputmode="numeric"
	autocomplete="one-time-code"
	{disabled}
	onComplete={oncomplete}
>
	{#snippet children({ cells })}
		<InputOTP.Group class="w-full">
			{#each cells as cell, i (i)}
				<InputOTP.Slot {cell} aria-invalid={invalid || undefined} class="h-12 flex-1 text-lg font-medium" />
			{/each}
		</InputOTP.Group>
	{/snippet}
</InputOTP.Root>
