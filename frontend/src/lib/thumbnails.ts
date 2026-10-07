/**
 * Small previews made in the browser before an upload, so the library can
 * show thumbnails without loading full images or PDFs. Everything here is
 * best effort: if a preview can't be made, the upload goes ahead without one.
 */

/** Longest side of a preview, in pixels. */
const THUMB_SIZE = 480;
/** The server takes previews up to 300 KB; stay well under. */
const MAX_THUMB_BYTES = 280 * 1024;

export interface Preview {
	thumb: Blob | null;
	width?: number;
	height?: number;
	pages?: number;
}

function toBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob | null> {
	return new Promise((resolve) => canvas.toBlob(resolve, type, quality));
}

/** Encodes a canvas as WebP (JPEG where WebP isn't supported) within the size limit. */
async function encode(canvas: HTMLCanvasElement): Promise<Blob | null> {
	for (const quality of [0.8, 0.6, 0.4]) {
		let blob = await toBlob(canvas, 'image/webp', quality);
		// Older Safari returns PNG when asked for WebP.
		if (!blob || blob.type !== 'image/webp') blob = await toBlob(canvas, 'image/jpeg', quality);
		if (blob && blob.size <= MAX_THUMB_BYTES) return blob;
	}
	return null;
}

/** A canvas sized to fit width×height within THUMB_SIZE. */
function fitCanvas(width: number, height: number): HTMLCanvasElement {
	const scale = Math.min(1, THUMB_SIZE / Math.max(width, height));
	const canvas = document.createElement('canvas');
	canvas.width = Math.max(1, Math.round(width * scale));
	canvas.height = Math.max(1, Math.round(height * scale));
	return canvas;
}

async function imagePreview(file: File): Promise<Preview> {
	const bitmap = await createImageBitmap(file);
	try {
		const canvas = fitCanvas(bitmap.width, bitmap.height);
		const ctx = canvas.getContext('2d');
		if (!ctx) return { thumb: null, width: bitmap.width, height: bitmap.height };
		ctx.imageSmoothingQuality = 'high';
		ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
		return { thumb: await encode(canvas), width: bitmap.width, height: bitmap.height };
	} finally {
		bitmap.close();
	}
}

async function pdfPreview(file: File): Promise<Preview> {
	// pdf.js is large; only load it when a PDF is actually uploaded.
	const [pdfjs, { default: workerUrl }] = await Promise.all([
		import('pdfjs-dist'),
		import('pdfjs-dist/build/pdf.worker.min.mjs?url')
	]);
	pdfjs.GlobalWorkerOptions.workerSrc = workerUrl;
	const task = pdfjs.getDocument({ data: new Uint8Array(await file.arrayBuffer()) });
	const doc = await task.promise;
	try {
		const page = await doc.getPage(1);
		const base = page.getViewport({ scale: 1 });
		const scale = THUMB_SIZE / Math.max(base.width, base.height);
		const viewport = page.getViewport({ scale });
		const canvas = document.createElement('canvas');
		canvas.width = Math.ceil(viewport.width);
		canvas.height = Math.ceil(viewport.height);
		const ctx = canvas.getContext('2d');
		if (!ctx) return { thumb: null, pages: doc.numPages };
		// PDFs are transparent where nothing is drawn; previews show white paper.
		ctx.fillStyle = '#fff';
		ctx.fillRect(0, 0, canvas.width, canvas.height);
		await page.render({ canvas, canvasContext: ctx, viewport }).promise;
		return { thumb: await encode(canvas), pages: doc.numPages };
	} finally {
		await task.destroy();
	}
}

/** A preview and the size (images) or page count (PDFs) of a file, or null if it can't be read. */
export async function makeThumb(file: File): Promise<Preview | null> {
	try {
		if (file.type === 'application/pdf' || /\.pdf$/i.test(file.name)) return await pdfPreview(file);
		if (file.type.startsWith('image/')) return await imagePreview(file);
	} catch (e) {
		console.warn('preview failed', e);
	}
	return null;
}
