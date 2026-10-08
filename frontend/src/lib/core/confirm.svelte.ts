/** What a confirm dialog asks. */
export interface ConfirmOptions {
	title: string;
	description?: string;
	confirmLabel?: string;
	cancelLabel?: string;
	/** Styles the confirm button as destructive. */
	destructive?: boolean;
}

interface Pending extends ConfirmOptions {
	resolve: (ok: boolean) => void;
}

/** The one confirm dialog, mounted in the root layout. */
class ConfirmState {
	current = $state<Pending | null>(null);

	settle(ok: boolean) {
		this.current?.resolve(ok);
		this.current = null;
	}
}

export const confirmState = new ConfirmState();

/**
 * Asks in an in-app dialog (instead of the browser's confirm()) and resolves
 * to whether the person agreed. Asking again cancels the earlier question.
 */
export function confirmDialog(options: ConfirmOptions): Promise<boolean> {
	confirmState.settle(false);
	return new Promise((resolve) => (confirmState.current = { ...options, resolve }));
}
