import { fileUrl } from '$lib/api/files';
import { showsOrgLogo, type OrgBranding } from '$lib/api/org';
import { avatarSrc, displayUrl, linkLabel, publicUrl, safeUrl, type AccentKey, type CardData } from '$lib/card/card';
import { e164, formatPhone } from '$lib/phone';
import { SIGNATURE_TEMPLATES, isSignatureTemplate, type SignatureTemplateKey } from './templates';

/*
 * Email clients are a hostile place for HTML: Gmail strips <style> and
 * classes, Outlook renders with Word and ignores most modern CSS. So
 * signatures are tables with inline styles only, web-safe fonts, absolute
 * image URLs with explicit sizes, and no SVG.
 */

/** Hex stand-ins for the card accents, which are oklch() and unsupported in mail clients. */
export const ACCENT_HEX: Record<AccentKey, string> = {
	indigo: '#4f46e5',
	violet: '#7c3aed',
	rose: '#e11d48',
	orange: '#ea580c',
	emerald: '#059669',
	sky: '#0284c7',
	slate: '#334155'
};

const FONT = 'Arial, Helvetica, sans-serif';
const TEXT = '#1f2937';
const MUTED = '#6b7280';

/** Natural pixel size of an image, measured by the page so the HTML can carry exact dimensions. */
export interface ImageSize {
	width: number;
	height: number;
}

export interface RenderOptions {
	/** Overrides the card's chosen template (used for gallery thumbnails). */
	template?: SignatureTemplateKey;
	logoSize?: ImageSize | null;
	bannerSize?: ImageSize | null;
}

export interface RenderedSignature {
	template: SignatureTemplateKey;
	/** A fragment to paste into a mail client's signature settings. */
	html: string;
	/** For clients (or clipboard targets) that only take plain text. */
	text: string;
}

function esc(s: string): string {
	return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!);
}

function absolute(path: string): string {
	return new URL(path, location.origin).href;
}

/** Fits an image inside maxW × maxH, keeping its shape; height-only when the size is unknown. */
function fit(size: ImageSize | null | undefined, maxW: number, maxH: number): { w: number | null; h: number } {
	if (!size || !size.width || !size.height) return { w: null, h: maxH };
	const scale = Math.min(maxW / size.width, maxH / size.height, 1);
	return { w: Math.round(size.width * scale), h: Math.round(size.height * scale) };
}

function img(src: string, alt: string, box: { w: number | null; h: number }, style = ''): string {
	const w = box.w ? ` width="${box.w}"` : '';
	const wCss = box.w ? `width:${box.w}px;` : '';
	return `<img src="${esc(src)}" alt="${esc(alt)}"${w} height="${box.h}" style="display:block;border:0;outline:none;${wCss}height:${box.h}px;${style}">`;
}

function a(href: string, text: string, color: string, extra = ''): string {
	return `<a href="${esc(href)}" style="color:${color};text-decoration:none;${extra}">${esc(text)}</a>`;
}

interface Item {
	text: string;
	href: string | null;
}

/** Everything the templates lay out, already filtered by the card's choices and the org's policy. */
interface Parts {
	name: string;
	/** "Title, Company" (or whichever is set). */
	role: string;
	title: string;
	company: string;
	accent: string;
	photo: string | null;
	logo: { src: string; alt: string; size: ImageSize | null } | null;
	contacts: Item[];
	socials: Item[];
	cardLink: string | null;
	banner: { src: string; href: string | null; size: ImageSize | null } | null;
	disclaimer: string;
}

function collect(card: CardData, slug: string, org: OrgBranding | null, opts: RenderOptions, template: SignatureTemplateKey): Parts {
	const s = card.signature;
	const sig = org?.signature ?? {};

	const contacts: Item[] = [];
	if (s.show_phone && card.phone_number) {
		contacts.push({ text: formatPhone(card.phone_country_code, card.phone_number), href: `tel:${e164(card.phone_country_code, card.phone_number)}` });
	}
	if (s.show_email && card.email.trim()) {
		contacts.push({ text: card.email.trim(), href: `mailto:${card.email.trim()}` });
	}
	const website = safeUrl(card.website);
	if (s.show_website && website) contacts.push({ text: displayUrl(website), href: website });
	if (s.show_location && card.location.trim()) contacts.push({ text: card.location.trim(), href: null });
	const booking = safeUrl(card.calendar_url);
	if (s.show_booking && booking) contacts.push({ text: 'Book a meeting', href: booking });

	const socials: Item[] = s.show_socials
		? card.links.flatMap((l) => {
				const url = safeUrl(l.url);
				return url ? [{ text: linkLabel(l), href: url }] : [];
			})
		: [];

	const company = card.company.trim() || org?.name || '';
	const logo =
		org?.logo_file && showsOrgLogo(org, s.show_logo)
			? { src: absolute(fileUrl(org.logo_file)), alt: org.name, size: opts.logoSize ?? null }
			: null;
	const bannerHref = sig.banner_url ? safeUrl(sig.banner_url) : null;

	return {
		name: card.name.trim() || 'Your name',
		title: card.title.trim(),
		company,
		role: [card.title.trim(), company].filter(Boolean).join(', '),
		accent: sig.brand_color || ACCENT_HEX[card.accent] || ACCENT_HEX.indigo,
		photo: SIGNATURE_TEMPLATES[template].photo && s.show_photo ? avatarSrc(card) : null,
		logo,
		contacts,
		socials,
		cardLink: s.show_card_link && slug ? publicUrl(slug) : null,
		banner: sig.banner_file ? { src: absolute(fileUrl(sig.banner_file)), href: bannerHref, size: opts.bannerSize ?? null } : null,
		disclaimer: sig.disclaimer?.trim() ?? ''
	};
}

