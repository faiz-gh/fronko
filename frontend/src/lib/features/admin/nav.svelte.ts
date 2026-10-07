import { getSummary } from './api';

/** Badge counts for the admin sidebar, refreshed when pages change what they count. */
class AdminNav {
	newFeedback = $state(0);

	async refresh() {
		try {
			this.newFeedback = (await getSummary()).new_feedback;
		} catch {
			// The badge is a convenience; pages show their own errors.
		}
	}
}

export const adminNav = new AdminNav();
