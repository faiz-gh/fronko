import {
	siApplemusic,
	siArtstation,
	siBehance,
	siBitbucket,
	siBluesky,
	siBuymeacoffee,
	siCaldotcom,
	siCalendly,
	siCashapp,
	siDevdotto,
	siDeviantart,
	siDiscord,
	siDribbble,
	siEtsy,
	siFacebook,
	siFigma,
	siFiverr,
	siGithub,
	siGitlab,
	siGlassdoor,
	siGoodreads,
	siGooglecalendar,
	siGoogledrive,
	siGooglescholar,
	siHashnode,
	siHubspot,
	siHuggingface,
	siIndeed,
	siInstagram,
	siKaggle,
	siKofi,
	siLeetcode,
	siLetterboxd,
	siLinktree,
	siMedium,
	siNotion,
	siNpm,
	siOrcid,
	siPatreon,
	siPaypal,
	siPeerlist,
	siPinterest,
	siProducthunt,
	siQuora,
	siReddit,
	siResearchgate,
	siSignal,
	siSnapchat,
	siSoundcloud,
	siSpotify,
	siStackoverflow,
	siStrava,
	siSubstack,
	siTelegram,
	siThreads,
	siTiktok,
	siTumblr,
	siTwitch,
	siUnsplash,
	siUpwork,
	siVenmo,
	siVimeo,
	siWellfound,
	siWhatsapp,
	siX,
	siXing,
	siYoutube,
	siZoho
} from 'simple-icons';
import { fileUrl } from '$lib/features/files/api';
import { apiUrl } from '$lib/core/api';
import { parseLegacyPhone } from '$lib/core/phone';
import { normalizeBlocks, templateBlocks, TEMPLATES, type CardBlock, type TemplateKey } from './blocks';
import { defaultSignature, normalizeSignature, type SignatureSettings } from '$lib/features/signatures/templates';

/** The shape stored in the profile's JSONB `data` column. */
export interface CardData {
	name: string;
	title: string;
	company: string;
	bio: string;
	avatar_url: string;
	/** A photo from the file library (public file id); takes precedence over avatar_url. */
	avatar_file: string;
	/** Cover banner from the file library (public file id), cropped to 3:1. */
	cover_file: string;
	email: string;
	/** ISO country of the phone number ("IN"); picks the right flag when dial codes are shared. */
	phone_country: string;
	/** Dial code with a leading "+", digits only ("+91"). */
	phone_country_code: string;
	/** National number, digits only ("9876543210"). */
	phone_number: string;
	website: string;
	location: string;
	links: CardLink[];
	/** PDF brochures from the file library, shown as downloads. */
	documents: CardDocument[];
	accent: AccentKey;
	theme: 'light' | 'dark';
	collect_leads: boolean;
	/** The layout preset last applied; the blocks may have been changed since. */
	template: TemplateKey;
	/** What the card shows, in order. */
	blocks: CardBlock[];
	/** What happens when someone taps the NFC tag or scans the QR code. */
	tap: Record<TapSource, TapAction>;
	/** Show the organisation's logo; ignored when the organisation requires it. */
	show_org_logo: boolean;
	/** How this card's email signature looks. */
	signature: SignatureSettings;
	/** How this card's QR code looks. */
	qr: QrStyle;
}

export type QrDots = 'square' | 'rounded' | 'dots';
export type QrCorners = 'square' | 'rounded' | 'dot';
/** What sits in the middle of the QR code: nothing, the organisation's logo, or an image from Files. */
export type QrImage = 'none' | 'org_logo' | 'custom';

export interface QrStyle {
	/** Module colour, #rrggbb. */
	fg: string;
	/** Background colour, #rrggbb. */
	bg: string;
	dots: QrDots;
	corners: QrCorners;
	image: QrImage;
	/** Public file id of the custom centre image. */
	image_file: string;
	/** Width of the centre image as a share of the code (0.15–0.25). */
	image_scale: number;
}

export const QR_IMAGE_SCALE = { min: 0.15, max: 0.25, default: 0.2 } as const;

