import { goto } from '$app/navigation';
import { adminLogout, adminMe, type PlatformAdmin } from './api';

type Status = 'unknown' | 'authenticated' | 'anonymous';

/**
 * The platform admin panel's session. It's separate from the user session:
 * its own HttpOnly cookie, its own sign-in page, and none of the org state.
 */
class AdminSession {
	status = $state<Status>('unknown');
	admin = $state<PlatformAdmin | null>(null);

	get isAuthenticated() {
		return this.status === 'authenticated';
	}

	#loading: Promise<void> | null = null;

	/** Resolves the session once per page load; safe to call repeatedly. */
	load(): Promise<void> {
		this.#loading ??= (async () => {
			try {
				this.signIn(await adminMe());
			} catch {
				this.clear();
			}
		})();
		return this.#loading;
	}

	signIn(admin: PlatformAdmin) {
		this.admin = admin;
		this.status = 'authenticated';
	}

	clear() {
		this.admin = null;
		this.status = 'anonymous';
	}

	async signOut() {
		try {
			await adminLogout();
		} finally {
			this.clear();
			await goto('/admin/login');
		}
	}

	/** Called when the API rejects the admin cookie. */
	expire() {
		if (this.status === 'anonymous') return;
		this.clear();
		const next = location.pathname + location.search;
		goto(`/admin/login?next=${encodeURIComponent(next)}&expired=1`);
	}
}

export const adminSession = new AdminSession();