function itemHtml(i: Item, color: string): string {
	return i.href ? a(i.href, i.text, color) : esc(i.text);
}

function joined(items: Item[], color: string, sep = ' &nbsp;|&nbsp; '): string {
	return items.map((i) => itemHtml(i, color)).join(`<span style="color:#d1d5db;">${sep}</span>`);
}

function logoHtml(p: Parts, maxH: number): string {
	// Logos are square; the width cap only matters for older, wider uploads.
	return p.logo ? img(p.logo.src, p.logo.alt, fit(p.logo.size, maxH * 3, maxH)) : '';
}

function photoHtml(p: Parts, size: number): string {
	return p.photo ? img(p.photo, p.name, { w: size, h: size }, 'border-radius:50%;') : '';
}

const table = (inner: string, style = '') =>
	`<table cellpadding="0" cellspacing="0" border="0" role="presentation" style="border-collapse:collapse;${style}">${inner}</table>`;

const BASE = `font-family:${FONT};font-size:13px;line-height:1.5;color:${TEXT};`;

function row(html: string, style = ''): string {
	return `<tr><td style="${style}">${html}</td></tr>`;
}

function classic(p: Parts): string {
	const lines = [
		row(`<span style="font-size:16px;font-weight:bold;color:${TEXT};">${esc(p.name)}</span>`),
		p.role ? row(esc(p.role), `color:${MUTED};padding-bottom:6px;`) : '',
		...p.contacts.map((c) => row(itemHtml(c, TEXT))),
		p.socials.length ? row(joined(p.socials, p.accent), 'padding-top:4px;') : '',
		p.cardLink ? row(a(p.cardLink, 'View my digital card →', p.accent, 'font-weight:bold;'), 'padding-top:4px;') : '',
		p.logo ? row(logoHtml(p, 36), 'padding-top:10px;') : ''
	].join('');
	const photo = p.photo ? `<td style="vertical-align:top;padding-right:14px;">${photoHtml(p, 72)}</td>` : '';
	return table(
		`<tr>${photo}<td style="vertical-align:top;padding-left:14px;border-left:2px solid ${p.accent};">${table(lines, BASE)}</td></tr>`,
		BASE
	);
}

function corporate(p: Parts): string {
	const head = p.photo
		? table(
				`<tr><td style="vertical-align:middle;padding-right:12px;">${photoHtml(p, 48)}</td><td style="vertical-align:middle;">${table(
					row(`<span style="font-size:16px;font-weight:bold;color:${p.accent};">${esc(p.name)}</span>`) +
						(p.title ? row(esc(p.title), `color:${MUTED};`) : '')
				, BASE)}</td></tr>`,
				BASE
			)
		: table(
				row(`<span style="font-size:16px;font-weight:bold;color:${p.accent};">${esc(p.name)}</span>`) +
					(p.title ? row(esc(p.title), `color:${MUTED};`) : ''),
				BASE
			);
	const lines = [
		p.logo ? row(logoHtml(p, 48), 'padding-bottom:10px;') : '',
		row(head),
		p.company ? row(`<span style="font-weight:bold;">${esc(p.company)}</span>`, 'padding-top:2px;') : '',
		row('', `border-top:1px solid ${p.accent};font-size:0;line-height:0;height:1px;padding-top:8px;`),
		p.contacts.length ? row(joined(p.contacts, TEXT), 'padding-top:8px;') : '',
		p.socials.length ? row(joined(p.socials, p.accent), 'padding-top:2px;') : '',
		p.cardLink ? row(a(p.cardLink, 'View my digital card →', p.accent, 'font-weight:bold;'), 'padding-top:4px;') : ''
	].join('');
	return table(lines, BASE + 'min-width:320px;');
}

function compact(p: Parts): string {
	const first = `<span style="font-weight:bold;">${esc(p.name)}</span>${p.role ? `<span style="color:${MUTED};"> · ${esc(p.role)}</span>` : ''}`;
	const items = [...p.contacts, ...(p.cardLink ? [{ text: 'My card', href: p.cardLink }] : [])];
	const second = joined(items, p.accent);
	const logo = p.logo ? `<td style="vertical-align:middle;padding-right:10px;">${logoHtml(p, 32)}</td>` : '';
	const text = table(row(first) + (second ? row(second, 'font-size:12px;') : ''), BASE);
	return table(`<tr>${logo}<td style="vertical-align:middle;">${text}</td></tr>`, BASE);
}

