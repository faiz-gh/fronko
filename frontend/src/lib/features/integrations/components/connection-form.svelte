<script lang="ts">
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Switch } from '$lib/components/ui/switch';
	import { Textarea } from '$lib/components/ui/textarea';
	import type { FormValues } from '../fields';
	import type { Connection, Field as ManifestField } from '../types';
	import SecretField from './secret-field.svelte';

	/**
	 * The settings form for any provider, rendered from its manifest's fields.
	 * `problems` holds messages by field key, from the browser's checks or
	 * the server.
	 */
	let {
		fields,
		values = $bindable(),
		saved,
		problems = {},
		idPrefix,
		disabled = false
	}: {
		fields: ManifestField[];
		values: FormValues;
		saved?: Connection;
		problems?: Record<string, string>;
		idPrefix: string;
		disabled?: boolean;
	} = $props();

	const fid = (key: string) => `${idPrefix}-${key}`;
</script>

<Field.Group class="gap-5">
	{#each fields as f (f.key)}
		{@const problem = problems[f.key]}
		{#if f.type === 'bool'}
			<Field.Field orientation="horizontal">
				<Field.Content>
					<Field.Label for={fid(f.key)}>{f.label}</Field.Label>
					{#if f.help}<Field.Description>{f.help}</Field.Description>{/if}
				</Field.Content>
				<Switch id={fid(f.key)} bind:checked={values.config[f.key] as boolean} {disabled} />
			</Field.Field>
		{:else}
			<Field.Field data-invalid={problem ? true : undefined}>
				<Field.Label for={fid(f.key)}>
					{f.label}
					{#if !f.required}<span class="text-muted-foreground font-normal">(optional)</span>{/if}
				</Field.Label>
				{#if f.type === 'secret'}
					<SecretField
						field={f}
						id={fid(f.key)}
						bind:value={values.secrets[f.key]}
						saved={saved?.secrets[f.key] ?? false}
						invalid={!!problem}
						{disabled}
					/>
				{:else if f.type === 'textarea'}
					<Textarea
						id={fid(f.key)}
						bind:value={values.config[f.key] as string}
						placeholder={f.placeholder}
						aria-invalid={problem ? true : undefined}
						{disabled}
						rows={4}
						maxlength={f.max_length}
						class="max-h-64 font-mono text-xs"
					/>
				{:else if f.type === 'select'}
					{@const current = f.options?.find((o) => o.value === values.config[f.key])}
					<Select.Root
						type="single"
						value={(values.config[f.key] as string) ?? ''}
						onValueChange={(v) => (values.config[f.key] = v)}
						{disabled}
					>
						<Select.Trigger id={fid(f.key)} class="w-full" aria-invalid={problem ? true : undefined}>
							{current?.label ?? f.placeholder ?? 'Choose…'}
						</Select.Trigger>
						<Select.Content>
							{#each f.options ?? [] as o (o.value)}
								<Select.Item value={o.value} label={o.label}>{o.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{:else}
					<Input
						id={fid(f.key)}
						type={f.type === 'url' ? 'url' : 'text'}
						inputmode={f.type === 'url' ? 'url' : undefined}
						bind:value={values.config[f.key] as string}
						placeholder={f.placeholder}
						autocomplete="off"
						spellcheck={false}
						aria-invalid={problem ? true : undefined}
						class={f.type === 'url' ? 'font-mono text-sm' : undefined}
						{disabled}
					/>
				{/if}
				{#if problem}
					<Field.Error>{problem}</Field.Error>
				{:else if f.help}
					<Field.Description>{f.help}</Field.Description>
				{/if}
			</Field.Field>
		{/if}
	{/each}
</Field.Group>
