import {
	siBehance,
	siBluesky,
	siDiscord,
	siDribbble,
	siFacebook,
	siGithub,
	siInstagram,
	siMedium,
	siTelegram,
	siThreads,
	siTiktok,
	siWhatsapp,
	siX,
	siYoutube,
	type SimpleIcon
} from 'simple-icons';

/** The shape stored in the profile's JSONB `data` column. */
export interface CardData {
	name: string;
	title: string;
	company: string;
	bio: string;
	avatar_url: string;
	/** A photo from the file library (public file id); takes precedence over avatar_url. */
	avatar_file: string;
	email: string;
	phone: string;
	website: string;
	location: string;
	links: CardLink[];
	/** PDF brochures from the file library, shown as downloads. */
	documents: CardDocument[];
	accent: AccentKey;
	theme: 'light' | 'dark';
	collect_leads: boolean;
}

export interface CardLink {
	id: string;
	label: string;
	url: string;
}

export interface CardDocument {
	id: string;
	/** Public file id in the library. */
	file: string;
	title: string;
}

export const MAX_DOCUMENTS = 10;

export const ACCENTS = {
	indigo: 'oklch(0.52 0.21 268)',
	violet: 'oklch(0.53 0.24 293)',
	rose: 'oklch(0.58 0.22 12)',
	orange: 'oklch(0.64 0.19 45)',
	emerald: 'oklch(0.56 0.14 160)',
	sky: 'oklch(0.58 0.14 236)',
	slate: 'oklch(0.35 0.03 264)'
} as const;

export type AccentKey = keyof typeof ACCENTS;

export function emptyCard(name = ''): CardData {
	return {
		name,
		title: '',
		company: '',
		bio: '',
		avatar_url: '',
		avatar_file: '',
		email: '',
		phone: '',
		website: '',
		location: '',
		links: [],
		documents: [],
		accent: 'indigo',
		theme: 'light',
		collect_leads: true
	};
}

function str(v: unknown): string {
	return typeof v === 'string' ? v : '';
}

export function newId(): string {
	return Math.random().toString(36).slice(2, 10);
}

/**
 * Coerces whatever is stored in `data` into a complete CardData. Older profiles
 * stored links as plain strings; those are upgraded to {label, url} objects.
 */
export function normalizeCard(raw: unknown): CardData {
	const d = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
	const base = emptyCard();

	const links: CardLink[] = Array.isArray(d.links)
		? d.links.flatMap((l): CardLink[] => {
				if (typeof l === 'string') return [{ id: newId(), label: '', url: l }];
				if (l && typeof l === 'object') {
					const o = l as Record<string, unknown>;
					return [{ id: str(o.id) || newId(), label: str(o.label), url: str(o.url) }];
				}
				return [];
			})
		: [];

	const documents: CardDocument[] = Array.isArray(d.documents)
		? d.documents.flatMap((doc): CardDocument[] => {
				if (!doc || typeof doc !== 'object') return [];
				const o = doc as Record<string, unknown>;
				const file = str(o.file);
				return file ? [{ id: str(o.id) || newId(), file, title: str(o.title) }] : [];
			}).slice(0, MAX_DOCUMENTS)
		: [];

	return {
		name: str(d.name),
		title: str(d.title),
		company: str(d.company),
		bio: str(d.bio),
		avatar_url: str(d.avatar_url),
		avatar_file: str(d.avatar_file),
		email: str(d.email),
		phone: str(d.phone),
		website: str(d.website),
		location: str(d.location),
		links,
		documents,
		accent: typeof d.accent === 'string' && d.accent in ACCENTS ? (d.accent as AccentKey) : base.accent,
		theme: d.theme === 'dark' ? 'dark' : 'light',
		collect_leads: typeof d.collect_leads === 'boolean' ? d.collect_leads : true
	};
}

/**
 * Returns an absolute http(s) URL, or null. Anything else (javascript:, data:, …)
 * is rejected so visitor-facing links can't run script.
 */
export function safeUrl(input: string): string | null {
	const trimmed = input.trim();
	if (!trimmed) return null;
	const withScheme = /^[a-z][a-z0-9+.-]*:/i.test(trimmed) ? trimmed : `https://${trimmed}`;
	try {
		const url = new URL(withScheme);
		return url.protocol === 'https:' || url.protocol === 'http:' ? url.href : null;
	} catch {
		return null;
	}
}

/** The card's photo: an uploaded file first, then a pasted URL. Null means show initials. */
export function avatarSrc(card: Pick<CardData, 'avatar_file' | 'avatar_url'>): string | null {
	if (card.avatar_file) return `${location.origin}/api/files/${encodeURIComponent(card.avatar_file)}`;
	return safeUrl(card.avatar_url);
}

export function displayUrl(input: string): string {
	const url = safeUrl(input);
	if (!url) return input;
	const u = new URL(url);
	return (u.host.replace(/^www\./, '') + u.pathname).replace(/\/$/, '');
}