export function defaultQrStyle(): QrStyle {
	return {
		fg: '#0a0a0a',
		bg: '#ffffff',
		dots: 'square',
		corners: 'square',
		image: 'none',
		image_file: '',
		image_scale: QR_IMAGE_SCALE.default
	};
}

const HEX = /^#[0-9a-f]{6}$/i;

export function normalizeQrStyle(raw: unknown): QrStyle {
	const d = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
	const base = defaultQrStyle();
	const pick = <T extends string>(v: unknown, options: readonly T[], fallback: T): T =>
		typeof v === 'string' && (options as readonly string[]).includes(v) ? (v as T) : fallback;
	const scale = typeof d.image_scale === 'number' && Number.isFinite(d.image_scale) ? d.image_scale : base.image_scale;
	const image = pick(d.image, ['none', 'org_logo', 'custom'] as const, 'none');
	const file = typeof d.image_file === 'string' ? d.image_file : '';
	return {
		fg: typeof d.fg === 'string' && HEX.test(d.fg) ? d.fg.toLowerCase() : base.fg,
		bg: typeof d.bg === 'string' && HEX.test(d.bg) ? d.bg.toLowerCase() : base.bg,
		dots: pick(d.dots, ['square', 'rounded', 'dots'] as const, 'square'),
		corners: pick(d.corners, ['square', 'rounded', 'dot'] as const, 'square'),
		// A custom image with no file falls back to none.
		image: image === 'custom' && !file ? 'none' : image,
		image_file: file,
		image_scale: Math.min(QR_IMAGE_SCALE.max, Math.max(QR_IMAGE_SCALE.min, scale))
	};
}

/** Where a visit came from, as marked on the URL written to the tag or QR code. */
export type TapSource = 'nfc' | 'qr';
export type TapAction = 'profile' | 'save_contact' | 'lead_form';

export const TAP_ACTIONS: Record<TapAction, { label: string; description: string }> = {
	profile: { label: 'Show profile', description: 'Open your card as usual.' },
	save_contact: {
		label: 'Save contact',
		description: 'Open the phone’s “Add contact” sheet, with your card behind it.'
	},
	lead_form: { label: 'Open contact form', description: 'Ask visitors for their details straight away.' }
};

