import { apiUrl } from '$lib/core/api';
import { defaultQrStyle, downloadBlob, type QrStyle } from './card';

// The encoder is only needed once a QR code is shown, so keep it out of the main bundle.
// `qrcode` is CommonJS; depending on interop its API is the default export or the namespace.
async function loadQr(): Promise<typeof import('qrcode')> {
	const mod = await import('qrcode');
	return (mod as unknown as { default?: typeof import('qrcode') }).default ?? mod;
}

/** Quiet zone around the code, in modules: the 4 the standard asks for, so downloads scan on their own. */
const MARGIN = 4;
const PNG_SIZE = 1024;

export interface QrRenderOptions {
	style?: QrStyle;
	/**
	 * Public id of the image for the middle of the code (the organisation logo
	 * or the card's own pick), already resolved from style.image. Ignored when
	 * style.image is 'none'.
	 */
	imageFile?: string | null;
}

/** The file to put in the middle of a card's code, or null for none. */
export function qrImageFile(style: QrStyle, orgLogo: string | null | undefined): string | null {
	if (style.image === 'org_logo') return orgLogo || null;
	if (style.image === 'custom') return style.image_file || null;
	return null;
}

// ---- Colour checks -------------------------------------------------------------

function luminance(hex: string): number {
	const [r, g, b] = [1, 3, 5].map((i) => {
		const c = parseInt(hex.slice(i, i + 2), 16) / 255;
		return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
	});
	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

export function contrastRatio(a: string, b: string): number {
	const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
	return (hi + 0.05) / (lo + 0.05);
}

/**
 * Why the colours might not scan, or null. `blocking` problems stop the card
 * from being saved; the others are warnings.
 */
export function qrContrastIssue(style: QrStyle): { message: string; blocking: boolean } | null {
	const ratio = contrastRatio(style.fg, style.bg);
	if (ratio < 3)
		return {
			message: 'These colours are too close for phones to read the code. Pick a darker or lighter pair.',
			blocking: true
		};
	if (luminance(style.fg) > luminance(style.bg))
		return {
			message: 'Light codes on dark backgrounds don’t scan on some phones. Dark on light is safest.',
			blocking: false
		};
	if (ratio < 4.5)
		return {
			message: 'Low contrast: some phones may struggle to scan this. Test it before printing.',
			blocking: false
		};
	return null;
}

// ---- Centre image --------------------------------------------------------------

const imageCache = new Map<string, Promise<string | null>>();

/**
 * The image as a data URL, so it can live inside the SVG and be drawn onto
 * the PNG canvas. Fetched from this origin (see GET /api/me/files/{id}/image),
 * which needs a signed-in user; null when it can't be read.
 */
function imageDataUrl(fileId: string): Promise<string | null> {
	let cached = imageCache.get(fileId);
	if (!cached) {
		cached = fetch(apiUrl(`/api/me/files/${encodeURIComponent(fileId)}/image`), { credentials: 'include' })
			.then((res) => (res.ok ? res.blob() : null))
			.then(
				(blob) =>
					blob &&
					new Promise<string | null>((resolve) => {
						const reader = new FileReader();
						reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : null);
						reader.onerror = () => resolve(null);
						reader.readAsDataURL(blob);
					})
			)
			.catch(() => null);
		cached.then((v) => v === null && imageCache.delete(fileId));
		imageCache.set(fileId, cached);
	}
	return cached;
}

// ---- Rendering -----------------------------------------------------------------

const f = (n: number) => +n.toFixed(3);

/** A rectangle with per-corner radii (tl, tr, br, bl), as path data. */
function roundedRect(x: number, y: number, w: number, h: number, [tl, tr, br, bl]: number[]): string {
	return (
		`M${f(x + tl)},${f(y)}H${f(x + w - tr)}` +
		(tr ? `A${tr},${tr} 0 0 1 ${f(x + w)},${f(y + tr)}` : '') +
		`V${f(y + h - br)}` +
		(br ? `A${br},${br} 0 0 1 ${f(x + w - br)},${f(y + h)}` : '') +
		`H${f(x + bl)}` +
		(bl ? `A${bl},${bl} 0 0 1 ${f(x)},${f(y + h - bl)}` : '') +
		`V${f(y + tl)}` +
		(tl ? `A${tl},${tl} 0 0 1 ${f(x + tl)},${f(y)}` : '') +
		'Z'
	);
}

function circle(cx: number, cy: number, r: number): string {
	return `M${f(cx - r)},${f(cy)}a${r},${r} 0 1 0 ${f(r * 2)},0a${r},${r} 0 1 0 ${f(-r * 2)},0Z`;
}

/** One finder pattern ("eye") with its top-left module at x, y. */
function eye(x: number, y: number, style: QrStyle['corners']): string {
	switch (style) {
		case 'dot':
			// Ring as two circles (even-odd fill), then the pupil.
			return circle(x + 3.5, y + 3.5, 3.5) + circle(x + 3.5, y + 3.5, 2.5) + circle(x + 3.5, y + 3.5, 1.5);
		case 'rounded':
			return (
				roundedRect(x, y, 7, 7, [2, 2, 2, 2]) +
				roundedRect(x + 1, y + 1, 5, 5, [1.4, 1.4, 1.4, 1.4]) +
				roundedRect(x + 2, y + 2, 3, 3, [0.9, 0.9, 0.9, 0.9])
			);
		default:
			return (
				roundedRect(x, y, 7, 7, [0, 0, 0, 0]) +
				roundedRect(x + 1, y + 1, 5, 5, [0, 0, 0, 0]) +
				roundedRect(x + 2, y + 2, 3, 3, [0, 0, 0, 0])
			);
	}
}

