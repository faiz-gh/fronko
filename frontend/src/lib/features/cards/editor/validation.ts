import { isValidPhone } from '$lib/core/phone';
import { isValidSlug, safeUrl, type CardData } from '../card';
import { qrContrastIssue } from '../qr';

/** Which fields of a card being edited are invalid; any of them blocks saving. */
export interface CardErrors {
	slug: boolean;
	avatar: boolean;
	website: boolean;
	email: boolean;
	phone: boolean;
	calendar: boolean;
	qr: boolean;
}

export function cardErrors(card: CardData, slug: string): CardErrors {
	return {
		slug: !isValidSlug(slug),
		// A pasted URL only matters when no library photo is set.
		avatar: !card.avatar_file && !!card.avatar_url.trim() && !safeUrl(card.avatar_url),
		website: !!card.website.trim() && !safeUrl(card.website),
		email: !!card.email.trim() && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(card.email.trim()),
		phone: !!card.phone_number && !isValidPhone(card.phone_country_code, card.phone_number),
		calendar: !!card.calendar_url.trim() && !safeUrl(card.calendar_url),
		// Colours too close to scan can't be saved.
		qr: !!qrContrastIssue(card.qr)?.blocking
	};
}

export function hasErrors(e: CardErrors): boolean {
	return Object.values(e).some(Boolean);
}
