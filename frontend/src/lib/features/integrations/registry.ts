import type { Component } from 'svelte';
import CalendarClockIcon from '@lucide/svelte/icons/calendar-clock';
import SendIcon from '@lucide/svelte/icons/send';
import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
import UsersRoundIcon from '@lucide/svelte/icons/users-round';
import WebhookIcon from '@lucide/svelte/icons/webhook';
import {
	siCalendly,
	siGoogle,
	siGooglecalendar,
	siHubspot,
	siMake,
	siN8n,
	siOkta,
	siZapier,
	siZoho
} from 'simple-icons';
import type { Category, CatalogEntry, ConnectionStatus, Scope } from './types';

/**
 * How a provider looks in the catalog. `path` is a simple-icons glyph on a
 * 24×24 viewBox; `icon` a Lucide icon; `mark` a multi-colour mark drawn by
 * ProviderLogo; otherwise `initials` (or ones taken from its name) on its
 * colour. Providers without an entry get initials on a neutral tile.
 */
export interface ProviderLogo {
	color: string;
	path?: string;
	icon?: Component<{ class?: string }>;
	/** simple-icons has no Microsoft glyphs, so its four squares are drawn instead. */
	mark?: 'microsoft';
	/** Set where the name's own initials would be unclear or clash (both Salesforce products). */
	initials?: string;
}

const microsoft: ProviderLogo = { mark: 'microsoft', color: '#0078d4' };

const glyph = (si: { path: string; hex: string }): ProviderLogo => ({ path: si.path, color: `#${si.hex}` });

export const LOGOS: Record<string, ProviderLogo> = {
	webhook: { icon: WebhookIcon, color: '#6366f1' },
	zapier: glyph(siZapier),
	make: glyph(siMake),
	n8n: glyph(siN8n),
	hubspot: glyph(siHubspot),
	'hubspot-meetings': glyph(siHubspot),
	salesforce: { color: '#00a1e0', initials: 'SF' },
	pardot: { color: '#00a1e0', initials: 'AE' },
	'zoho-crm': glyph(siZoho),
	'dynamics-365': microsoft,
	pipedrive: { color: '#25a85a', initials: 'PD' },
	monday: { color: '#ff3d57' },
	marketo: { color: '#5c4c9f', initials: 'MK' },
	slack: { color: '#4a154b' },
	outlook: microsoft,
	calendly: glyph(siCalendly),
	'chili-piper': { color: '#e8483f' },
	'microsoft-bookings': microsoft,
	'google-calendar': glyph(siGooglecalendar),
	'booking-link': { icon: CalendarClockIcon, color: '#0ea5e9' },
	'entra-scim': microsoft,
	'entra-saml': microsoft,
	'okta-scim': glyph(siOkta),
	'okta-saml': glyph(siOkta),
	'google-workspace': glyph(siGoogle),
	saml: { icon: ShieldCheckIcon, color: '#475569' }
};

export function logoFor(id: string): ProviderLogo {
	return LOGOS[id] ?? { color: '#71717a' };
}

/** Up to two letters from a provider's name, for logos without a glyph. */
export function initialsOf(name: string): string {
	const words = name
		.replace(/[^\p{L}\p{N} ]/gu, ' ')
		.split(/\s+/)
		.filter(Boolean);
	if (words.length === 0) return '?';
	if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
	return (words[0][0] + words[1][0]).toUpperCase();
}

export const CATEGORY_ICONS: Record<Category, Component<{ class?: string }>> = {
	lead_sync: SendIcon,
	calendar: CalendarClockIcon,
	directory: UsersRoundIcon,
	sso: ShieldCheckIcon
};

export const SCOPE_LABEL: Record<Scope, string> = { org: 'Organisation', user: 'Personal' };

/** What a connection's scope means, by category. */
export const SCOPE_HELP: Record<Category, Record<Scope, string>> = {
	lead_sync: {
		org: 'Gets every lead your organisation’s cards collect.',
		user: 'Gets the leads on the cards you hold.'
	},
	calendar: {
		org: 'The booking page on cards whose holder hasn’t connected their own.',
		user: 'Your own booking page, shown on the cards you hold.'
	},
	directory: { org: 'Manages your organisation’s people.', user: '' },
	sso: { org: 'How people in your organisation sign in.', user: '' }
};

export const STATUS_LABEL: Record<ConnectionStatus, string> = {
	pending: 'Setup incomplete',
	active: 'Active',
	error: 'Needs attention'
};

export type Tone = 'success' | 'warning' | 'danger' | 'muted';

/** The badge for a provider tile: how its connections are doing, if it has any. */
export function catalogBadge(entry: CatalogEntry): { label: string; tone: Tone } | null {
	if (entry.status === 'coming_soon') return { label: 'Coming soon', tone: 'muted' };
	if (entry.unavailable) return { label: 'Unavailable', tone: 'muted' };
	const conns = entry.connections;
	if (conns.length === 0) return entry.status === 'beta' ? { label: 'Beta', tone: 'muted' } : null;
	if (conns.some((c) => c.enabled && c.status === 'error')) return { label: 'Needs attention', tone: 'danger' };
	const active = conns.filter((c) => c.enabled && c.status === 'active').length;
	if (active > 0) return { label: conns.length > 1 ? `${active} active` : 'Connected', tone: 'success' };
	if (conns.some((c) => c.status === 'pending')) return { label: 'Setup incomplete', tone: 'warning' };
	return { label: 'Paused', tone: 'muted' };
}

/** Case-insensitive search over a provider's name, description and keywords. */
export function matchesSearch(entry: Pick<CatalogEntry, 'name' | 'description' | 'keywords'>, query: string): boolean {
	const q = query.trim().toLowerCase();
	if (!q) return true;
	return [entry.name, entry.description, ...(entry.keywords ?? [])].some((s) => s.toLowerCase().includes(q));
}
