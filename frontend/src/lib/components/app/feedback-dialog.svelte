<script lang="ts">
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import StarIcon from '@lucide/svelte/icons/star';
	import { CATEGORY_LABEL, sendFeedback, type FeedbackCategory } from '$lib/api/feedback';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Field from '$lib/components/ui/field';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import { cn } from '$lib/utils';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const MAX_LENGTH = 5000;
	const RATING_LABEL = ['', 'Very poor', 'Poor', 'Okay', 'Good', 'Excellent'];

	let category = $state<FeedbackCategory>('idea');
	let rating = $state<number | null>(null);
	let hovered = $state<number | null>(null);
	let message = $state('');
	let sending = $state(false);
	let error = $state('');

	const shown = $derived(hovered ?? rating ?? 0);
	const PLACEHOLDER: Record<FeedbackCategory, string> = {
		bug: 'What happened, and what did you expect to happen?',
		idea: 'What would make Fronko more useful for you?',
		other: 'Tell us anything.'
	};

	function reset() {
		category = 'idea';
		rating = null;
		hovered = null;
		message = '';
		error = '';
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (!message.trim()) return;
		sending = true;
		error = '';
		try {
			await sendFeedback({ category, rating, message: message.trim(), page_path: page.url.pathname });
			toast.success('Thanks! Your feedback was sent.');
			open = false;
			reset();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to send feedback';
		} finally {
			sending = false;
		}
	}
</script>

<Dialog.Root bind:open onOpenChange={(o) => !o && reset()}>
	<Dialog.Content class="sm:max-w-lg">
		<form onsubmit={handleSubmit} class="flex flex-col gap-6">
			<Dialog.Header>
				<Dialog.Title class="text-lg">Send feedback</Dialog.Title>
				<Dialog.Description>
					Tell the Fronko team what's working and what isn't. We'll see your email and organisation, so we can reply.
				</Dialog.Description>
			</Dialog.Header>
			<Field.Group>
				<Field.Field>
					<Field.Label id="feedback-category-label">Type</Field.Label>
					<ToggleGroup.Root
						type="single"
						variant="outline"
						value={category}
						onValueChange={(v) => v && (category = v as FeedbackCategory)}
						aria-labelledby="feedback-category-label"
						class="w-full"
					>
						{#each Object.entries(CATEGORY_LABEL) as [value, label] (value)}
							<ToggleGroup.Item {value} class="flex-1">{label}</ToggleGroup.Item>
						{/each}
					</ToggleGroup.Root>
				</Field.Field>
				<Field.Field>
					<Field.Label id="feedback-rating-label">
						How's Fronko overall? <span class="text-muted-foreground font-normal">(optional)</span>
					</Field.Label>
					<div class="flex items-center gap-3">
						<div class="flex items-center" role="radiogroup" aria-labelledby="feedback-rating-label">
							{#each [1, 2, 3, 4, 5] as n (n)}
								<button
									type="button"
									role="radio"
									aria-checked={rating === n}
									aria-label="{n} of 5: {RATING_LABEL[n]}"
									class="focus-visible:ring-ring/50 rounded-md p-1 outline-none focus-visible:ring-3"
									onclick={() => (rating = rating === n ? null : n)}
									onpointerenter={() => (hovered = n)}
									onpointerleave={() => (hovered = null)}
								>
									<StarIcon
										class={cn(
											'size-6 transition-colors',
											n <= shown ? 'fill-amber-400 text-amber-400' : 'text-muted-foreground/50'
										)}
									/>
								</button>
							{/each}
						</div>
						<span class="text-muted-foreground text-sm" aria-live="polite">{RATING_LABEL[shown]}</span>
					</div>
				</Field.Field>
				<Field.Field>
					<Field.Label for="feedback-message">Message</Field.Label>
					<Textarea
						id="feedback-message"
						bind:value={message}
						rows={5}
						maxlength={MAX_LENGTH}
						placeholder={PLACEHOLDER[category]}
						required
					/>
					{#if message.length > MAX_LENGTH * 0.9}
						<Field.Description class="tabular">{message.length} / {MAX_LENGTH}</Field.Description>
					{/if}
				</Field.Field>
				{#if error}
					<Field.Error>{error}</Field.Error>
				{/if}
			</Field.Group>
			<Dialog.Footer>
				<Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close>
				<Button type="submit" disabled={sending || !message.trim()}>
					{#if sending}<Spinner data-icon="inline-start" />{/if}
					Send feedback
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
