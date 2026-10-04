import { apiClient } from './client';

/** The session token itself is set as an HttpOnly cookie and never exposed to scripts. */
export interface AuthUser {
	id: number;
	username: string;
	/** Null only for accounts created before emails were required. */
	email: string | null;
	email_verified: boolean;
}

/** `identifier` is a username or an email address. */
export function login(identifier: string, password: string): Promise<AuthUser> {
	return apiClient<AuthUser>('/auth/login', {
		method: 'POST',
		body: JSON.stringify({ username: identifier, password })
	});
}

export function register(username: string, email: string, password: string): Promise<AuthUser> {
	return apiClient<AuthUser>('/auth/register', {
		method: 'POST',
		body: JSON.stringify({ username, email, password })
	});
}

export function logout(): Promise<void> {
	return apiClient<void>('/auth/logout', { method: 'POST' });
}

/** Who is signed in. A 401 here just means "nobody", so it must not trigger the expiry redirect. */
export function me(): Promise<AuthUser> {
	return apiClient<AuthUser>('/api/me/user', {}, { redirectOnUnauthorized: false });
}

/** Emails a reset code if a verified account uses this address. Succeeds either way. */
export function forgotPassword(email: string): Promise<void> {
	return apiClient<void>('/auth/password/forgot', { method: 'POST', body: JSON.stringify({ email }) });
}

/** Sets a new password and signs out every session; sign in afterwards. */
export function resetPassword(email: string, code: string, password: string): Promise<void> {
	return apiClient<void>('/auth/password/reset', {
		method: 'POST',
		body: JSON.stringify({ email, code, password })
	});
}

/** Adds or corrects the (still unverified) email and sends a code to it. */
export function setEmail(email: string): Promise<AuthUser> {
	return apiClient<AuthUser>('/api/me/email', { method: 'PUT', body: JSON.stringify({ email }) });
}

export function verifyEmail(code: string): Promise<AuthUser> {
	return apiClient<AuthUser>('/api/me/email/verify', { method: 'POST', body: JSON.stringify({ code }) });
}

export function resendVerification(): Promise<void> {
	return apiClient<void>('/api/me/email/resend', { method: 'POST' });
}

/** Signs out every other session; this one stays signed in. */
export function changePassword(currentPassword: string, newPassword: string): Promise<void> {
	return apiClient<void>('/api/me/password', {
		method: 'PUT',
		body: JSON.stringify({ current_password: currentPassword, new_password: newPassword })
	});
}

/** Starts changing a verified email: checks the password and sends a code to the new address. */
export function requestEmailChange(email: string, password: string): Promise<void> {
	return apiClient<void>('/api/me/email/change', { method: 'POST', body: JSON.stringify({ email, password }) });
}

/** Confirms the new address with its code; the old one is notified. */
export function confirmEmailChange(code: string): Promise<AuthUser> {
	return apiClient<AuthUser>('/api/me/email/change/confirm', { method: 'POST', body: JSON.stringify({ code }) });
}