/**
 * Builds the code as an SVG string. The modules come from `qrcode`; we draw
 * them ourselves so the dots, eyes and centre image can be styled. A centre
 * image raises error correction to H (30% may be covered) and clears the
 * modules behind it, so the code still scans.
 */
export async function qrSvg(
	url: string,
	{ style = defaultQrStyle(), imageFile = null }: QrRenderOptions = {}
): Promise<string> {
	const QRCode = await loadQr();
	const image = style.image !== 'none' && imageFile ? await imageDataUrl(imageFile) : null;
	const styled = style.dots !== 'square' || style.corners !== 'square';
	const level = image ? 'H' : styled ? 'Q' : 'M';
	const qr = QRCode.create(url, { errorCorrectionLevel: level });
	const n = qr.modules.size;
	const dark = (r: number, c: number) => r >= 0 && c >= 0 && r < n && c < n && !!qr.modules.get(r, c);

	const inEye = (r: number, c: number) => (r < 7 && c < 7) || (r < 7 && c >= n - 7) || (r >= n - 7 && c < 7);

	// The cleared square for the image: an odd number of modules, centred.
	let hole: { from: number; to: number } | null = null;
	if (image) {
		let span = Math.round(n * style.image_scale);
		if (span % 2 === 0) span += 1;
		const from = (n - span) / 2;
		hole = { from, to: from + span };
	}
	const inHole = (r: number, c: number) =>
		!!hole && r >= hole.from - 0.5 && r < hole.to + 0.5 && c >= hole.from - 0.5 && c < hole.to + 0.5;
	const on = (r: number, c: number) => dark(r, c) && !inEye(r, c) && !inHole(r, c);

	let modules = '';
	for (let r = 0; r < n; r++) {
		for (let c = 0; c < n; c++) {
			if (!on(r, c)) continue;
			const x = c + MARGIN;
			const y = r + MARGIN;
			if (style.dots === 'dots') {
				modules += circle(x + 0.5, y + 0.5, 0.48);
			} else if (style.dots === 'rounded') {
				// Round a corner only where neither neighbour on that corner is dark,
				// so runs of modules join into smooth shapes.
				const up = on(r - 1, c),
					down = on(r + 1, c),
					left = on(r, c - 1),
					right = on(r, c + 1);
				const k = 0.5;
				modules += roundedRect(x, y, 1, 1, [
					!up && !left ? k : 0,
					!up && !right ? k : 0,
					!down && !right ? k : 0,
					!down && !left ? k : 0
				]);
			} else {
				modules += `M${x},${y}h1v1h-1Z`;
			}
		}
	}

	const eyes =
		eye(MARGIN, MARGIN, style.corners) +
		eye(MARGIN + n - 7, MARGIN, style.corners) +
		eye(MARGIN, MARGIN + n - 7, style.corners);
	const size = n + MARGIN * 2;

	let centre = '';
	if (image && hole) {
		const x = hole.from + MARGIN;
		const span = hole.to - hole.from;
		const pad = 0.4;
		centre =
			`<rect x="${f(x - pad)}" y="${f(x - pad)}" width="${f(span + pad * 2)}" height="${f(span + pad * 2)}" rx="${f(span * 0.18)}" fill="${style.bg}"/>` +
			`<image href="${image}" x="${x}" y="${x}" width="${span}" height="${span}" preserveAspectRatio="xMidYMid meet"/>`;
	}

	return (
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${size} ${size}"${style.dots === 'square' && style.corners === 'square' ? ' shape-rendering="crispEdges"' : ''}>` +
		`<rect width="${size}" height="${size}" fill="${style.bg}"/>` +
		`<path fill="${style.fg}" d="${modules}"/>` +
		`<path fill="${style.fg}" fill-rule="evenodd" d="${eyes}"/>` +
		centre +
		`</svg>`
	);
}

/** Draws the SVG onto a canvas; 1024px is large enough for print and NFC card artwork. */
export async function downloadQrPng(svg: string, slug: string) {
	// An intrinsic size, which some browsers need before they'll draw an SVG.
	const sized = svg.replace('<svg ', `<svg width="${PNG_SIZE}" height="${PNG_SIZE}" `);
	const blobUrl = URL.createObjectURL(new Blob([sized], { type: 'image/svg+xml' }));
	try {
		const img = new Image();
		img.width = PNG_SIZE;
		img.height = PNG_SIZE;
		await new Promise<void>((resolve, reject) => {
			img.onload = () => resolve();
			img.onerror = () => reject(new Error('Could not draw the QR code'));
			img.src = blobUrl;
		});
		const canvas = document.createElement('canvas');
		canvas.width = PNG_SIZE;
		canvas.height = PNG_SIZE;
		const ctx = canvas.getContext('2d');
		if (!ctx) throw new Error('Could not draw the QR code');
		ctx.drawImage(img, 0, 0, PNG_SIZE, PNG_SIZE);
		const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'));
		if (!blob) throw new Error('Could not draw the QR code');
		downloadBlob(blob, `${slug}-qr.png`);
	} finally {
		URL.revokeObjectURL(blobUrl);
	}
}

export function downloadQrSvg(svg: string, slug: string) {
	downloadBlob(new Blob([svg], { type: 'image/svg+xml' }), `${slug}-qr.svg`);
}
