import { downloadBlob } from './card';

// The encoder is only needed once a QR code is shown, so keep it out of the main bundle.
// `qrcode` is CommonJS; depending on interop its API is the default export or the namespace.
async function loadQr(): Promise<typeof import('qrcode')> {
	const mod = await import('qrcode');
	return (mod as unknown as { default?: typeof import('qrcode') }).default ?? mod;
}

// Dark modules on a white quiet zone scan most reliably, so codes ignore the card's theme.
const qrOptions = { errorCorrectionLevel: 'M', margin: 2, color: { dark: '#0a0a0a', light: '#ffffff' } } as const;

export async function qrSvg(url: string): Promise<string> {
	const QRCode = await loadQr();
	return QRCode.toString(url, { ...qrOptions, type: 'svg' });
}

/** 1024px is large enough for print and NFC card artwork. */
export async function downloadQrPng(url: string, slug: string) {
	const QRCode = await loadQr();
	const dataUrl = await QRCode.toDataURL(url, { ...qrOptions, width: 1024 });
	const blob = await (await fetch(dataUrl)).blob();
	downloadBlob(blob, `${slug}-qr.png`);
}

export function downloadQrSvg(svg: string, slug: string) {
	downloadBlob(new Blob([svg], { type: 'image/svg+xml' }), `${slug}-qr.svg`);
}