export function initials(name: string, fallback = '?'): string {
	const parts = name.trim().split(/\s+/).filter(Boolean);
	if (parts.length === 0) return fallback.charAt(0).toUpperCase();
	return (parts[0][0] + (parts.length > 1 ? parts[parts.length - 1][0] : '')).toUpperCase();
}

export function slugify(input: string): string {
	return input
		.toLowerCase()
		.normalize('NFKD')
		.replace(/[\u0300-\u036f]/g, '')
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, 48);
}

export const SLUG_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

export function isValidSlug(slug: string): boolean {
	return slug.length >= 3 && slug.length <= 48 && SLUG_PATTERN.test(slug);
}

// ----------------------------------------------------------------------------
// Social link detection
// ----------------------------------------------------------------------------

interface Brand {
	name: string;
	icon: SimpleIcon | null;
	hosts: string[];
}

const BRANDS: Brand[] = [
	{ name: 'GitHub', icon: siGithub, hosts: ['github.com'] },
	// simple-icons dropped LinkedIn for trademark reasons; fall back to the generic icon.
	{ name: 'LinkedIn', icon: null, hosts: ['linkedin.com'] },
	{ name: 'X', icon: siX, hosts: ['x.com', 'twitter.com'] },
	{ name: 'Instagram', icon: siInstagram, hosts: ['instagram.com'] },
	{ name: 'YouTube', icon: siYoutube, hosts: ['youtube.com', 'youtu.be'] },
	{ name: 'Facebook', icon: siFacebook, hosts: ['facebook.com', 'fb.com'] },
	{ name: 'TikTok', icon: siTiktok, hosts: ['tiktok.com'] },
	{ name: 'Dribbble', icon: siDribbble, hosts: ['dribbble.com'] },
	{ name: 'Behance', icon: siBehance, hosts: ['behance.net'] },
	{ name: 'Medium', icon: siMedium, hosts: ['medium.com'] },
	{ name: 'Threads', icon: siThreads, hosts: ['threads.net', 'threads.com'] },
	{ name: 'Bluesky', icon: siBluesky, hosts: ['bsky.app'] },
	{ name: 'WhatsApp', icon: siWhatsapp, hosts: ['wa.me', 'whatsapp.com'] },
	{ name: 'Telegram', icon: siTelegram, hosts: ['t.me', 'telegram.me'] },
	{ name: 'Discord', icon: siDiscord, hosts: ['discord.gg', 'discord.com'] }
];

export function detectBrand(url: string): Brand | null {
	const safe = safeUrl(url);
	if (!safe) return null;
	const host = new URL(safe).hostname.replace(/^www\./, '');
	return BRANDS.find((b) => b.hosts.some((h) => host === h || host.endsWith(`.${h}`))) ?? null;
}

export function linkLabel(link: CardLink): string {
	return link.label.trim() || detectBrand(link.url)?.name || displayUrl(link.url);
}

// ----------------------------------------------------------------------------
// vCard export
// ----------------------------------------------------------------------------

function vEscape(value: string): string {
	return value.replace(/\\/g, '\\\\').replace(/\n/g, '\\n').replace(/[,;]/g, (m) => `\\${m}`);
}

/** Builds a vCard 3.0 file, which iOS and Android both import as a contact. */
export function buildVCard(card: CardData, profileUrl: string): string {
	const name = card.name.trim() || 'Contact';
	const parts = name.split(/\s+/);
	const last = parts.length > 1 ? parts[parts.length - 1] : '';
	const first = parts.length > 1 ? parts.slice(0, -1).join(' ') : parts[0];

	const lines = ['BEGIN:VCARD', 'VERSION:3.0', `N:${vEscape(last)};${vEscape(first)};;;`, `FN:${vEscape(name)}`];
	if (card.company) lines.push(`ORG:${vEscape(card.company)}`);
	if (card.title) lines.push(`TITLE:${vEscape(card.title)}`);
	if (card.email) lines.push(`EMAIL;TYPE=INTERNET:${vEscape(card.email)}`);
	if (card.phone) lines.push(`TEL;TYPE=CELL:${vEscape(card.phone)}`);
	const website = safeUrl(card.website);
	if (website) lines.push(`URL:${vEscape(website)}`);
	lines.push(`URL:${vEscape(profileUrl)}`);
	if (card.location) lines.push(`ADR;TYPE=WORK:;;;${vEscape(card.location)};;;`);
	if (card.bio) lines.push(`NOTE:${vEscape(card.bio)}`);
	lines.push('END:VCARD');
	return lines.join('\r\n');
}

/** Triggers a browser download of in-memory data. */
export function downloadBlob(blob: Blob, filename: string) {
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	a.click();
	URL.revokeObjectURL(url);
}

export function downloadVCard(card: CardData, profileUrl: string, slug: string) {
	downloadBlob(new Blob([buildVCard(card, profileUrl)], { type: 'text/vcard;charset=utf-8' }), `${slug}.vcf`);
}

export function publicUrl(slug: string): string {
	return `${location.origin}/p/${slug}`;
}
