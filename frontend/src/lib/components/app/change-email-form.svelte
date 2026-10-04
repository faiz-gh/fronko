<script lang="ts">
	import { toast } from 'svelte-sonner';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import { ApiError } from '$lib/api/client';
	import { confirmEmailChange, requestEmailChange } from '$lib/api/auth';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import CodeInput from '$lib/components/app/code-input.svelte';
	import { Cooldown, RESEND_COOLDOWN_SECONDS } from '$lib/cooldown.svelte';
	import { session } from '$lib/session.svelte';

	let { onclose }: { onclose: () => void } = $props();

	let step = $state<'details' | 'code'>('details');
	let email = $state('');
	let password = $state('');
	let code = $state('');
	let error = $state('');
	let busy = $state<'send' | 'confirm' | null>(null);
	const resend = new Cooldown();
	$effect(() => () => resend.stop());

	const target = $derived(email.trim().toLowerCase());

	async function send(event?: SubmitEvent) {
		event?.preventDefault();
		error = '';
		busy = 'send';
		try {
			await requestEmailChange(target, password);
			step = 'code';
			code = '';
			resend.start(RESEND_COOLDOWN_SECONDS);
		} catch (e) {
			// A code to this address went out moments ago and is still valid.
			if (e instanceof ApiError && e.status === 429 && e.retryAfter) {
				step = 'code';
				resend.start(e.retryAfter);
			} else {
				error = e instanceof Error ? e.message : 'Could not send the code';
			}
		} finally {
			busy = null;
		}
	}

	async function confirm(value = code) {
		if (value.length !== 6 || busy) return;
		error = '';
		busy = 'confirm';
		try {
			session.signIn(await confirmEmailChange(value));
			toast.success(`Email changed to ${session.email}`);
			onclose();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Could not confirm the code';
			code = '';
		} finally {
			busy = null;
		}
	}
</script>

<div class="bg-muted/30 flex flex-col gap-5 rounded-xl border p-4 sm:p-5">
	{#if error}
		<Alert.Root variant="destructive">
			<CircleAlertIcon />
			<Alert.Title>{error}</Alert.Title>
		</Alert.Root>
	{/if}

	{#if step === 'details'}
		<form onsubmit={send} class="flex flex-col gap-5">
			<Field.Group class="grid gap-5 sm:grid-cols-2">
				<Field.Field>
					<Field.Label for="new-email">New email</Field.Label>
					<Input
						id="new-email"
						type="email"
						autocomplete="email"
						autocapitalize="none"
						spellcheck={false}
						bind:value={email}
						disabled={!!busy}
					/>
				</Field.Field>
				<Field.Field>
					<Field.Label for="email-password">Current password</Field.Label>
					<Input
						id="email-password"
						type="password"
						autocomplete="current-password"
						bind:value={password}
						disabled={!!busy}
					/>
				</Field.Field>
				<Field.Description class="sm:col-span-2">
					We'll send a code to the new address. Your current email keeps working until you confirm it.
				</Field.Description>
			</Field.Group>
			<div class="flex flex-wrap gap-2">
				<Button type="submit" disabled={!!busy || !target || !password}>
					{#if busy === 'send'}<Spinner data-icon="inline-start" />{/if}
					Send code
				</Button>
				<Button type="button" variant="ghost" onclick={onclose} disabled={!!busy}>Cancel</Button>
			</div>
		</form>
	{:else}
		<form
			onsubmit={(e) => {
				e.preventDefault();
				confirm();
			}}
			class="flex flex-col gap-5"
		>
			<Field.Field>
				<Field.Label for="email-code">Code sent to <span class="break-all">{target}</span></Field.Label>
				<div class="max-w-xs">
					<CodeInput id="email-code" bind:value={code} disabled={!!busy} invalid={!!error} oncomplete={confirm} />
				</div>
				<Field.Description>
					Didn't get it? Check spam, or
					<button
						type="button"
						class="text-foreground font-medium underline-offset-4 hover:underline disabled:no-underline disabled:opacity-60"
						onclick={() => send()}
						disabled={!!busy || resend.active}
					>
						{resend.active ? `resend in ${resend.remaining}s` : 'send a new code'}</button
					>.
				</Field.Description>
			</Field.Field>
			<div class="flex flex-wrap gap-2">
				<Button type="submit" disabled={!!busy || code.length !== 6}>
					{#if busy === 'confirm'}<Spinner data-icon="inline-start" />{/if}
					Confirm new email
				</Button>
				<Button
					type="button"
					variant="ghost"
					onclick={() => {
						step = 'details';
						error = '';
						resend.stop();
					}}
					disabled={!!busy}
				>
					Use a different address
				</Button>
				<Button type="button" variant="ghost" onclick={onclose} disabled={!!busy}>Cancel</Button>
			</div>
		</form>
	{/if}
</div>
