import { goto } from '$app/navigation';
import { logout, me, type AuthUser } from '$lib/api/auth';

type Status = 'unknown' | 'authenticated' | 'anonymous';

/**
 * Client-side view of the login session. The JWT itself lives in an HttpOnly
 * cookie that scripts can't read, so we learn who is signed in by asking the API.
 */
class Session {
	status = $state<Status>('unknown');
	username = $state<string | null>(null);
	email = $state<string | null>(null);
	emailVerified = $state(false);

	get isAuthenticated() {
		return this.status === 'authenticated';
	}

	#loading: Promise<void> | null = null;

	/** Resolves the session once per page load; safe to call repeatedly. */
	load(): Promise<void> {
		this.#loading ??= (async () => {
			// Tokens used to be stored here; drop them from older installs.
			try {
				localStorage.removeItem('jwt_token');
				localStorage.removeItem('username');
			} catch {
				// Storage unavailable; nothing to clean up.
			}
			try {
				this.signIn(await me());
			} catch {
				this.clear();
			}
		})();
		return this.#loading;
	}

	signIn(user: AuthUser) {
		this.username = user.username;
		this.email = user.email;
		this.emailVerified = user.email_verified;
		this.status = 'authenticated';
	}

	clear() {
		this.username = null;
		this.email = null;
		this.emailVerified = false;
		this.status = 'anonymous';
	}

	async signOut() {
		try {
			await logout();
		} finally {
			this.clear();
			await goto('/login');
		}
	}

	/** Called when the API says the email must be verified before going further. */
	requireVerification() {
		if (this.status !== 'authenticated') return;
		this.emailVerified = false;
		if (location.pathname === '/verify-email') return;
		const next = location.pathname + location.search;
		goto(`/verify-email?next=${encodeURIComponent(next)}`, { replaceState: true });
	}

	/** Called when the API rejects the cookie; returns the user to where they were after login. */
	expire() {
		if (this.status === 'anonymous') return;
		this.clear();
		const next = location.pathname + location.search;
		goto(`/login?next=${encodeURIComponent(next)}&expired=1`);
	}
}

export const session = new Session();
