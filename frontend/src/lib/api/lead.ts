import { apiClient } from './client';

export interface Lead {
	id: number;
	profile_id: number;
	name: string;
	email: string;
	notes: string;
	created_at: string;
}

export interface LeadPage {
	leads: Lead[];
	total: number;
	page: number;
	page_size: number;
}

export interface LeadQuery {
	/** Only this card's leads. */
	profileId?: number;
	/** Case-insensitive match on name, email or message. */
	q?: string;
	/** Received at or after this time. */
	since?: Date;
	/** 1-based. */
	page?: number;
	/** 1–100; the server defaults to 25. */
	pageSize?: number;
}

/** The server's maximum page size, used when fetching everything (e.g. CSV export). */
export const MAX_LEAD_PAGE_SIZE = 100;

export function submitLead(profileId: number, name: string, email: string, notes: string): Promise<void> {
	return apiClient<void>(`/api/profiles/${profileId}/leads`, {
		method: 'POST',
		body: JSON.stringify({ name, email, notes })
	});
}

/** One page of the signed-in user's leads across all their cards, newest first. */
export function listLeads(query: LeadQuery = {}): Promise<LeadPage> {
	const params = new URLSearchParams();
	if (query.profileId) params.set('profile_id', String(query.profileId));
	if (query.q?.trim()) params.set('q', query.q.trim());
	if (query.since) params.set('since', query.since.toISOString());
	if (query.page) params.set('page', String(query.page));
	if (query.pageSize) params.set('page_size', String(query.pageSize));
	const qs = params.toString();
	return apiClient<LeadPage>(`/api/me/leads${qs ? `?${qs}` : ''}`);
}

/** Every lead matching the query, fetched page by page. */
export async function listAllLeads(query: Omit<LeadQuery, 'page' | 'pageSize'> = {}): Promise<Lead[]> {
	const all: Lead[] = [];
	for (let page = 1; ; page++) {
		const res = await listLeads({ ...query, page, pageSize: MAX_LEAD_PAGE_SIZE });
		all.push(...res.leads);
		if (all.length >= res.total || res.leads.length === 0) return all;
	}
}
