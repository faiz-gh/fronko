import { adminSession } from '$lib/features/admin/session.svelte';
import { session } from './session.svelte';

export class ApiError extends Error {
	constructor(
		message: string,
		readonly status: number,
		/** Machine-readable reason, when the backend sends one (e.g. "email_unverified"). */
		readonly code?: string,
		/** Seconds from a 429's Retry-After header. */
		readonly retryAfter?: number,
		/** Why the organisation was suspended, on an "org_suspended" error. */
		readonly reason?: string
	) {
		super(message);
	}
}

interface ClientOptions {
	/** Send the user to /login when a protected call returns 401. Defaults to true. */
	redirectOnUnauthorized?: boolean;
}

/**
 * Public backend URL from the runtime config. Empty (the default) means
 * same-origin: in dev Vite proxies /api and /auth, and in Docker nginx does.
 * When set, the backend must list this site in CORS_ALLOWED_ORIGINS.
 */
export const API_URL = (window.__FRONKO_CONFIG__?.apiUrl ?? '').replace(/\/+$/, '');

/** Absolute or relative URL for a backend path such as `/api/files/x`. */
export function apiUrl(path: string): string {
	return API_URL + path;
}

/**
 * Thin fetch wrapper for the Go backend. Credentials are included so the
 * HttpOnly session cookie is sent even when the API is on another origin.
 */
export async function apiClient<T>(
	endpoint: string,
	options: RequestInit = {},
	{ redirectOnUnauthorized = true }: ClientOptions = {}
): Promise<T> {
	const headers = new Headers(options.headers);
	// JSON bodies are strings; FormData sets its own multipart boundary.
	if (typeof options.body === 'string') headers.set('Content-Type', 'application/json');

	let response: Response;
	try {
		response = await fetch(apiUrl(endpoint), { credentials: 'include', ...options, headers });
	} catch {
		throw new ApiError('Could not reach the server. Check your connection and try again.', 0);
	}

	if (!response.ok) {
		const body = await response.json().catch(() => ({}));
		// The platform suspended the whole organisation: sign out and say why.
		if (response.status === 401 && body.code === 'org_suspended') {
			session.suspend(body.reason ?? '');
		}
		// An expired or invalid session on a protected route: sign out and send to login.
		else if (response.status === 401 && redirectOnUnauthorized && /^\/api\/(me|org)\b/.test(endpoint)) {
			session.expire();
		}
		// The platform admin panel has its own session and sign-in page.
		else if (response.status === 401 && redirectOnUnauthorized && /^\/api\/admin\b/.test(endpoint)) {
			adminSession.expire();
		}
		// Signed in but the email isn't verified yet: finish that first.
		if (response.status === 403 && body.code === 'email_unverified') {
			session.requireVerification();
		}
		// Still on the organisation's temporary password: choose one first.
		if (response.status === 403 && body.code === 'password_change_required') {
			session.requirePasswordChange();
		}
		const retryAfter = Number(response.headers.get('Retry-After')) || undefined;
		throw new ApiError(
			body.error || `Request failed (${response.status})`,
			response.status,
			body.code,
			retryAfter,
			body.reason
		);
	}

	if (response.status === 204) {
		return undefined as T;
	}

	return response.json() as Promise<T>;
}