function tapAction(v: unknown): TapAction {
	return typeof v === 'string' && v in TAP_ACTIONS ? (v as TapAction) : 'profile';
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
		cover_file: '',
		email: '',
		phone_country: '',
		phone_country_code: '',
		phone_number: '',
		website: '',
		location: '',
		links: [],
		documents: [],
		accent: 'indigo',
		theme: 'light',
		collect_leads: true,
		template: 'classic',
		blocks: templateBlocks('classic'),
		tap: { nfc: 'profile', qr: 'profile' },
		show_org_logo: true,
		signature: defaultSignature(),
		qr: defaultQrStyle()
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
 * Older profiles also stored the phone as one free-text `phone` string; that is
 * split into dial code and number here, and the old key is gone after a save.
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
		? d.documents
				.flatMap((doc): CardDocument[] => {
					if (!doc || typeof doc !== 'object') return [];
					const o = doc as Record<string, unknown>;
					const file = str(o.file);
					return file ? [{ id: str(o.id) || newId(), file, title: str(o.title) }] : [];
				})
				.slice(0, MAX_DOCUMENTS)
		: [];

	const phone = str(d.phone_number)
		? { country: str(d.phone_country), code: str(d.phone_country_code), number: str(d.phone_number) }
		: parseLegacyPhone(str(d.phone));

	return {
		name: str(d.name),
		title: str(d.title),
		company: str(d.company),
		bio: str(d.bio),
		avatar_url: str(d.avatar_url),
		avatar_file: str(d.avatar_file),
		cover_file: str(d.cover_file),
		email: str(d.email),
		phone_country: phone.country,
		phone_country_code: phone.code,
		phone_number: phone.number,
		website: str(d.website),
		location: str(d.location),
		links,
		documents,
		accent: typeof d.accent === 'string' && d.accent in ACCENTS ? (d.accent as AccentKey) : base.accent,
		theme: d.theme === 'dark' ? 'dark' : 'light',
		collect_leads: typeof d.collect_leads === 'boolean' ? d.collect_leads : true,
		template: typeof d.template === 'string' && d.template in TEMPLATES ? (d.template as TemplateKey) : 'classic',
		blocks: normalizeBlocks(d.blocks),
		tap: {
			nfc: tapAction((d.tap as Record<string, unknown> | undefined)?.nfc),
			qr: tapAction((d.tap as Record<string, unknown> | undefined)?.qr)
		},
		show_org_logo: typeof d.show_org_logo === 'boolean' ? d.show_org_logo : true,
		signature: normalizeSignature(d.signature),
		qr: normalizeQrStyle(d.qr)
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

/** The card's cover banner, or null to show the accent gradient. */
export function coverSrc(card: Pick<CardData, 'cover_file'>): string | null {
	return card.cover_file ? new URL(fileUrl(card.cover_file), location.origin).href : null;
}

/** The card's photo: an uploaded file first, then a pasted URL. Null means show initials. */
export function avatarSrc(card: Pick<CardData, 'avatar_file' | 'avatar_url'>): string | null {
	if (card.avatar_file) return new URL(fileUrl(card.avatar_file), location.origin).href;
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

/** An SVG path on a 24×24 viewBox; matches simple-icons' shape. */
export interface BrandGlyph {
	path: string;
}

interface Brand {
	name: string;
	icon: BrandGlyph | null;
	hosts: string[];
}

// simple-icons dropped LinkedIn for trademark reasons; this is its former path.
const LINKEDIN: BrandGlyph = {
	path: 'M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 4.268v6.293zM5.337 7.433c-1.144 0-2.063-.926-2.063-2.065 0-1.138.92-2.063 2.063-2.063 1.14 0 2.064.925 2.064 2.063 0 1.139-.925 2.065-2.064 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z'
};

const BRANDS: Brand[] = [
	// Professional
	{ name: 'LinkedIn', icon: LINKEDIN, hosts: ['linkedin.com', 'lnkd.in'] },
	{ name: 'GitHub', icon: siGithub, hosts: ['github.com'] },
	{ name: 'GitLab', icon: siGitlab, hosts: ['gitlab.com'] },
	{ name: 'Bitbucket', icon: siBitbucket, hosts: ['bitbucket.org'] },
	{ name: 'Stack Overflow', icon: siStackoverflow, hosts: ['stackoverflow.com'] },
	{ name: 'Indeed', icon: siIndeed, hosts: ['indeed.com', 'indeed.co.uk', 'indeed.co.in', 'indeed.ca', 'indeed.me'] },
	{ name: 'Glassdoor', icon: siGlassdoor, hosts: ['glassdoor.com', 'glassdoor.co.in', 'glassdoor.co.uk'] },
	{ name: 'Wellfound', icon: siWellfound, hosts: ['wellfound.com', 'angel.co'] },
	{ name: 'Xing', icon: siXing, hosts: ['xing.com'] },
	{ name: 'Upwork', icon: siUpwork, hosts: ['upwork.com'] },
	{ name: 'Fiverr', icon: siFiverr, hosts: ['fiverr.com'] },
	{ name: 'Peerlist', icon: siPeerlist, hosts: ['peerlist.io'] },
	{ name: 'Product Hunt', icon: siProducthunt, hosts: ['producthunt.com'] },
	{ name: 'Google Scholar', icon: siGooglescholar, hosts: ['scholar.google.com'] },
	{ name: 'ResearchGate', icon: siResearchgate, hosts: ['researchgate.net'] },
	{ name: 'ORCID', icon: siOrcid, hosts: ['orcid.org'] },
	{ name: 'Notion', icon: siNotion, hosts: ['notion.so', 'notion.site'] },
	{ name: 'Google Drive', icon: siGoogledrive, hosts: ['drive.google.com', 'docs.google.com'] },
	// Developer
	{ name: 'DEV', icon: siDevdotto, hosts: ['dev.to'] },
	{ name: 'Hashnode', icon: siHashnode, hosts: ['hashnode.com', 'hashnode.dev'] },
	{ name: 'LeetCode', icon: siLeetcode, hosts: ['leetcode.com'] },
	{ name: 'Kaggle', icon: siKaggle, hosts: ['kaggle.com'] },
	{ name: 'Hugging Face', icon: siHuggingface, hosts: ['huggingface.co'] },
	{ name: 'npm', icon: siNpm, hosts: ['npmjs.com'] },
	// Design & creative
	{ name: 'Figma', icon: siFigma, hosts: ['figma.com'] },
	{ name: 'Dribbble', icon: siDribbble, hosts: ['dribbble.com'] },
	{ name: 'Behance', icon: siBehance, hosts: ['behance.net'] },
	{ name: 'ArtStation', icon: siArtstation, hosts: ['artstation.com'] },
	{ name: 'DeviantArt', icon: siDeviantart, hosts: ['deviantart.com'] },
	{ name: 'Unsplash', icon: siUnsplash, hosts: ['unsplash.com'] },
	{ name: 'Etsy', icon: siEtsy, hosts: ['etsy.com'] },
	// Social
	{ name: 'X', icon: siX, hosts: ['x.com', 'twitter.com'] },
	{ name: 'Instagram', icon: siInstagram, hosts: ['instagram.com'] },
	{ name: 'Facebook', icon: siFacebook, hosts: ['facebook.com', 'fb.com'] },
	{ name: 'Threads', icon: siThreads, hosts: ['threads.net', 'threads.com'] },
	{ name: 'Bluesky', icon: siBluesky, hosts: ['bsky.app'] },
	{ name: 'TikTok', icon: siTiktok, hosts: ['tiktok.com'] },
	{ name: 'Snapchat', icon: siSnapchat, hosts: ['snapchat.com'] },
	{ name: 'Pinterest', icon: siPinterest, hosts: ['pinterest.com', 'pin.it'] },
	{ name: 'Reddit', icon: siReddit, hosts: ['reddit.com'] },
	{ name: 'Tumblr', icon: siTumblr, hosts: ['tumblr.com'] },
	{ name: 'Quora', icon: siQuora, hosts: ['quora.com'] },
	{ name: 'Linktree', icon: siLinktree, hosts: ['linktr.ee'] },
	{ name: 'Goodreads', icon: siGoodreads, hosts: ['goodreads.com'] },
	{ name: 'Letterboxd', icon: siLetterboxd, hosts: ['letterboxd.com', 'boxd.it'] },
	{ name: 'Strava', icon: siStrava, hosts: ['strava.com'] },
	// Video, music & writing
	{ name: 'YouTube', icon: siYoutube, hosts: ['youtube.com', 'youtu.be'] },
	{ name: 'Twitch', icon: siTwitch, hosts: ['twitch.tv'] },
	{ name: 'Vimeo', icon: siVimeo, hosts: ['vimeo.com'] },
	{ name: 'Spotify', icon: siSpotify, hosts: ['spotify.com', 'spotify.link'] },
	{ name: 'Apple Music', icon: siApplemusic, hosts: ['music.apple.com'] },
	{ name: 'SoundCloud', icon: siSoundcloud, hosts: ['soundcloud.com'] },
	{ name: 'Medium', icon: siMedium, hosts: ['medium.com'] },
	{ name: 'Substack', icon: siSubstack, hosts: ['substack.com'] },
	// Messaging
	{ name: 'WhatsApp', icon: siWhatsapp, hosts: ['wa.me', 'whatsapp.com'] },
	{ name: 'Telegram', icon: siTelegram, hosts: ['t.me', 'telegram.me'] },
	{ name: 'Signal', icon: siSignal, hosts: ['signal.me', 'signal.group'] },
	{ name: 'Discord', icon: siDiscord, hosts: ['discord.gg', 'discord.com'] },
	// Support & payments
	{ name: 'Patreon', icon: siPatreon, hosts: ['patreon.com'] },
	{ name: 'Buy Me a Coffee', icon: siBuymeacoffee, hosts: ['buymeacoffee.com'] },
	{ name: 'Ko-fi', icon: siKofi, hosts: ['ko-fi.com'] },
	{ name: 'PayPal', icon: siPaypal, hosts: ['paypal.me', 'paypal.com'] },
	{ name: 'Cash App', icon: siCashapp, hosts: ['cash.app'] },
	{ name: 'Venmo', icon: siVenmo, hosts: ['venmo.com'] }
];

function matchHost<T extends { hosts: string[] }>(url: string, list: T[]): T | null {
	const safe = safeUrl(url);
	if (!safe) return null;
	const host = new URL(safe).hostname.replace(/^www\./, '');
	return list.find((b) => b.hosts.some((h) => host === h || host.endsWith(`.${h}`))) ?? null;
}

export function detectBrand(url: string): Brand | null {
	return matchHost(url, BRANDS);
}

// ----------------------------------------------------------------------------
// Booking links
// ----------------------------------------------------------------------------

/** A null icon means "use the generic calendar icon". */
const CALENDAR_PROVIDERS: Brand[] = [
	{ name: 'Calendly', icon: siCalendly, hosts: ['calendly.com'] },
	{ name: 'Cal.com', icon: siCaldotcom, hosts: ['cal.com'] },
	{ name: 'Google Calendar', icon: siGooglecalendar, hosts: ['calendar.google.com', 'calendar.app.google'] },
	{ name: 'HubSpot', icon: siHubspot, hosts: ['meetings.hubspot.com'] },
	{
		name: 'Zoho Bookings',
		icon: siZoho,
		hosts: [
			'bookings.zoho.com',
			'bookings.zoho.eu',
			'bookings.zoho.in',
			'zohobookings.com',
			'zohobookings.eu',
			'zohobookings.in'
		]
	},
	{
		name: 'Microsoft Bookings',
		icon: null,
		hosts: ['outlook.office.com', 'outlook.office365.com', 'outlook.live.com']
	},
	{ name: 'SavvyCal', icon: null, hosts: ['savvycal.com'] },
	{ name: 'TidyCal', icon: null, hosts: ['tidycal.com'] },
	{ name: 'zcal', icon: null, hosts: ['zcal.co'] }
];

export function detectCalendar(url: string): Brand | null {
	return matchHost(url, CALENDAR_PROVIDERS);
}

/** What a visitor told the card's lead form, for filling in a booking page. */
export interface Visitor {
	name: string;
	email: string;
}

/**
 * The booking page's link, with the visitor's details filled in where the
 * page supports it. Returns null for a link that isn't http(s).
 */
export function bookingHref(
	booking: { url: string; prefill?: Record<string, string> },
	visitor?: Visitor | null
): string | null {
	const safe = safeUrl(booking.url);
	if (!safe) return null;
	if (!visitor || !booking.prefill) return safe;
	const [first, ...rest] = visitor.name.trim().split(/\s+/);
	const values: Record<string, string> = {
		name: visitor.name.trim(),
		first_name: first ?? '',
		last_name: rest.join(' '),
		email: visitor.email.trim()
	};
	const url = new URL(safe);
	for (const [param, from] of Object.entries(booking.prefill)) {
		if (values[from]) url.searchParams.set(param, values[from]);
	}
	return url.toString();
}

export function linkLabel(link: CardLink): string {
	return link.label.trim() || detectBrand(link.url)?.name || displayUrl(link.url);
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

/**
 * The card's contact file, built by the server; opening it shows the phone's
 * "Add contact" sheet. On the public card, `visit` ties the save to the visit
 * for analytics.
 */
export function vcardUrl(org: string, slug: string, visit?: { via: string; session: string }): string {
	const base = apiUrl(`/api/profiles/${cardPath(org, slug)}/vcard`);
	return visit ? `${base}?${new URLSearchParams({ via: visit.via, s: visit.session })}` : base;
}

/**
 * A card's place in its link, `{org}/{slug}`: the organisation's handle, then
 * the card's slug, which only has to be unique inside the organisation.
 */
export function cardPath(org: string, slug: string): string {
	return `${encodeURIComponent(org)}/${encodeURIComponent(slug)}`;
}

export function publicUrl(org: string, slug: string): string {
	return `${location.origin}/p/${cardPath(org, slug)}`;
}

/** The link to write to an NFC tag or encode in a QR code; it runs the card's tap action. */
export function tapUrl(org: string, slug: string, via: TapSource): string {
	return `${publicUrl(org, slug)}?via=${via}`;
}
