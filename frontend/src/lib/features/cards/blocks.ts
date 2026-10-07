// Card layout: an ordered list of blocks. Contact details stay on CardData
// (they also feed the vCard); blocks decide what shows and in what order, and
// only the free-content blocks (heading, text, gallery, event) carry data.

export type HeaderStyle = 'banner' | 'badge' | 'compact';

export interface GalleryImage {
	id: string;
	/** Public file id in the library. */
	file: string;
	caption: string;
}

interface Base {
	id: string;
	hidden?: boolean;
}

export type CardBlock =
	| (Base & { type: 'header'; style: HeaderStyle })
	| (Base & { type: 'bio' | 'quick_actions' | 'booking' | 'actions' | 'links' | 'documents' | 'divider' })
	| (Base & { type: 'heading'; text: string })
	| (Base & { type: 'text'; text: string })
	| (Base & { type: 'gallery'; images: GalleryImage[] })
	| (Base & { type: 'event'; name: string; dates: string; venue: string; role: string });

export type BlockType = CardBlock['type'];
export type BlockOf<T extends BlockType> = Extract<CardBlock, { type: T }>;

export const MAX_BLOCKS = 24;
export const MAX_GALLERY_IMAGES = 12;
export const MAX_HEADING = 80;
export const MAX_TEXT = 1000;
export const MAX_EVENT_FIELD = 80;
export const MAX_CAPTION = 120;

/** Blocks that show fields edited elsewhere; a card has at most one of each. */
export const SINGLETON_BLOCKS: BlockType[] = ['header', 'bio', 'quick_actions', 'booking', 'actions', 'links', 'documents'];

export const BLOCK_INFO: Record<BlockType, { label: string; description: string }> = {
	header: { label: 'Header', description: 'Photo, name, title and location' },
	bio: { label: 'Bio', description: 'Your short bio' },
	quick_actions: { label: 'Quick actions', description: 'Email, call and website buttons' },
	booking: { label: 'Book a meeting', description: 'Your booking page' },
	actions: { label: 'Visitor buttons', description: 'Save contact, share your contact, share' },
	links: { label: 'Links', description: 'Your links' },
	documents: { label: 'Brochures', description: 'Your PDF brochures' },
	heading: { label: 'Heading', description: 'A section title' },
	text: { label: 'Text', description: 'A paragraph of your own' },
	gallery: { label: 'Gallery', description: 'A grid of images: work, products, projects' },
	event: { label: 'Event', description: 'Event name, dates, venue and your role' },
	divider: { label: 'Divider', description: 'A thin line between sections' }
};

export const HEADER_STYLES: Record<HeaderStyle, string> = {
	banner: 'Banner',
	badge: 'Badge',
	compact: 'Compact'
};

export type TemplateKey = 'classic' | 'event' | 'portfolio' | 'minimal';

export const TEMPLATES: Record<TemplateKey, { label: string; description: string; header: HeaderStyle; blocks: BlockType[] }> = {
	classic: {
		label: 'Classic',
		description: 'A business card with everything in one place.',
		header: 'banner',
		blocks: ['header', 'bio', 'quick_actions', 'booking', 'actions', 'links', 'documents']
	},
	event: {
		label: 'Event tag',
		description: 'A name badge for conferences and meetups.',
		header: 'badge',
		blocks: ['header', 'event', 'actions', 'quick_actions', 'links']
	},
	portfolio: {
		label: 'Portfolio',
		description: 'Lead with your work: a gallery up front.',
		header: 'banner',
		blocks: ['header', 'bio', 'gallery', 'links', 'documents', 'actions']
	},
	minimal: {
		label: 'Minimal',
		description: 'Just the essentials, no banner.',
		header: 'compact',
		blocks: ['header', 'quick_actions', 'actions', 'links']
	}
};

function newId(): string {
	return Math.random().toString(36).slice(2, 10);
}

function str(v: unknown, max = Infinity): string {
	return typeof v === 'string' ? v.slice(0, max) : '';
}

export function newBlock(type: BlockType, header: HeaderStyle = 'banner'): CardBlock {
	const id = newId();
	switch (type) {
		case 'header':
			return { id, type, style: header };
		case 'heading':
		case 'text':
			return { id, type, text: '' };
		case 'gallery':
			return { id, type, images: [] };
		case 'event':
			return { id, type, name: '', dates: '', venue: '', role: '' };
		default:
			return { id, type };
	}
}

