import { session } from '$lib/session.svelte';

export class ApiError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
	}
}

interface ClientOptions {
	/** Send the user to /login when a protected call returns 401. Defaults to true. */
	redirectOnUnauthorized?: boolean;
}

/**
 * Thin fetch wrapper for the Go backend. Requests use relative paths: in dev
 * Vite proxies /api and /auth, and in Docker nginx does, so it's always
 * same-origin and the HttpOnly session cookie is sent automatically.
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
		response = await fetch(endpoint, { ...options, headers });
	} catch {
		throw new ApiError('Could not reach the server. Check your connection and try again.', 0);
	}

	if (!response.ok) {
		// An expired or invalid session on a protected route: sign out and send to login.
		if (response.status === 401 && redirectOnUnauthorized && endpoint.startsWith('/api/me/')) {
			session.expire();
		}
		const body = await response.json().catch(() => ({}));
		throw new ApiError(body.error || `Request failed (${response.status})`, response.status);
	}

	if (response.status === 204) {
		return undefined as T;
	}

	return response.json() as Promise<T>;
}
