<script lang="ts">
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import { Input } from '$lib/components/ui/input';
	import * as Popover from '$lib/components/ui/popover';
	import { parsePhoneNumberFromString } from 'libphonenumber-js';
	import {
		countries,
		defaultCountry,
		dialCode,
		digits,
		exampleNumber,
		formatNational,
		isCountry
	} from '$lib/core/phone';
	import { cn } from '$lib/utils';

	/**
	 * A country picker plus a number field that formats as you type. Only digits
	 * are written back: `code` is the dial code ("+91") and `number` the national
	 * number ("9876543210"). Both are empty while there is no number, so callers
	 * can store them as-is. Nothing is written until the user edits, so mounting
	 * this never marks a form dirty.
	 */
	let {
		country = $bindable(''),
		code = $bindable(''),
		number = $bindable(''),
		id,
		invalid = false,
		disabled = false,
		autocomplete = 'tel-national',
		contentClass,
		class: className
	}: {
		country?: string;
		code?: string;
		number?: string;
		id?: string;
		invalid?: boolean;
		disabled?: boolean;
		autocomplete?: 'tel-national' | 'off';
		/** Classes for the country list, which renders in a portal (e.g. "dark" to match a dark card). */
		contentClass?: string;
		class?: string;
	} = $props();

	const list = countries();
	let open = $state(false);
	// Settles on a country without writing it back: the stored one, else the
	// first country with the stored dial code, else the browser's region.
	const current = $derived.by(() => {
		if (isCountry(country)) return country;
		const byCode = code ? list.find((c) => c.dial === code) : undefined;
		return byCode?.code ?? defaultCountry();
	});
	const selected = $derived(list.find((c) => c.code === current));
	const display = $derived(formatNational(current, number));

	function commit(nextCountry: string, nextNumber: string) {
		country = nextCountry;
		number = nextNumber;
		code = nextNumber ? dialCode(nextCountry) : '';
	}

	function pick(next: string) {
		open = false;
		commit(next, number);
	}

	function onInput(e: Event & { currentTarget: HTMLInputElement }) {
		const value = e.currentTarget.value;
		let nextCountry: string = current;
		let next = digits(value);

		if (value.trim().startsWith('+')) {
			// A pasted international number picks its own country.
			const parsed = parsePhoneNumberFromString(value);
			if (parsed?.country) {
				nextCountry = parsed.country;
				next = String(parsed.nationalNumber);
			}
		} else if (next === number && value.length < display.length) {
			// Backspace over a formatting space: drop the digit before it.
			next = next.slice(0, -1);
		}

		next = next.slice(0, 15);
		commit(nextCountry, next);
		e.currentTarget.value = formatNational(nextCountry, next);
	}
</script>

<div class={cn('flex gap-2', className)}>
	<Popover.Root bind:open>
		<Popover.Trigger {disabled}>
			{#snippet child({ props })}
				<Button
					{...props}
					variant="outline"
					class="h-9 shrink-0 gap-1.5 px-2.5 font-normal"
					aria-label="Country code: {selected?.name ?? current}"
				>
					<span class="text-base leading-none">{selected?.flag}</span>
					<span class="tabular-nums">{selected?.dial}</span>
					<ChevronsUpDownIcon class="text-muted-foreground size-3.5" />
				</Button>
			{/snippet}
		</Popover.Trigger>
		<Popover.Content class={cn('w-72 p-0', contentClass)} align="start">
			<Command.Root>
				<Command.Input placeholder="Search country or code…" />
				<Command.List>
					<Command.Empty>No country found.</Command.Empty>
					<Command.Group>
						{#each list as c (c.code)}
							<Command.Item
								value="{c.name} {c.dial} {c.code}"
								data-checked={c.code === current}
								onSelect={() => pick(c.code)}
							>
								<span class="text-base leading-none">{c.flag}</span>
								<span class="truncate">{c.name}</span>
								<span class="text-muted-foreground ml-auto tabular-nums">{c.dial}</span>
							</Command.Item>
						{/each}
					</Command.Group>
				</Command.List>
			</Command.Root>
		</Popover.Content>
	</Popover.Root>
	<Input
		{id}
		type="tel"
		inputmode="tel"
		{autocomplete}
		{disabled}
		value={display}
		oninput={onInput}
		aria-invalid={invalid || undefined}
		placeholder={formatNational(current, exampleNumber(current))}
	/>
</div>
