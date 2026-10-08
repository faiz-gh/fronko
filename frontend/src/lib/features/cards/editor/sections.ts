import CalendarCheckIcon from '@lucide/svelte/icons/calendar-check';
import FileTextIcon from '@lucide/svelte/icons/file-text';
import LayoutTemplateIcon from '@lucide/svelte/icons/layout-template';
import LinkIcon from '@lucide/svelte/icons/link';
import PaletteIcon from '@lucide/svelte/icons/palette';
import PhoneIcon from '@lucide/svelte/icons/phone';
import QrCodeIcon from '@lucide/svelte/icons/qr-code';
import Share2Icon from '@lucide/svelte/icons/share-2';
import SmartphoneNfcIcon from '@lucide/svelte/icons/smartphone-nfc';
import UserIcon from '@lucide/svelte/icons/user';
import type { RailItem } from '$lib/components/shared/section-rail.svelte';
import { plural } from '$lib/core/format';
import type { Booking } from '../api';
import { isPreset, TEMPLATES } from '../blocks';
import { MAX_DOCUMENTS, type CardData } from '../card';
import type { CardErrors } from './validation';

/**
 * The card editor's sections, in rail order: what's on the card, how it
 * looks, then how it's shared. One shows at a time; the URL hash (#contact)
 * remembers which, so links can open a given section.
 */
export const SECTIONS = [
	'profile',
	'contact',
	'booking',
	'links',
	'brochures',
	'layout',
	'appearance',
	'sharing',
	'qr',
	'nfc'
] as const;
export type SectionId = (typeof SECTIONS)[number];

export function sectionFrom(hash: string): SectionId {
	const id = hash.slice(1);
	return (SECTIONS as readonly string[]).includes(id) ? (id as SectionId) : 'profile';
}

/** Which sections hold an invalid field. */
export function sectionErrors(e: CardErrors): Record<SectionId, boolean> {
	return {
		profile: e.avatar,
		contact: e.website || e.email || e.phone,
		booking: false,
		links: e.links.length > 0,
		brochures: false,
		layout: false,
		appearance: false,
		qr: e.qr,
		sharing: e.slug,
		nfc: false
	};
}

/** Whether the block layout was changed from its template's. */
export function isCustomised(card: CardData): boolean {
	return !isPreset(card.blocks, card.template);
}

/** The rail entries, each with a one-line summary of what's filled in. */
export function railItems(
	card: CardData,
	link: string,
	errors: Record<SectionId, boolean>,
	bookings: Booking[]
): RailItem<SectionId>[] {
	// Location is edited under About you, so only the Contact section's own fields count.
	const contactFields = [card.email, card.phone_number, card.website];
	const contacts = contactFields.filter((v) => v.trim()).length;
	const links = card.links.filter((l) => l.url.trim()).length;
	const hasPhoto = !!(card.avatar_file || card.avatar_url.trim());
	const booking = bookings.find((b) => b.connection_id === card.booking_connection_id);
	const items: Omit<RailItem<SectionId>, 'error'>[] = [
		{
			id: 'profile',
			group: 'Content',
			label: 'About you',
			icon: UserIcon,
			summary: card.name.trim() ? `${card.name.trim()}${hasPhoto ? '' : ' · no photo'}` : 'Add your name'
		},
		{
			id: 'contact',
			group: 'Content',
			label: 'Contact details',
			icon: PhoneIcon,
			summary: contacts ? `${contacts} of ${contactFields.length} filled` : 'Nothing yet'
		},
		{
			id: 'booking',
			group: 'Content',
			label: 'Booking',
			icon: CalendarCheckIcon,
			summary: booking
				? booking.label || booking.name
				: card.booking_connection_id !== null
					? 'Page unavailable'
					: 'Not shown'
		},
		{
			id: 'links',
			group: 'Content',
			label: 'Links',
			icon: LinkIcon,
			summary: links ? plural(links, 'link') : 'None yet'
		},
		{
			id: 'brochures',
			group: 'Content',
			label: 'Brochures',
			icon: FileTextIcon,
			summary: `${card.documents.length} of ${MAX_DOCUMENTS}`
		},
		{
			id: 'layout',
			group: 'Design',
			label: 'Template & blocks',
			icon: LayoutTemplateIcon,
			summary: `${TEMPLATES[card.template].label}${isCustomised(card) ? ' · customised' : ''}`
		},
		{
			id: 'appearance',
			group: 'Design',
			label: 'Colours & logo',
			icon: PaletteIcon,
			summary: `${card.accent[0].toUpperCase()}${card.accent.slice(1)} · ${card.theme === 'dark' ? 'Dark' : 'Light'}`
		},
		{ id: 'sharing', group: 'Share', label: 'Link & tap', icon: Share2Icon, summary: link },
		{
			id: 'qr',
			group: 'Share',
			label: 'QR code',
			icon: QrCodeIcon,
			summary:
				card.qr.image === 'org_logo'
					? 'With logo'
					: card.qr.image === 'custom'
						? 'With image'
						: card.qr.dots === 'square' && card.qr.fg === '#0a0a0a'
							? 'Plain'
							: 'Styled'
		},
		{
			id: 'nfc',
			group: 'Share',
			label: 'NFC & signature',
			icon: SmartphoneNfcIcon,
			summary: 'NFC card · email signature'
		}
	];
	return items.map((i) => ({ ...i, error: errors[i.id] }));
}
