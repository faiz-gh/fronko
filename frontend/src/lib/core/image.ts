/** Canvas helpers for the crop dialog. All work happens in the browser. */

export interface PixelArea {
	x: number;
	y: number;
	width: number;
	height: number;
}

function loadImage(src: string): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const img = new Image();
		img.onload = () => resolve(img);
		img.onerror = () => reject(new Error('Could not read that image'));
		img.src = src;
	});
}

function toBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob | null> {
	return new Promise((resolve) => canvas.toBlob(resolve, type, quality));
}

/** Returns an object URL of `src` turned 90° clockwise. The caller revokes it. */
export async function rotateImage(src: string): Promise<string> {
	const img = await loadImage(src);
	const canvas = document.createElement('canvas');
	canvas.width = img.naturalHeight;
	canvas.height = img.naturalWidth;
	const ctx = canvas.getContext('2d');
	if (!ctx) throw new Error('Your browser cannot edit images');
	ctx.translate(canvas.width, 0);
	ctx.rotate(Math.PI / 2);
	ctx.drawImage(img, 0, 0);
	const blob = await toBlob(canvas, 'image/png', 1);
	if (!blob) throw new Error('Could not rotate that image');
	return URL.createObjectURL(blob);
}

/**
 * Output encoding. WebP is smallest, but email clients (Outlook desktop) can't
 * show it, so images meant for email signatures use PNG (keeps transparency,
 * for logos) or JPEG (photos and banners).
 */
/** A crop shape the user can pick, and the pixel size it is saved at. */
export interface CropRatio {
	label: string;
	aspect: number;
	width: number;
	height: number;
}

export type ImageFormat = 'webp' | 'png' | 'jpeg';

/**
 * Cuts `area` out of `src` and scales it to width×height. WebP falls back to
 * JPEG on browsers that can't encode it (older Safari silently returns PNG).
 */
export async function cropImage(
	src: string,
	area: PixelArea,
	width: number,
	height: number,
	name: string,
	format: ImageFormat = 'webp'
): Promise<File> {
	const img = await loadImage(src);
	const canvas = document.createElement('canvas');
	canvas.width = width;
	canvas.height = height;
	const ctx = canvas.getContext('2d');
	if (!ctx) throw new Error('Your browser cannot edit images');
	ctx.imageSmoothingQuality = 'high';
	ctx.drawImage(img, area.x, area.y, area.width, area.height, 0, 0, width, height);

	let blob: Blob | null = null;
	let ext = 'jpg';
	if (format === 'png') {
		blob = await toBlob(canvas, 'image/png', 1);
		ext = 'png';
	} else if (format === 'webp') {
		blob = await toBlob(canvas, 'image/webp', 0.9);
		ext = 'webp';
	}
	if (!blob || blob.type !== `image/${format}`) {
		blob = await toBlob(canvas, 'image/jpeg', 0.9);
		ext = 'jpg';
	}
	if (!blob) throw new Error('Could not save the cropped image');
	const base = name.replace(/\.[^.]+$/, '') || 'image';
	return new File([blob], `${base}.${ext}`, { type: blob.type });
}
