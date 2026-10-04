import { apiClient } from './client';

export interface Lead {
	id: number;
	profile_id: number;
	name: string;
	email: string;
	/** Dial code ("+91"); absent when the visitor left no number. */
	phone_country_code?: string;
	/** National number, digits only. */
	phone_number?: string;
	notes: string;
	created_at: string;
	/** Who held the card when the lead arrived; null means the organisation did. */
	assigned_user: { id: number; username: string } | null;
}

export interface NewLead {
	name: string;
	email: string;
	/** Both phone parts are set together, or both left empty. */
	phone_country_code: string;
	phone_number: string;
	notes: string;
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
	/** Admins only: leads that arrived while this user held the card, or 'none' for the organisation's. */
	userId?: number | 'none';
	/** Case-insensitive match on name, email, phone or message. */
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

export function submitLead(profileId: number, lead: NewLead): Promise<void> {
	return apiClient<void>(`/api/profiles/${profileId}/leads`, {
		method: 'POST',
		body: JSON.stringify(lead)
	});
}

/** One page of the leads the signed-in user can see (the whole organisation's for admins), newest first. */
export function listLeads(query: LeadQuery = {}): Promise<LeadPage> {
	const params = new URLSearchParams();
	if (query.profileId) params.set('profile_id', String(query.profileId));
	if (query.userId) params.set('user_id', String(query.userId));
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
