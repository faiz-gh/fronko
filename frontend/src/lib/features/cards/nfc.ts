// Writing a card's link onto the NFC sticker inside a physical card.
//
// Only Chrome on Android can write NFC from a web page (Web NFC). Everywhere
// else the dashboard hands over the link for the free NFC Tools app instead.

/** The Web NFC API isn't in TypeScript's DOM types yet; this is the part we use. */
interface NdefRecord {
	recordType: 'url';
	data: string;
}
interface NdefReader {
	write(message: { records: NdefRecord[] }, options?: { signal?: AbortSignal; overwrite?: boolean }): Promise<void>;
}
declare global {
	interface Window {
		NDEFReader?: new () => NdefReader;
	}
}

/**
 * How this device can write an NFC card:
 * - `web-nfc`: right here (Chrome on Android, over HTTPS)
 * - `ios`: an iPhone or iPad; Safari can't write NFC, but NFC Tools can
 * - `android`: an Android phone in a browser without Web NFC; NFC Tools again
 * - `desktop`: a computer; write it from a phone instead
 */
export type NfcSupport = 'web-nfc' | 'ios' | 'android' | 'desktop';

export function nfcSupport(): NfcSupport {
	if (typeof window === 'undefined') return 'desktop';
	if (window.NDEFReader && window.isSecureContext) return 'web-nfc';
	const ua = navigator.userAgent;
	// iPadOS reports itself as a Mac; touch support gives it away.
	if (/iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1)) return 'ios';
	if (/Android/.test(ua)) return 'android';
	return 'desktop';
}

/**
 * Writes `url` as the card's only record, replacing whatever was there.
 * Resolves once the phone has written the tag. The tag is never locked, so it
 * can be rewritten later.
 */
export async function writeNfcUrl(url: string, signal: AbortSignal): Promise<void> {
	if (!window.NDEFReader) throw new DOMException('Web NFC is not available', 'NotSupportedError');
	const reader = new window.NDEFReader();
	await reader.write({ records: [{ recordType: 'url', data: url }] }, { signal, overwrite: true });
}

/** A message for a failed write that says what to do next. */
export function nfcErrorMessage(err: unknown, timedOut = false): string {
	const name = err instanceof DOMException ? err.name : '';
	switch (name) {
		case 'AbortError':
			return timedOut
				? "No card was found. Move it slowly around the back of your phone and try again."
				: 'Writing was cancelled.';
		case 'NotAllowedError':
			return 'Chrome isn’t allowed to use NFC, or this card is locked. Allow NFC for this site in Chrome’s settings, then try again.';
		case 'NotSupportedError':
			return 'This phone can’t write NFC cards. Use “Copy NFC link” with the NFC Tools app instead.';
		case 'NotReadableError':
			return 'NFC is turned off. Turn it on in your phone’s settings (Connected devices → NFC), then try again.';
		case 'NetworkError':
			return 'The card moved away before writing finished. Hold it still against your phone until it says it’s done.';
		default:
			return 'Couldn’t write the card. Hold it still against the back of your phone and try again.';
	}
}
