import { beforeNavigate, goto } from '$app/navigation';
import { confirmDialog } from './confirm.svelte';

/**
 * Asks before leaving a page with unsaved changes. In-app navigation gets the
 * app's dialog; closing the tab or leaving the site gets the browser's own
 * prompt. `skip` lets a page allow some navigations (such as switching the
 * section in the URL hash).
 */
export function guardUnsaved(dirty: () => boolean, skip?: (to: URL) => boolean) {
	let leaving = false;
	beforeNavigate((nav) => {
		if (leaving || !dirty()) return;
		if (nav.to && skip?.(nav.to.url)) return;
		nav.cancel();
		// The browser asks by itself when the tab closes or the site changes.
		if (nav.type === 'leave' || !nav.to) return;
		const to = nav.to.url;
		confirmDialog({
			title: 'Discard unsaved changes?',
			description: "You have changes that haven't been saved. They'll be lost if you leave this page.",
			confirmLabel: 'Leave without saving',
			cancelLabel: 'Keep editing',
			destructive: true
		}).then((ok) => {
			if (!ok) return;
			leaving = true;
			goto(to).finally(() => (leaving = false));
		});
	});
}
