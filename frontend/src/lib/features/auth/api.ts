import { apiClient, apiUrl } from '$lib/core/api';
import type { TeamRef } from '$lib/features/teams/api';

/** owner registered the organisation; admins help run it; members work on the cards assigned to them. */
export type Role = 'owner' | 'admin' | 'member';

/** The session token itself is set as an HttpOnly cookie and never exposed to scripts. */
export interface AuthUser {
	id: number;
	username: string;
	/** Null only for accounts created before emails were required. */
	email: string | null;
	email_verified: boolean;
	role: Role;
	org_name: string;
	/** The organisation's part of every card link, /p/{org_handle}/{slug}. */
	org_handle: string;
	/** Still using the temporary password the organisation set. */
	must_change_password: boolean;
	/** False for accounts that only sign in with single sign-on. */
	has_password?: boolean;
	/** Teams they're in, with their role in each. */
	teams?: TeamRef[];
}

/** `identifier` is a username or an email address. */
export function login(identifier: string, password: string): Promise<AuthUser> {
	return apiClient<AuthUser>('/auth/login', {
		method: 'POST',
		body: JSON.stringify({ username: identifier, password })
	});
}

/**
 * Finds where to sign in with single sign-on, from a work email on a
 * verified domain or an organisation's handle. `url` is a backend path.
 */
export function discoverSSO(identifier: string): Promise<{ url: string; handle: string }> {
	return apiClient('/auth/sso/discover', { method: 'POST', body: JSON.stringify({ identifier }) });
}

/** The address that starts single sign-on; the browser navigates there. */
export function ssoStartUrl(path: string, returnTo?: string): string {
	const q = returnTo && returnTo !== '/dashboard' ? `?${new URLSearchParams({ return_to: returnTo })}` : '';
	return apiUrl(path + q);
}

/** Creates an organisation (named `organization`, or after the username) with this account as its owner. */
export function register(username: string, email: string, password: string, organization = ''): Promise<AuthUser> {
	return apiClient<AuthUser>('/auth/register', {
		method: 'POST',
		body: JSON.stringify({ username, email, password, organization })
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

/** Signs out every other session; this one stays signed in. Also replaces an organisation-set temporary password. */
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

/** What deleting the account would do, shown before anything happens. */
export interface DeletionInfo {
	username: string;
	role: Role;
	has_password: boolean;
	/** Managed by the organisation's identity provider (SCIM): removed there, not here. */
	managed: boolean;
	org_name: string;
	org_handle: string;
	/** Who a member's or admin's cards and files pass to. */
	owner_username?: string;
	/** The owner is the only person in the organisation. */
	sole_member: boolean;
	summary: {
		cards_held: number;
		cards_made: number;
		files: number;
		personal_files: number;
		leads: number;
		org_members: number;
		org_cards: number;
		org_leads: number;
		org_files: number;
		org_teams: number;
	};
	/** Who an owner can hand the organisation to. */
	transfer_candidates: { id: number; username: string }[];
}

export function getDeletionInfo(): Promise<DeletionInfo> {
	return apiClient<DeletionInfo>('/api/me/account/deletion');
}

/** Emails a code that confirms deleting (or handing over) an account with no password. */
export function sendDeletionCode(): Promise<void> {
	return apiClient<void>('/api/me/account/deletion/code', { method: 'POST' });
}

/** Re-authentication for destructive account actions: the password, or an emailed code. */
export interface Reauth {
	password?: string;
	code?: string;
}

/** Makes someone else the owner; the caller becomes an admin. Returns the caller's updated account. */
export function transferOwnership(userId: number, reauth: Reauth): Promise<AuthUser> {
	return apiClient<AuthUser>('/api/me/ownership/transfer', {
		method: 'POST',
		body: JSON.stringify({ user_id: userId, ...reauth })
	});
}

/**
 * Permanently deletes the signed-in account (and, for an owner with
 * `deleteOrg`, the whole organisation). `confirm` is the username, or the
 * organisation's handle when deleting the organisation.
 */
export function deleteAccount(confirm: string, reauth: Reauth, deleteOrg = false): Promise<void> {
	return apiClient<void>(
		'/api/me/account',
		{ method: 'DELETE', body: JSON.stringify({ confirm, delete_org: deleteOrg, ...reauth }) },
		{ redirectOnUnauthorized: false }
	);
}

/** Everything Fronko holds about the signed-in person, as a JSON document. */
export function exportMyData(): Promise<unknown> {
	return apiClient<unknown>('/api/me/export');
}