export function templateBlocks(key: TemplateKey): CardBlock[] {
	const t = TEMPLATES[key];
	return t.blocks.map((type) => newBlock(type, t.header));
}

/**
 * The blocks for a new template. Content blocks the new layout also uses keep
 * what was typed into them, so switching back and forth loses nothing.
 */
export function applyTemplate(current: CardBlock[], key: TemplateKey): CardBlock[] {
	const pool = current.filter((b) => b.type === 'heading' || b.type === 'text' || b.type === 'gallery' || b.type === 'event');
	return templateBlocks(key).map((b) => {
		const i = pool.findIndex((p) => p.type === b.type);
		if (i === -1) return b;
		const [kept] = pool.splice(i, 1);
		return { ...kept, hidden: false };
	});
}

/** True when the blocks are exactly a template's preset (nothing to lose by switching). */
export function isPreset(blocks: CardBlock[], key: TemplateKey): boolean {
	const t = TEMPLATES[key];
	return (
		blocks.length === t.blocks.length &&
		blocks.every((b, i) => {
			if (b.type !== t.blocks[i] || b.hidden) return false;
			if (b.type === 'header') return b.style === t.header;
			if (b.type === 'heading' || b.type === 'text') return !b.text;
			if (b.type === 'gallery') return b.images.length === 0;
			if (b.type === 'event') return !b.name && !b.dates && !b.venue && !b.role;
			return true;
		})
	);
}

function normalizeBlock(raw: unknown): CardBlock | null {
	if (!raw || typeof raw !== 'object') return null;
	const o = raw as Record<string, unknown>;
	const type = o.type as BlockType;
	if (!(type in BLOCK_INFO)) return null;
	const base = { id: str(o.id) || newId(), ...(o.hidden === true ? { hidden: true } : {}) };
	switch (type) {
		case 'header':
			return { ...base, type, style: typeof o.style === 'string' && o.style in HEADER_STYLES ? (o.style as HeaderStyle) : 'banner' };
		case 'heading':
			return { ...base, type, text: str(o.text, MAX_HEADING) };
		case 'text':
			return { ...base, type, text: str(o.text, MAX_TEXT) };
		case 'gallery': {
			const images = Array.isArray(o.images)
				? o.images.flatMap((img): GalleryImage[] => {
						if (!img || typeof img !== 'object') return [];
						const i = img as Record<string, unknown>;
						const file = str(i.file);
						return file ? [{ id: str(i.id) || newId(), file, caption: str(i.caption, MAX_CAPTION) }] : [];
					})
				: [];
			return { ...base, type, images: images.slice(0, MAX_GALLERY_IMAGES) };
		}
		case 'event':
			return {
				...base,
				type,
				name: str(o.name, MAX_EVENT_FIELD),
				dates: str(o.dates, MAX_EVENT_FIELD),
				venue: str(o.venue, MAX_EVENT_FIELD),
				role: str(o.role, MAX_EVENT_FIELD)
			};
		default:
			return { ...base, type };
	}
}

/**
 * Coerces stored blocks into a valid layout: unknown blocks dropped, one of
 * each field block, and a header always first. Cards saved before layouts
 * existed get the Classic preset, which matches how they always looked.
 */
export function normalizeBlocks(raw: unknown): CardBlock[] {
	if (!Array.isArray(raw)) return templateBlocks('classic');
	const seen = new Set<BlockType>();
	const blocks: CardBlock[] = [];
	for (const item of raw) {
		const b = normalizeBlock(item);
		if (!b) continue;
		if (SINGLETON_BLOCKS.includes(b.type)) {
			if (seen.has(b.type)) continue;
			seen.add(b.type);
		}
		blocks.push(b);
	}
	const header = blocks.find((b) => b.type === 'header') ?? newBlock('header');
	const rest = blocks.filter((b) => b.type !== 'header').slice(0, MAX_BLOCKS - 1);
	// The header can't be hidden: it carries the name.
	const shown = { ...header };
	delete shown.hidden;
	return [shown, ...rest];
}
