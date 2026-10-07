import { apiClient } from './client';
import type { FeedbackCategory } from './feedback';

/** Someone who runs Fronko itself. Not an organisation user. */
export interface PlatformAdmin {
	id: number;
	email: string;
	last_login_at: string | null;
	created_at: string;
}

/** An organisation's usage: counts and bytes only, never its content or members. */
export interface OrgUsage {
	id: number;
	name: string;
	/** The organisation's part of its card links, /p/{handle}/… */
	handle: string;
	created_at: string;
	owner_email: string | null;
	suspended_at: string | null;
	suspended_reason?: string | null;
	user_count: number;
	admin_count: number;
	member_count: number;
	suspended_user_count: number;
	team_count: number;
	card_count: number;
	lead_count: number;
	file_count: number;
	/** Files uploaded through Fronko; anything else in the bucket isn't counted. */
	storage_used_bytes: number;
	storage_connected: boolean;
	storage_verified: boolean;
	storage_provider: string | null;
	default_quota_bytes: number | null;
	last_active_at: string | null;
	/** Branding, as yes/no settings only. */
	logo_set: boolean;
	logo_policy: 'required' | 'optional';
	signature_locked: boolean;
	/** Files per purpose (logo, brochure, …): counts only. */
	files_by_purpose: Record<string, number>;
}

export interface PlatformSummary {
	org_count: number;
	suspended_org_count: number;
	new_orgs_30d: number;
	user_count: number;
	card_count: number;
	lead_count: number;
	file_count: number;
	orgs_with_storage: number;
	storage_used_bytes: number;
	active_orgs_30d: number;
	new_feedback: number;
	team_count: number;
	orgs_with_teams: number;
	orgs_with_logo: number;
}

/** One day of a trend. The platform-only fields are absent on organisation trends. */
export interface UsagePoint {
	date: string;
	org_count?: number;
	user_count: number;
	card_count: number;
	lead_count: number;
	file_count: number;
	orgs_with_storage?: number;
	storage_used_bytes: number;
	new_orgs?: number;
	feedback_count?: number;
	team_count: number;
	orgs_with_teams?: number;
	orgs_with_logo?: number;
}

export type FeedbackStatus = 'new' | 'read' | 'resolved';

export const FEEDBACK_STATUS_LABEL: Record<FeedbackStatus, string> = { new: 'New', read: 'Read', resolved: 'Resolved' };

export interface FeedbackReply {
	id: number;
	admin_email: string | null;
	body: string;
	email_sent: boolean;
	created_at: string;
}

export interface Feedback {
	id: number;
	org_id: number | null;
	sender_email: string;
	org_name: string;
	category: FeedbackCategory;
	rating: number | null;
	message: string;
	page_path: string | null;
	status: FeedbackStatus;
	reply_count: number;
	replies?: FeedbackReply[];
	created_at: string;
	updated_at: string;
}

export interface AuditEntry {
	id: number;
	admin_email: string;
	action: string;
	target_type: string | null;
	target_id: number | null;
	detail: Record<string, unknown>;
	created_at: string;
}

export interface Page<T> {
	items: T[];
	total: number;
	page: number;
	page_size: number;
}

export type OrgSort = 'newest' | 'oldest' | 'name' | 'users' | 'teams' | 'cards' | 'leads' | 'storage' | 'last_active';
export type OrgStatusFilter = '' | 'active' | 'suspended';

function query(params: Record<string, string | number | undefined>): string {
	const q = new URLSearchParams();
	for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== '') q.set(k, String(v));
	const s = q.toString();
	return s ? `?${s}` : '';
}

export function adminLogin(email: string, password: string): Promise<PlatformAdmin> {
	return apiClient<PlatformAdmin>('/auth/admin/login', { method: 'POST', body: JSON.stringify({ email, password }) });
}

export function adminLogout(): Promise<void> {
	return apiClient<void>('/auth/admin/logout', { method: 'POST' });
}

/** Who is signed in to the admin panel. A 401 just means "nobody". */
export function adminMe(): Promise<PlatformAdmin> {
	return apiClient<PlatformAdmin>('/api/admin/me', {}, { redirectOnUnauthorized: false });
}

export function getSummary(): Promise<PlatformSummary> {
	return apiClient<PlatformSummary>('/api/admin/summary');
}

export function getPlatformTrend(days: number): Promise<{ days: number; points: UsagePoint[] }> {
	return apiClient(`/api/admin/trends${query({ days })}`);
}

export function listOrgs(opts: {
	q?: string;
	status?: OrgStatusFilter;
	sort?: OrgSort;
	page?: number;
	pageSize?: number;
}): Promise<Page<OrgUsage>> {
	return apiClient(
		`/api/admin/orgs${query({ q: opts.q, status: opts.status, sort: opts.sort, page: opts.page, page_size: opts.pageSize })}`
	);
}

export function getOrg(id: number): Promise<OrgUsage> {
	return apiClient<OrgUsage>(`/api/admin/orgs/${id}`);
}

export function getOrgTrend(id: number, days: number): Promise<{ days: number; points: UsagePoint[] }> {
	return apiClient(`/api/admin/orgs/${id}/trends${query({ days })}`);
}

/** Signs everyone in the organisation out, takes its cards offline and emails the owner the reason. */
export function suspendOrg(id: number, reason: string): Promise<OrgUsage> {
	return apiClient<OrgUsage>(`/api/admin/orgs/${id}/suspend`, { method: 'POST', body: JSON.stringify({ reason }) });
}

export function reinstateOrg(id: number): Promise<OrgUsage> {
	return apiClient<OrgUsage>(`/api/admin/orgs/${id}/reinstate`, { method: 'POST' });
}

export function listFeedback(opts: {
	status?: FeedbackStatus | '';
	page?: number;
	pageSize?: number;
}): Promise<Page<Feedback> & { counts: Record<FeedbackStatus, number> }> {
	return apiClient(`/api/admin/feedback${query({ status: opts.status, page: opts.page, page_size: opts.pageSize })}`);
}

export function getFeedback(id: number): Promise<Feedback> {
	return apiClient<Feedback>(`/api/admin/feedback/${id}`);
}

export function setFeedbackStatus(id: number, status: FeedbackStatus): Promise<Feedback> {
	return apiClient<Feedback>(`/api/admin/feedback/${id}`, { method: 'PATCH', body: JSON.stringify({ status }) });
}

/** Emails the reply to whoever sent the feedback and keeps it with the thread. */
export function replyFeedback(id: number, body: string): Promise<Feedback> {
	return apiClient<Feedback>(`/api/admin/feedback/${id}/replies`, { method: 'POST', body: JSON.stringify({ body }) });
}

export function listAudit(opts: { page?: number; pageSize?: number }): Promise<Page<AuditEntry>> {
	return apiClient(`/api/admin/audit${query({ page: opts.page, page_size: opts.pageSize })}`);
}
