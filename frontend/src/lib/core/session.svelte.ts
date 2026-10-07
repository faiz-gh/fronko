import { goto } from '$app/navigation';
import { logout, me, type AuthUser, type Role } from '$lib/features/auth/api';
import type { TeamRef } from '$lib/features/teams/api';

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
	role = $state<Role | null>(null);
	orgName = $state('');
	/** The organisation's part of every card link, /p/{orgHandle}/{slug}. */
	orgHandle = $state('');
	mustChangePassword = $state(false);
	/** False for accounts that only sign in with single sign-on. */
	hasPassword = $state(true);
	/** Teams the user is in, with their role in each. */
	teams = $state<TeamRef[]>([]);
	/** Set when the platform suspended the organisation; the login page shows it. */
	suspendedReason = $state<string | null>(null);

	get isAuthenticated() {
		return this.status === 'authenticated';
	}

	/** Owners and admins manage the organisation: users, every card, every lead. */
	get isAdmin() {
		return this.role === 'owner' || this.role === 'admin';
	}

	/** Only the owner manages storage and organisation settings. */
	get isOwner() {
		return this.role === 'owner';
	}

	/** Teams the user leads: they look after those teams' files and see their people's cards and leads. */
	get ledTeams(): TeamRef[] {
		return this.teams.filter((t) => t.role === 'lead');
	}

	get isLead() {
		return this.ledTeams.length > 0;
	}

	/** Admins and team leads see more than their own cards and leads. */
	get seesOthers() {
		return this.isAdmin || this.isLead;
	}

	/** Signed in, verified, and using a password they chose: the dashboard is open. */
	get ready() {
		return this.isAuthenticated && this.emailVerified && !this.mustChangePassword;
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
		this.role = user.role;
		this.orgName = user.org_name;
		this.orgHandle = user.org_handle;
		this.mustChangePassword = user.must_change_password;
		this.hasPassword = user.has_password ?? true;
		this.teams = user.teams ?? [];
		this.status = 'authenticated';
	}

	clear() {
		this.username = null;
		this.email = null;
		this.emailVerified = false;
		this.role = null;
		this.orgName = '';
		this.orgHandle = '';
		this.mustChangePassword = false;
		this.hasPassword = true;
		this.teams = [];
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

	/** Called when the API says the organisation's temporary password must be replaced first. */
	requirePasswordChange() {
		if (this.status !== 'authenticated') return;
		this.mustChangePassword = true;
		if (location.pathname === '/set-password') return;
		const next = location.pathname + location.search;
		goto(`/set-password?next=${encodeURIComponent(next)}`, { replaceState: true });
	}

	/** Where to go right after signing in or verifying: unfinished account setup first. */
	nextStep(next: string): string {
		const q = `?next=${encodeURIComponent(next)}`;
		if (!this.emailVerified) return `/verify-email${q}`;
		if (this.mustChangePassword) return `/set-password${q}`;
		return next;
	}

	/** Called when the API says the organisation is suspended: sign out and explain on the login page. */
	suspend(reason: string) {
		this.suspendedReason = reason;
		if (this.status === 'anonymous') return;
		this.clear();
		if (location.pathname !== '/login') goto('/login', { replaceState: true });
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
