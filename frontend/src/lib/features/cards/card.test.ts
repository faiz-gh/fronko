import { describe, expect, it } from 'vitest';
import {
	cardPath,
	detectBrand,
	detectCalendar,
	displayUrl,
	emptyCard,
	initials,
	isValidSlug,
	linkLabel,
	MAX_DOCUMENTS,
	normalizeCard,
	normalizeQrStyle,
	QR_IMAGE_SCALE,
	safeUrl,
	slugify,
	tapUrl
} from './card';

describe('normalizeCard', () => {
	it('keeps a whole-number booking page id and drops anything else', () => {
		expect(normalizeCard({}).booking_connection_id).toBeNull();
		expect(normalizeCard({ booking_connection_id: 12 }).booking_connection_id).toBe(12);
		expect(normalizeCard({ booking_connection_id: '12' as never }).booking_connection_id).toBeNull();
		expect(normalizeCard({ booking_connection_id: 1.5 }).booking_connection_id).toBeNull();
	});
	it('fills every field for empty or junk data', () => {
		const fresh = emptyCard();
		for (const raw of [undefined, null, 'x', 42, {}]) {
			expect({ ...normalizeCard(raw), blocks: [] }).toEqual({ ...fresh, blocks: [] });
		}
	});

	it('round-trips a normalised card', () => {
		const card = normalizeCard({ name: 'Ada', links: [{ id: 'l1', label: 'Site', url: 'https://ada.dev' }] });
		expect(normalizeCard(JSON.parse(JSON.stringify(card)))).toEqual(card);
	});

	it('upgrades links stored as plain strings', () => {
		const { links } = normalizeCard({
			links: ['https://github.com/ada', 7, { url: 'https://ada.dev', label: 'Site' }]
		});
		expect(links.map(({ label, url }) => ({ label, url }))).toEqual([
			{ label: '', url: 'https://github.com/ada' },
			{ label: 'Site', url: 'https://ada.dev' }
		]);
		expect(links.every((l) => l.id)).toBe(true);
	});

	it('splits the old free-text phone into dial code and number', () => {
		const card = normalizeCard({ phone: '+44 20 7946 0958' });
		expect([card.phone_country, card.phone_country_code, card.phone_number]).toEqual(['GB', '+44', '2079460958']);
	});

	it('prefers the split phone fields over the old one', () => {
		const card = normalizeCard({
			phone: '+1 415 555 2671',
			phone_country: 'IN',
			phone_country_code: '+91',
			phone_number: '9876543210'
		});
		expect([card.phone_country, card.phone_country_code, card.phone_number]).toEqual(['IN', '+91', '9876543210']);
	});

	it('drops documents without a file and caps the count', () => {
		const documents = [{ title: 'missing' }, ...Array.from({ length: 15 }, (_, i) => ({ file: `d${i}` }))];
		expect(normalizeCard({ documents }).documents).toHaveLength(MAX_DOCUMENTS);
		expect(normalizeCard({ documents }).documents[0].file).toBe('d0');
	});

	it('rejects unknown choices', () => {
		const card = normalizeCard({
			accent: 'neon',
			theme: 'sepia',
			template: 'poster',
			tap: { nfc: 'explode', qr: 'lead_form' }
		});
		expect(card.accent).toBe('indigo');
		expect(card.theme).toBe('light');
		expect(card.template).toBe('classic');
		expect(card.tap).toEqual({ nfc: 'profile', qr: 'lead_form' });
	});

	it('keeps lead collection and the org logo on unless switched off', () => {
		expect(normalizeCard({}).collect_leads).toBe(true);
		expect(normalizeCard({ collect_leads: false, show_org_logo: false })).toMatchObject({
			collect_leads: false,
			show_org_logo: false
		});
		expect(normalizeCard({ collect_leads: 'no' }).collect_leads).toBe(true);
	});
});

