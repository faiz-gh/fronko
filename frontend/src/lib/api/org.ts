import { apiClient } from './client';
import type { Role } from './auth';
import type { Profile } from './profile';

export interface Organization {
	id: number;
	name: string;
	/** Storage limit new users start with; null is unlimited. */
	default_quota_bytes: number | null;
	created_at: string;
	updated_at: string;
}

/** Someone in the organisation, as the Users page lists them. */
export interface OrgUser {
	id: number;
	role: Role;
	username: string;
	email: string | null;
	email_verified_at: string | null;
	/** Still on the temporary password the organisation set. */
	must_change_password: boolean;
	/** Limit on their personal files; null is unlimited. */
	storage_quota_bytes: number | null;
	suspended_at: string | null;
	last_login_at: string | null;
	created_at: string;
	card_count: number;
	/** Leads that arrived while they held a card. */
	lead_count: number;
	/** Size of their personal files. */
	used_bytes: number;
}

/** Where a user is with their account, most urgent first. */
export type UserStatus = 'suspended' | 'unverified' | 'temporary_password' | 'active';

export function userStatus(u: OrgUser): UserStatus {
	if (u.suspended_at) return 'suspended';
	if (!u.email_verified_at) return 'unverified';
	if (u.must_change_password) return 'temporary_password';
	return 'active';
}

export const STATUS_LABEL: Record<UserStatus, string> = {
	suspended: 'Suspended',
	unverified: 'Hasn’t verified email',
	temporary_password: 'Hasn’t set a password',
	active: 'Active'
};

export const ROLE_LABEL: Record<Role, string> = { owner: 'Owner', admin: 'Admin', member: 'Member' };

export function getOrganization(): Promise<Organization> {
	return apiClient<Organization>('/api/org');
}

export function updateOrganization(name: string, defaultQuotaBytes: number | null): Promise<Organization> {
	return apiClient<Organization>('/api/org', {
		method: 'PUT',
		body: JSON.stringify({ name, default_quota_bytes: defaultQuotaBytes })
	});
}

export function listOrgUsers(): Promise<OrgUser[]> {
	return apiClient<OrgUser[]>('/api/org/users');
}

export function getOrgUser(id: number): Promise<OrgUser> {
	return apiClient<OrgUser>(`/api/org/users/${id}`);
}

export interface NewOrgUser {
	username: string;
	email: string;
	/** Temporary; they replace it after verifying their email. */
	password: string;
	role: 'member' | 'admin';
	/** Omit to use the organisation's default; null is unlimited. */
	quota_bytes?: number | null;
}

export function createOrgUser(user: NewOrgUser): Promise<OrgUser> {
	return apiClient<OrgUser>('/api/org/users', { method: 'POST', body: JSON.stringify(user) });
}

export interface OrgUserPatch {
	/** Only while unverified; a new code goes to the new address. */
	email?: string;
	quota_bytes?: number | null;
	/** Owner only. */
	role?: 'member' | 'admin';
	suspended?: boolean;
}

export function updateOrgUser(id: number, patch: OrgUserPatch): Promise<OrgUser> {
	return apiClient<OrgUser>(`/api/org/users/${id}`, { method: 'PATCH', body: JSON.stringify(patch) });
}

/** Gives the user a new temporary password and signs them out everywhere. */
export function resetOrgUserPassword(id: number, password: string): Promise<OrgUser> {
	return apiClient<OrgUser>(`/api/org/users/${id}/password`, { method: 'POST', body: JSON.stringify({ password }) });
}

/** Their cards return to the organisation and their files move to the organisation's files. */
export function deleteOrgUser(id: number): Promise<void> {
	return apiClient<void>(`/api/org/users/${id}`, { method: 'DELETE' });
}

/** Hands a card to a user, or back to the organisation with null. */
export function setCardAssignee(profileId: number, userId: number | null): Promise<Profile> {
	return apiClient<Profile>(`/api/org/profiles/${profileId}/assignee`, {
		method: 'PUT',
		body: JSON.stringify({ user_id: userId })
	});
}

export interface UserRef {
	id: number;
	username: string;
}

export function getFileGrants(fileId: string): Promise<{ users: UserRef[] }> {
	return apiClient<{ users: UserRef[] }>(`/api/org/files/${encodeURIComponent(fileId)}/grants`);
}

export function setFileGrants(fileId: string, userIds: number[]): Promise<{ users: UserRef[] }> {
	return apiClient<{ users: UserRef[] }>(`/api/org/files/${encodeURIComponent(fileId)}/grants`, {
		method: 'PUT',
		body: JSON.stringify({ user_ids: userIds })
	});
}

/** 'required': every card and signature shows the logo; 'optional': each card chooses. */
export type LogoPolicy = 'required' | 'optional';

export interface OrgSignatureSettings {
	/** When set, the only signature template employees can use. */
	locked_template?: string;
	/** #rrggbb; when set, replaces each card's accent in signatures. */
	brand_color?: string;
	/** Shown in small print under every signature. */
	disclaimer?: string;
	/** An org image under every signature, linking to banner_url. */
	banner_file?: string;
	banner_url?: string;
}

/** How the organisation appears on its cards and email signatures. */
export interface OrgBranding {
	name: string;
	/** Public file id of an org image; null when there's no logo. */
	logo_file: string | null;
	logo_policy: LogoPolicy;
	signature: OrgSignatureSettings;
}

/** Anyone in the organisation can read it. */
export function getBranding(): Promise<OrgBranding> {
	return apiClient<OrgBranding>('/api/org/branding');
}

/** Owners and admins. */
export function updateBranding(b: Omit<OrgBranding, 'name'>): Promise<OrgBranding> {
	return apiClient<OrgBranding>('/api/org/branding', { method: 'PUT', body: JSON.stringify(b) });
}

/** Whether a card (or signature) shows the organisation's logo, given its own choice. */
export function showsOrgLogo(
	org: Pick<OrgBranding, 'logo_file' | 'logo_policy'> | null | undefined,
	choice: boolean
): boolean {
	if (!org?.logo_file) return false;
	return org.logo_policy === 'required' || choice;
}
