import { apiClient } from './client';

/** The session token itself is set as an HttpOnly cookie and never exposed to scripts. */
export interface AuthUser {
	id: number;
	username: string;
}

export function login(username: string, password: string): Promise<AuthUser> {
	return apiClient<AuthUser>('/auth/login', {
		method: 'POST',
		body: JSON.stringify({ username, password })
	});
}

export function register(username: string, password: string): Promise<AuthUser> {
	return apiClient<AuthUser>('/auth/register', {
		method: 'POST',
		body: JSON.stringify({ username, password })
	});
}

export function logout(): Promise<void> {
	return apiClient<void>('/auth/logout', { method: 'POST' });
}

/** Who is signed in. A 401 here just means "nobody", so it must not trigger the expiry redirect. */
export function me(): Promise<AuthUser> {
	return apiClient<AuthUser>('/api/me/user', {}, { redirectOnUnauthorized: false });
}