describe('normalizeQrStyle', () => {
	it('accepts valid colours and lowercases them', () => {
		expect(normalizeQrStyle({ fg: '#AABBCC', bg: 'red' })).toMatchObject({ fg: '#aabbcc', bg: '#ffffff' });
	});

	it('clamps the centre image size', () => {
		expect(normalizeQrStyle({ image_scale: 1 }).image_scale).toBe(QR_IMAGE_SCALE.max);
		expect(normalizeQrStyle({ image_scale: 0 }).image_scale).toBe(QR_IMAGE_SCALE.min);
		expect(normalizeQrStyle({ image_scale: Number.NaN }).image_scale).toBe(QR_IMAGE_SCALE.default);
	});

	it('falls back to no image when a custom image has no file', () => {
		expect(normalizeQrStyle({ image: 'custom' }).image).toBe('none');
		expect(normalizeQrStyle({ image: 'custom', image_file: 'f1' }).image).toBe('custom');
	});
});

describe('safeUrl', () => {
	it('adds https to bare hosts', () => {
		expect(safeUrl('  ada.dev/about ')).toBe('https://ada.dev/about');
	});

	it('only lets http and https through', () => {
		expect(safeUrl('http://ada.dev')).toBe('http://ada.dev/');
		for (const bad of ['javascript:alert(1)', 'JaVaScRiPt:alert(1)', 'data:text/html,hi', 'mailto:a@b.c', '', '   ']) {
			expect(safeUrl(bad)).toBeNull();
		}
	});

	it('rejects text that is not an address', () => {
		for (const bad of ['not a url', 'https://not a url', 'hello', 'https://ada', 'ada.', 'https://%20.com']) {
			expect(safeUrl(bad)).toBeNull();
		}
		expect(safeUrl('http://localhost:3000/p')).toBe('http://localhost:3000/p');
		expect(safeUrl('https://192.168.1.10/x')).toBe('https://192.168.1.10/x');
		expect(safeUrl('sub.ada-lovelace.co.uk')).toBe('https://sub.ada-lovelace.co.uk/');
	});
});

describe('link helpers', () => {
	it('shortens URLs for display', () => {
		expect(displayUrl('https://www.ada.dev/about/')).toBe('ada.dev/about');
		expect(displayUrl('not a url at all')).toBe('not a url at all');
	});

	it('recognises brands by host, including subdomains', () => {
		expect(detectBrand('https://www.linkedin.com/in/ada')?.name).toBe('LinkedIn');
		expect(detectBrand('twitter.com/ada')?.name).toBe('X');
		expect(detectBrand('https://gist.github.com/ada')?.name).toBe('GitHub');
	});

	it('does not match look-alike hosts', () => {
		expect(detectBrand('https://notgithub.com/ada')).toBeNull();
		expect(detectBrand('https://github.com.evil.example/ada')).toBeNull();
		expect(detectBrand('javascript:github.com')).toBeNull();
	});

	it('recognises booking pages', () => {
		expect(detectCalendar('https://calendly.com/ada/30min')?.name).toBe('Calendly');
		expect(detectCalendar('https://cal.com/ada')?.name).toBe('Cal.com');
		expect(detectCalendar('https://ada.dev/book')).toBeNull();
	});

	it('labels links by their own label, then brand, then URL', () => {
		expect(linkLabel({ id: '1', label: ' Code ', url: 'https://github.com/ada' })).toBe('Code');
		expect(linkLabel({ id: '1', label: '', url: 'https://github.com/ada' })).toBe('GitHub');
		expect(linkLabel({ id: '1', label: '', url: 'https://ada.dev/' })).toBe('ada.dev');
	});
});

describe('names and slugs', () => {
	it('takes initials from the first and last name', () => {
		expect(initials('Ada King Lovelace')).toBe('AL');
		expect(initials('  ada ')).toBe('A');
		expect(initials('', 'zed')).toBe('Z');
	});

	it('slugifies names', () => {
		expect(slugify('  Zoë  Ångström — Sales ')).toBe('zoe-angstrom-sales');
		expect(slugify('x'.repeat(100))).toHaveLength(48);
	});

	it('validates slugs', () => {
		expect(isValidSlug('ada-lovelace')).toBe(true);
		for (const bad of ['ab', '-ada', 'ada-', 'ada--l', 'Ada', 'a'.repeat(49)]) expect(isValidSlug(bad)).toBe(false);
	});
});

describe('card links', () => {
	it('escapes the organisation handle and slug', () => {
		expect(cardPath('acme co', 'a/b')).toBe('acme%20co/a%2Fb');
	});

	it('marks tap links with their source', () => {
		expect(tapUrl('acme', 'ada', 'nfc')).toBe('https://cards.example.com/p/acme/ada?via=nfc');
	});
});
