import { describe, expect, it } from 'vitest';
import { normalizeCard, type CardData } from '$lib/features/cards/card';
import type { OrgBranding } from '$lib/features/orgs/api';
import { ACCENT_HEX, darkModeHtml, renderSignature, signatureDocument, signatureTemplate } from './render';
import { SIGNATURE_TEMPLATE_KEYS } from './templates';

const CARD_URL = 'https://cards.example.com/p/acme/ada';

function card(over: Record<string, unknown> = {}): CardData {
	return normalizeCard({
		name: 'Ada Lovelace',
		title: 'Engineer',
		company: 'Acme',
		email: 'ada@acme.test',
		phone_country: 'IN',
		phone_country_code: '+91',
		phone_number: '9876543210',
		website: 'acme.test',
		accent: 'emerald',
		links: [{ id: 'l1', label: '', url: 'https://github.com/ada' }],
		...over
	});
}

function org(over: Partial<OrgBranding> = {}): OrgBranding {
	return { name: 'Acme Org', logo_file: 'logo1', logo_policy: 'optional', signature: {}, ...over };
}

describe('renderSignature', () => {
	it('renders every template with the contact details', () => {
		for (const template of SIGNATURE_TEMPLATE_KEYS) {
			const { html } = renderSignature(card(), CARD_URL, null, { template });
			expect(html, template).toContain('Ada Lovelace');
			expect(html, template).toContain('mailto:ada@acme.test');
			expect(html, template).toContain('tel:+919876543210');
			expect(html, template).not.toMatch(/<style|class=/);
		}
	});

	it('escapes everything the user typed', () => {
		const evil = card({
			name: '<img src=x onerror=alert(1)>',
			title: '"><script>',
			links: [{ label: '<b>', url: 'https://ada.dev/?q="x"' }]
		});
		const { html } = renderSignature(evil, CARD_URL, null);
		expect(html).not.toContain('<img src=x');
		expect(html).not.toContain('<script>');
		expect(html).not.toContain('<b>');
		expect(html).toContain('&lt;img src=x onerror=alert(1)&gt;');
		expect(html).toContain('q=%22x%22');
	});

	it('drops links that are not http(s)', () => {
		const { html } = renderSignature(
			card({ website: 'javascript:alert(1)', links: ['javascript:alert(2)'] }),
			CARD_URL,
			null
		);
		expect(html).not.toContain('javascript:');
	});

	it('follows the card’s show/hide choices', () => {
		const hidden = card({
			signature: { show_email: false, show_phone: false, show_card_link: false, show_socials: false }
		});
		const { html, text } = renderSignature(hidden, CARD_URL, null);
		expect(html).not.toContain('mailto:');
		expect(html).not.toContain('tel:');
		expect(html).not.toContain(CARD_URL);
		expect(text).not.toContain('GitHub');
	});

	it('shows the booking link only when asked', () => {
		const booked = { bookingUrl: 'calendly.com/ada' };
		expect(renderSignature(card(), CARD_URL, null, booked).html).not.toContain('calendly.com');
		const shown = card({ signature: { show_booking: true } });
		expect(renderSignature(shown, CARD_URL, null, booked).html).toContain('https://calendly.com/ada');
		expect(renderSignature(shown, CARD_URL, null).html).not.toContain('Book a meeting');
	});

	it('uses the card accent, or the organisation’s brand colour', () => {
		expect(renderSignature(card(), CARD_URL, null).html).toContain(ACCENT_HEX.emerald);
		const branded = renderSignature(card(), CARD_URL, org({ signature: { brand_color: '#123456' } })).html;
		expect(branded).toContain('#123456');
		expect(branded).not.toContain(ACCENT_HEX.emerald);
	});

	it('shows the organisation logo by policy and by choice', () => {
		const logo = '/api/files/logo1';
		const optedOut = card({ signature: { show_logo: false } });
		expect(renderSignature(card(), CARD_URL, org()).html).toContain(logo);
		expect(renderSignature(optedOut, CARD_URL, org()).html).not.toContain(logo);
		expect(renderSignature(optedOut, CARD_URL, org({ logo_policy: 'required' })).html).toContain(logo);
		expect(renderSignature(card(), CARD_URL, org({ logo_file: null })).html).not.toContain('/api/files/');
	});

	it('sizes the logo from its measured shape', () => {
		const { html } = renderSignature(card(), CARD_URL, org(), {
			template: 'classic',
			logoSize: { width: 400, height: 100 }
		});
		expect(html).toContain('width="108" height="27"');
	});

	it('adds the organisation’s banner and disclaimer', () => {
		const sig = { banner_file: 'banner1', banner_url: 'acme.test/offer', disclaimer: 'Line one\nLine <two>' };
		const { html, text } = renderSignature(card(), CARD_URL, org({ signature: sig }));
		expect(html).toContain('https://cards.example.com/api/files/banner1');
		expect(html).toContain('href="https://acme.test/offer"');
		expect(html).toContain('Line one<br>Line &lt;two&gt;');
		expect(text.endsWith('Line one\nLine <two>')).toBe(true);
	});

	it('writes a plain-text version', () => {
		const { text } = renderSignature(card(), CARD_URL, null);
		expect(text.split('\n')).toEqual([
			'--',
			'Ada Lovelace',
			'Engineer, Acme',
			'+91 98765 43210',
			'ada@acme.test',
			'acme.test',
			'GitHub: https://github.com/ada',
			`My card: ${CARD_URL}`
		]);
	});
});

describe('signatureTemplate', () => {
	it('uses the organisation’s locked template when valid', () => {
		const c = card({ signature: { template: 'bold' } });
		expect(signatureTemplate(c, null)).toBe('bold');
		expect(signatureTemplate(c, org({ signature: { locked_template: 'minimal' } }))).toBe('minimal');
		expect(signatureTemplate(c, org({ signature: { locked_template: 'retro' } }))).toBe('bold');
		expect(renderSignature(c, CARD_URL, org({ signature: { locked_template: 'compact' } })).template).toBe('compact');
	});
});

describe('signatureDocument', () => {
	it('wraps the fragment in a page', () => {
		expect(signatureDocument('<p>x</p>', '#000')).toMatch(
			/^<!doctype html>.*background:#000;"><p>x<\/p><\/body><\/html>$/
		);
	});
});

describe('darkModeHtml', () => {
	it('lightens dark text in every template', () => {
		for (const template of SIGNATURE_TEMPLATE_KEYS) {
			const { html } = renderSignature(card({ accent: 'slate' }), CARD_URL, null, { template });
			const dark = darkModeHtml(html);
			for (const hex of ['#1f2937', '#6b7280', `color:${ACCENT_HEX.slate}`]) {
				expect(dark).not.toContain(hex);
			}
		}
	});

	it('leaves backgrounds alone', () => {
		const html = '<td bgcolor="#334155" style="background:#334155;"><a style="color:#ffffff;">x</a></td>';
		expect(darkModeHtml(html)).toBe(html);
	});
});