function bold(p: Parts): string {
	const button = p.cardLink
		? row(
				table(
					`<tr><td bgcolor="${p.accent}" style="background:${p.accent};border-radius:4px;padding:6px 14px;">${a(p.cardLink, 'View my card', '#ffffff', 'font-weight:bold;font-size:12px;')}</td></tr>`
				),
				'padding-top:10px;'
			)
		: '';
	const lines = [
		row(`<span style="font-size:20px;font-weight:bold;color:${TEXT};">${esc(p.name)}</span>`),
		p.title
			? row(esc(p.title.toUpperCase()), `color:${p.accent};font-size:11px;font-weight:bold;letter-spacing:1px;`)
			: '',
		p.company ? row(esc(p.company), `color:${MUTED};padding-bottom:6px;`) : '',
		...p.contacts.map((c) => row(itemHtml(c, TEXT))),
		p.socials.length ? row(joined(p.socials, p.accent), 'padding-top:4px;') : '',
		button,
		p.logo ? row(logoHtml(p, 40), 'padding-top:12px;') : ''
	].join('');
	const photo = p.photo ? `<td style="vertical-align:top;padding-right:16px;">${photoHtml(p, 80)}</td>` : '';
	return table(
		`<tr><td width="4" bgcolor="${p.accent}" style="width:4px;background:${p.accent};font-size:0;line-height:0;">&nbsp;</td><td style="padding-left:14px;"></td>${photo}<td style="vertical-align:top;">${table(lines, BASE)}</td></tr>`,
		BASE
	);
}

function minimal(p: Parts): string {
	const items = [...p.contacts, ...p.socials, ...(p.cardLink ? [{ text: 'My card', href: p.cardLink }] : [])];
	const lines = [
		row(`<span style="font-weight:bold;">${esc(p.name)}</span>`),
		p.role ? row(esc(p.role), `color:${MUTED};`) : '',
		items.length ? row(joined(items, MUTED, ' &nbsp;·&nbsp; '), 'font-size:12px;padding-top:2px;') : '',
		p.logo ? row(logoHtml(p, 24), 'padding-top:8px;') : ''
	].join('');
	return table(lines, BASE);
}

const LAYOUTS: Record<SignatureTemplateKey, (p: Parts) => string> = { classic, corporate, compact, bold, minimal };

/** The banner and disclaimer the organisation puts under every signature. */
function footer(p: Parts): string {
	const parts: string[] = [];
	if (p.banner) {
		// 600 px is the usual email width; a 4:1 banner shows at 600 × 150.
		const box = fit(p.banner.size, 600, 200);
		const image = img(p.banner.src, '', box);
		parts.push(row(p.banner.href ? `<a href="${esc(p.banner.href)}" style="text-decoration:none;">${image}</a>` : image, 'padding-top:12px;'));
	}
	if (p.disclaimer) {
		const text = esc(p.disclaimer).replace(/\r?\n/g, '<br>');
		parts.push(row(text, `padding-top:12px;font-size:10px;line-height:1.4;color:#9ca3af;max-width:520px;`));
	}
	return parts.length ? table(parts.join(''), `font-family:${FONT};`) : '';
}

function plainText(p: Parts): string {
	const lines = [
		'--',
		p.name,
		p.role,
		...p.contacts.map((c) => c.text),
		...p.socials.map((s) => `${s.text}: ${s.href}`),
		p.cardLink ? `My card: ${p.cardLink}` : '',
		p.banner?.href ?? '',
		p.disclaimer ? `\n${p.disclaimer}` : ''
	];
	return lines.filter(Boolean).join('\n');
}

/**
 * The template a card's signature uses: the organisation's locked one when
 * set, else the card's choice.
 */
export function signatureTemplate(card: CardData, org: OrgBranding | null): SignatureTemplateKey {
	const locked = org?.signature.locked_template;
	return isSignatureTemplate(locked) ? locked : card.signature.template;
}

export function renderSignature(card: CardData, slug: string, org: OrgBranding | null, opts: RenderOptions = {}): RenderedSignature {
	const template = opts.template ?? signatureTemplate(card, org);
	const p = collect(card, slug, org, opts, template);
	const html = `<div style="font-family:${FONT};">${LAYOUTS[template](p)}${footer(p)}</div>`;
	return { template, html, text: plainText(p) };
}

/** A standalone page holding the signature, for the .html download and previews. */
export function signatureDocument(html: string, background = '#ffffff'): string {
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Email signature</title></head><body style="margin:0;padding:16px;background:${background};">${html}</body></html>`;
}

/** Loads an image to learn its natural size; null if it can't be loaded. */
export function measureImage(src: string): Promise<ImageSize | null> {
	return new Promise((resolve) => {
		const image = new Image();
		image.onload = () => resolve({ width: image.naturalWidth, height: image.naturalHeight });
		image.onerror = () => resolve(null);
		image.src = src;
	});
}
