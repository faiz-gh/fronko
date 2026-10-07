export type SignatureTemplateKey = 'classic' | 'corporate' | 'compact' | 'bold' | 'minimal';

export interface SignatureTemplate {
	label: string;
	description: string;
	/** Whether the layout has room for the card's photo. */
	photo: boolean;
}

/** Keep in step with signatureTemplates in backend/internal/handlers/branding.go. */
export const SIGNATURE_TEMPLATES: Record<SignatureTemplateKey, SignatureTemplate> = {
	classic: { label: 'Classic', description: 'Photo beside your details, with an accent divider.', photo: true },
	corporate: {
		label: 'Corporate',
		description: 'Logo first, then your name and contact details in a row.',
		photo: true
	},
	compact: { label: 'Compact', description: 'Two short lines of text. Good for replies.', photo: false },
	bold: { label: 'Bold', description: 'Accent bar, large name and a “View my card” button.', photo: true },
	minimal: { label: 'Minimal', description: 'Name, title and links in plain type.', photo: false }
};

export const SIGNATURE_TEMPLATE_KEYS = Object.keys(SIGNATURE_TEMPLATES) as SignatureTemplateKey[];

export function isSignatureTemplate(v: unknown): v is SignatureTemplateKey {
	return typeof v === 'string' && v in SIGNATURE_TEMPLATES;
}

/** A card's email signature choices, stored in its data under `signature`. */
export interface SignatureSettings {
	template: SignatureTemplateKey;
	show_photo: boolean;
	/** Ignored when the organisation requires its logo. */
	show_logo: boolean;
	/** A link to the card's public page. */
	show_card_link: boolean;
	/** The card's links (LinkedIn, GitHub, …). */
	show_socials: boolean;
	show_phone: boolean;
	show_email: boolean;
	show_website: boolean;
	show_location: boolean;
	show_booking: boolean;
}

export function defaultSignature(): SignatureSettings {
	return {
		template: 'classic',
		show_photo: true,
		show_logo: true,
		show_card_link: true,
		show_socials: true,
		show_phone: true,
		show_email: true,
		show_website: true,
		show_location: false,
		show_booking: false
	};
}

export function normalizeSignature(raw: unknown): SignatureSettings {
	const d = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
	const base = defaultSignature();
	const flag = (k: Exclude<keyof SignatureSettings, 'template'>) =>
		typeof d[k] === 'boolean' ? (d[k] as boolean) : base[k];
	return {
		template: isSignatureTemplate(d.template) ? d.template : base.template,
		show_photo: flag('show_photo'),
		show_logo: flag('show_logo'),
		show_card_link: flag('show_card_link'),
		show_socials: flag('show_socials'),
		show_phone: flag('show_phone'),
		show_email: flag('show_email'),
		show_website: flag('show_website'),
		show_location: flag('show_location'),
		show_booking: flag('show_booking')
	};
}
