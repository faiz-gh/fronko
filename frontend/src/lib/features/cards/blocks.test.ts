import { describe, expect, it } from 'vitest';
import {
	applyTemplate,
	isPreset,
	MAX_BLOCKS,
	MAX_GALLERY_IMAGES,
	MAX_HEADING,
	normalizeBlocks,
	TEMPLATES,
	templateBlocks
} from './blocks';

const types = (blocks: { type: string }[]) => blocks.map((b) => b.type);

describe('normalizeBlocks', () => {
	it('gives cards saved before layouts the Classic preset', () => {
		expect(types(normalizeBlocks(undefined))).toEqual(TEMPLATES.classic.blocks);
		expect(types(normalizeBlocks('nonsense'))).toEqual(TEMPLATES.classic.blocks);
	});

	it('drops unknown and malformed blocks', () => {
		const blocks = normalizeBlocks([{ type: 'header' }, { type: 'marquee' }, null, 'bio', { type: 'bio' }]);
		expect(types(blocks)).toEqual(['header', 'bio']);
	});

	it('keeps one of each field block but any number of content blocks', () => {
		const blocks = normalizeBlocks([
			{ type: 'header' },
			{ type: 'links' },
			{ type: 'text', text: 'a' },
			{ type: 'links' },
			{ type: 'text', text: 'b' }
		]);
		expect(types(blocks)).toEqual(['header', 'links', 'text', 'text']);
	});

	it('moves the header first, adds one when missing, and never hides it', () => {
		expect(types(normalizeBlocks([{ type: 'bio' }, { type: 'header', hidden: true }]))).toEqual(['header', 'bio']);
		expect(normalizeBlocks([{ type: 'header', hidden: true }])[0].hidden).toBeUndefined();
		expect(types(normalizeBlocks([{ type: 'bio' }]))).toEqual(['header', 'bio']);
	});

	it('keeps ids and the hidden flag', () => {
		const [, bio] = normalizeBlocks([{ type: 'header' }, { id: 'b1', type: 'bio', hidden: true }]);
		expect(bio).toEqual({ id: 'b1', type: 'bio', hidden: true });
	});

	it('falls back to the banner header for an unknown style', () => {
		expect(normalizeBlocks([{ type: 'header', style: 'huge' }])[0]).toMatchObject({ style: 'banner' });
		expect(normalizeBlocks([{ type: 'header', style: 'badge' }])[0]).toMatchObject({ style: 'badge' });
	});

	it('trims text to its limit', () => {
		const [, heading] = normalizeBlocks([{ type: 'header' }, { type: 'heading', text: 'x'.repeat(500) }]);
		expect(heading).toMatchObject({ type: 'heading', text: 'x'.repeat(MAX_HEADING) });
	});

	it('drops gallery images without a file and caps the count', () => {
		const images = [{ caption: 'no file' }, ...Array.from({ length: 20 }, (_, i) => ({ file: `f${i}` }))];
		const [, gallery] = normalizeBlocks([{ type: 'header' }, { type: 'gallery', images }]);
		expect(gallery.type === 'gallery' && gallery.images.map((i) => i.file)).toEqual(
			Array.from({ length: MAX_GALLERY_IMAGES }, (_, i) => `f${i}`)
		);
	});

	it('caps the number of blocks', () => {
		const many = [{ type: 'header' }, ...Array.from({ length: 40 }, () => ({ type: 'divider' }))];
		expect(normalizeBlocks(many)).toHaveLength(MAX_BLOCKS);
	});
});

describe('templates', () => {
	it('builds every preset with its own header style', () => {
		for (const [key, t] of Object.entries(TEMPLATES)) {
			const blocks = templateBlocks(key as keyof typeof TEMPLATES);
			expect(types(blocks)).toEqual(t.blocks);
			expect(blocks[0]).toMatchObject({ type: 'header', style: t.header });
			expect(isPreset(blocks, key as keyof typeof TEMPLATES)).toBe(true);
		}
	});

	it('stops being a preset once something is typed or hidden', () => {
		const blocks = templateBlocks('event');
		const event = blocks.find((b) => b.type === 'event')!;
		expect(
			isPreset(
				blocks.map((b) => (b === event ? { ...b, name: 'Expo' } : b)),
				'event'
			)
		).toBe(false);
		expect(
			isPreset(
				blocks.map((b, i) => (i === 1 ? { ...b, hidden: true } : b)),
				'event'
			)
		).toBe(false);
		expect(isPreset(blocks, 'classic')).toBe(false);
	});

	it('keeps typed content when switching to a layout that uses it', () => {
		const blocks = templateBlocks('event').map((b) => (b.type === 'event' ? { ...b, name: 'Expo', hidden: true } : b));
		const classic = applyTemplate(blocks, 'classic');
		expect(types(classic)).toEqual(TEMPLATES.classic.blocks);
		const back = applyTemplate(classic.concat(blocks.filter((b) => b.type === 'event')), 'event');
		expect(back.find((b) => b.type === 'event')).toMatchObject({ name: 'Expo', hidden: false });
	});
});
