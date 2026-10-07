import { apiClient } from './client';
import type { VisitSource } from '$lib/analytics/track';

/** Headline numbers for a period. */
export interface AnalyticsTotals {
	views: number;
	/** Distinct visitors per day, summed (visitor hashes rotate daily). */
	unique_visitors: number;
	sessions: number;
	nfc_views: number;
	qr_views: number;
	link_views: number;
	saves: number;
	form_opens: number;
	leads: number;
	doc_opens: number;
	clicks: number;
	gallery_opens: number;
	shares: number;
	/** Visits that clicked, saved, opened something, shared or scrolled half way. */
	engaged_sessions: number;
	/** Visits that saved the contact or opened the contact form. */
	action_sessions: number;
	/** Visitors who came back the same day. */
	repeat_visitors: number;
	avg_scroll_depth: number;
	median_time_ms: number;
}

export interface AnalyticsSummary {
	from: string;
	to: string;
	current: AnalyticsTotals;
	previous: AnalyticsTotals;
	devices: Partial<Record<'mobile' | 'tablet' | 'desktop', number>>;
	lead_sources: Partial<Record<VisitSource | 'unknown', number>>;
	/** Views by weekday (0 = Sunday) and hour. */
	heatmap: number[][];
	/** Visits by deepest scroll: <25, 25, 50, 75, 100 %. */
	scroll_depths: number[];
	/** Visits by time on card: <10s, 10–30s, 30s–1m, 1–3m, 3m+. */
	time_buckets: number[];
}

export interface AnalyticsPoint {
	date: string;
	views: number;
	nfc_views: number;
	qr_views: number;
	link_views: number;
	unique_visitors: number;
	saves: number;
	leads: number;
}

export interface ContentStat {
	type: 'click' | 'doc_open' | 'gallery_open';
	/** A link's URL, a quick action (email, call, website, booking), or a file id. */
	target: string;
	label: string;
	count: number;
	unique: number;
}

export interface UserRef {
	id: number;
	username: string;
}

export interface CardStat {
	profile_id: number;
	slug: string;
	name: string;
	assigned_user: UserRef | null;
	views: number;
	unique_visitors: number;
	nfc_views: number;
	qr_views: number;
	link_views: number;
	saves: number;
	form_opens: number;
	leads: number;
	doc_opens: number;
	/** The card's latest view ever; null if never viewed. */
	last_viewed_at: string | null;
}

export interface TeamStat {
	id: number;
	name: string;
	color: string;
	members: number;
	cards: number;
	active_cards: number;
	views: number;
	unique_visitors: number;
	sessions: number;
	engaged_sessions: number;
	saves: number;
	form_opens: number;
	leads: number;
	prev_views: number;
	prev_saves: number;
	prev_leads: number;
}

export interface MemberStat {
	user_id: number;
	username: string;
	cards: number;
	views: number;
	unique_visitors: number;
	saves: number;
	form_opens: number;
	doc_opens: number;
	leads: number;
}

export interface ActivityItem {
	type: 'view' | 'vcard' | 'form_submit' | 'doc_open';
	source: VisitSource;
	label: string;
	profile_id: number;
	card_name: string;
	slug: string;
	assigned_user: UserRef | null;
	created_at: string;
}

export interface AnalyticsQuery {
	from?: Date;
	to?: Date;
	profileId?: number;
	userId?: number;
	teamId?: number;
}

function qs(q: AnalyticsQuery, extra: Record<string, string> = {}): string {
	const p = new URLSearchParams(extra);
	if (q.from) p.set('from', q.from.toISOString());
	if (q.to) p.set('to', q.to.toISOString());
	if (q.profileId) p.set('profile_id', String(q.profileId));
	if (q.userId) p.set('user_id', String(q.userId));
	if (q.teamId) p.set('team_id', String(q.teamId));
	p.set('tz', Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC');
	return p.toString();
}

export function getAnalyticsSummary(q: AnalyticsQuery = {}): Promise<AnalyticsSummary> {
	return apiClient(`/api/me/analytics/summary?${qs(q)}`);
}

export async function getAnalyticsTimeseries(q: AnalyticsQuery = {}): Promise<AnalyticsPoint[]> {
	return (await apiClient<{ points: AnalyticsPoint[] }>(`/api/me/analytics/timeseries?${qs(q)}`)).points;
}

export async function getAnalyticsContent(q: AnalyticsQuery = {}): Promise<ContentStat[]> {
	return (await apiClient<{ items: ContentStat[] }>(`/api/me/analytics/content?${qs(q)}`)).items;
}

export async function getAnalyticsCards(q: AnalyticsQuery = {}): Promise<CardStat[]> {
	return (await apiClient<{ cards: CardStat[] }>(`/api/me/analytics/cards?${qs(q)}`)).cards;
}

/** Admins and team leads only. */
export async function getAnalyticsTeams(q: AnalyticsQuery = {}): Promise<TeamStat[]> {
	return (await apiClient<{ teams: TeamStat[] }>(`/api/me/analytics/teams?${qs(q)}`)).teams;
}

export async function getAnalyticsMembers(q: AnalyticsQuery = {}): Promise<MemberStat[]> {
	return (await apiClient<{ members: MemberStat[] }>(`/api/me/analytics/members?${qs(q)}`)).members;
}

export async function getAnalyticsActivity(q: AnalyticsQuery = {}, limit = 15): Promise<ActivityItem[]> {
	return (await apiClient<{ items: ActivityItem[] }>(`/api/me/analytics/activity?${qs(q, { limit: String(limit) })}`)).items;
}

/** The last `days` days, ending now. */
export function lastDays(days: number): { from: Date; to: Date } {
	const to = new Date();
	return { from: new Date(to.getTime() - days * 86_400_000), to };
}

/** a/b as a percentage, or null when b is 0. */
export function rate(a: number, b: number): number | null {
	return b > 0 ? (a / b) * 100 : null;
}

/** Change from prev to cur in percent; null when there's nothing to compare against. */
export function change(cur: number, prev: number): number | null {
	if (prev === 0) return cur === 0 ? 0 : null;
	return ((cur - prev) / prev) * 100;
}

export const SOURCES: { key: VisitSource; label: string; color: string }[] = [
	{ key: 'nfc', label: 'NFC tap', color: 'var(--viz-1)' },
	{ key: 'qr', label: 'QR scan', color: 'var(--viz-2)' },
	{ key: 'link', label: 'Link', color: 'var(--viz-3)' }
];

export function formatDuration(ms: number): string {
	if (ms <= 0) return '–';
	const s = Math.round(ms / 1000);
	if (s < 60) return `${s}s`;
	const m = Math.floor(s / 60);
	return `${m}m ${String(s % 60).padStart(2, '0')}s`;
}

const compact = new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 });
export function formatCount(n: number): string {
	return n < 10_000 ? n.toLocaleString() : compact.format(n);
}
