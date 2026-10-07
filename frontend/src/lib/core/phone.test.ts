import { describe, expect, it } from 'vitest';
import {
	countries,
	dialCode,
	digits,
	e164,
	formatNational,
	formatPhone,
	isValidPhone,
	parseLegacyPhone
} from './phone';

describe('phone helpers', () => {
	it('keeps digits only', () => {
		expect(digits('+1 (415) 555-2671')).toBe('14155552671');
	});

	it('looks up dial codes by country', () => {
		expect(dialCode('IN')).toBe('+91');
		expect(dialCode('ZZ')).toBe('');
	});

	it('lists countries once each, sorted by name', () => {
		const list = countries();
		expect(new Set(list.map((c) => c.code)).size).toBe(list.length);
		expect(list.find((c) => c.code === 'GB')).toMatchObject({ dial: '+44', flag: '🇬🇧' });
		const names = list.map((c) => c.name);
		expect(names).toEqual([...names].sort((a, b) => a.localeCompare(b)));
	});

	it('formats numbers for display and links', () => {
		expect(formatPhone('+91', '9876543210')).toBe('+91 98765 43210');
		expect(formatPhone('+91', '')).toBe('');
		expect(e164('+91', '9876543210')).toBe('+919876543210');
		expect(e164('+91', '')).toBe('');
		expect(formatNational('ZZ', '123')).toBe('123');
	});

	it('validates the parts together', () => {
		expect(isValidPhone('+91', '9876543210')).toBe(true);
		expect(isValidPhone('+91', '12')).toBe(false);
		expect(isValidPhone('', '9876543210')).toBe(false);
	});
});

describe('parseLegacyPhone', () => {
	it('splits international numbers', () => {
		expect(parseLegacyPhone(' +1 415 555 2671 ')).toEqual({ country: 'US', code: '+1', number: '4155552671' });
	});

	it('keeps the digits of numbers without a dial code', () => {
		expect(parseLegacyPhone('(415) 555-2671')).toEqual({ country: '', code: '', number: '4155552671' });
	});

	it('returns empty parts for nothing', () => {
		expect(parseLegacyPhone('  ')).toEqual({ country: '', code: '', number: '' });
	});
});
