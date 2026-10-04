import {
	AsYouType,
	getCountries,
	getCountryCallingCode,
	getExampleNumber,
	isValidPhoneNumber,
	parsePhoneNumberFromString,
	type CountryCode
} from 'libphonenumber-js';
import examples from 'libphonenumber-js/mobile/examples';

/**
 * Phone numbers are stored as two digit-only parts: a dial code ("+91") and the
 * national number ("9876543210"). The ISO country is kept alongside on cards
 * because several countries share a dial code (+1: US, CA, …).
 */
export interface Country {
	code: CountryCode;
	name: string;
	dial: string;
	flag: string;
}

function flagEmoji(code: string): string {
	return String.fromCodePoint(...[...code].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65));
}

let cached: Country[] | null = null;

/** Every country libphonenumber knows, sorted by display name. */
export function countries(): Country[] {
	if (cached) return cached;
	let names: Intl.DisplayNames | null = null;
	try {
		names = new Intl.DisplayNames(['en'], { type: 'region' });
	} catch {
		// Very old browsers: fall back to the ISO code.
	}
	cached = getCountries()
		.map((code) => ({
			code,
			name: names?.of(code) ?? code,
			dial: `+${getCountryCallingCode(code)}`,
			flag: flagEmoji(code)
		}))
		.sort((a, b) => a.name.localeCompare(b.name));
	return cached;
}

export function isCountry(code: string): code is CountryCode {
	return (getCountries() as string[]).includes(code);
}

export function dialCode(country: string): string {
	return isCountry(country) ? `+${getCountryCallingCode(country)}` : '';
}

/** A best guess at the visitor's country from the browser locale. */
export function defaultCountry(): CountryCode {
	try {
		for (const lang of navigator.languages ?? [navigator.language]) {
			const region = new Intl.Locale(lang).maximize().region;
			if (region && isCountry(region)) return region;
		}
	} catch {
		// ignore
	}
	return 'US';
}

export function digits(input: string): string {
	return input.replace(/\D/g, '');
}

/** Formats the national part as the user types, e.g. "98765 43210". */
export function formatNational(country: string, number: string): string {
	if (!number) return '';
	if (!isCountry(country)) return number;
	return new AsYouType(country).input(number);
}

/** A sample mobile number for the placeholder, digits only. */
export function exampleNumber(country: string): string {
	if (!isCountry(country)) return '';
	return String(getExampleNumber(country, examples)?.nationalNumber ?? '');
}

/** True when both parts form a valid number for that dial code. */
export function isValidPhone(code: string, number: string): boolean {
	if (!code || !number) return false;
	return isValidPhoneNumber(`${code}${number}`);
}

/** "+919876543210", for tel: links and vCards. Empty when there is no number. */
export function e164(code: string, number: string): string {
	if (!number) return '';
	return `${code}${number}`;
}

/** "+91 98765 43210" for display; falls back to the raw parts. */
export function formatPhone(code: string, number: string): string {
	if (!number) return '';
	const parsed = parsePhoneNumberFromString(`${code}${number}`);
	return parsed ? parsed.formatInternational() : `${code} ${number}`.trim();
}

/**
 * Splits an old free-text phone ("+1 555 010 0000") into the stored parts.
 * Numbers without a "+" can't be placed reliably, so the digits are kept and
 * the dial code is left for the user to pick.
 */
export function parseLegacyPhone(input: string): { country: string; code: string; number: string } {
	const raw = input.trim();
	if (!raw) return { country: '', code: '', number: '' };
	const parsed = raw.startsWith('+') ? parsePhoneNumberFromString(raw) : undefined;
	if (parsed) {
		return {
			country: parsed.country ?? '',
			code: `+${parsed.countryCallingCode}`,
			number: String(parsed.nationalNumber)
		};
	}
	return { country: '', code: '', number: digits(raw) };
}
