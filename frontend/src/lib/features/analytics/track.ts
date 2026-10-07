import { apiUrl } from '$lib/core/api';
import { cardPath } from '$lib/features/cards/card';

/** How the visitor reached the card: the NFC tag, the QR code, or any other link. */
export type VisitSource = 'nfc' | 'qr' | 'link';

/** What the public card reports. Contact saves and sent forms are counted by the server. */
export type TrackType = 'view' | 'click' | 'scroll' | 'doc_open' | 'gallery_open' | 'form_open' | 'share' | 'leave';

export interface TrackEvent {
	type: TrackType;
	/** A link's URL, a document's file id, or a quick action (email, call, website, booking). */
	target?: string;
	label?: string;
	/** Scroll depth in percent, or time on the card in ms. */
	value?: number;
}

export function visitSource(via: string | null): VisitSource {
	return via === 'nfc' || via === 'qr' ? via : 'link';
}

const FLUSH_DELAY = 5000;
const MAX_BATCH = 20;
const DEPTHS = [25, 50, 75, 100];

function randomId(): string {
	if (typeof crypto.randomUUID === 'function') return crypto.randomUUID();
	// Plain-http dev hosts have no randomUUID; build a v4 UUID by hand.
	const b = crypto.getRandomValues(new Uint8Array(16));
	b[6] = (b[6] & 0x0f) | 0x40;
	b[8] = (b[8] & 0x3f) | 0x80;
	const h = [...b].map((x) => x.toString(16).padStart(2, '0')).join('');
	return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

export type Tracker = ReturnType<typeof createTracker>;

/**
 * Reports what a visitor does on a public card. There are no cookies and
 * nothing is stored: the visit id lives in this tab's memory only, and the
 * server never sees more than an anonymous, daily-rotating hash.
 * Events are batched and sent with sendBeacon, so leaving the page loses nothing.
 */
export function createTracker(org: string, slug: string, source: VisitSource) {
	const session = randomId();
	const endpoint = apiUrl(`/api/profiles/${cardPath(org, slug)}/events`);
	let queue: TrackEvent[] = [];
	let timer: ReturnType<typeof setTimeout> | undefined;
	let started = false;

	// Time the card was actually on screen.
	let visibleMs = 0;
	let visibleSince: number | null = null;
	let reportedMs = 0;
	const depths = new Set<number>();

	function flush() {
		clearTimeout(timer);
		timer = undefined;
		while (queue.length > 0) {
			const body = JSON.stringify({ session, source, events: queue.slice(0, MAX_BATCH) });
			queue = queue.slice(MAX_BATCH);
			// text/plain keeps sendBeacon a "simple" request (no CORS preflight).
			const blob = new Blob([body], { type: 'text/plain' });
			if (!navigator.sendBeacon?.(endpoint, blob)) {
				fetch(endpoint, { method: 'POST', body: blob, keepalive: true, credentials: 'include' }).catch(() => {});
			}
		}
	}

	function track(event: TrackEvent) {
		if (!started) return;
		queue.push(event);
		timer ??= setTimeout(flush, FLUSH_DELAY);
	}

	function reportTime() {
		if (visibleSince !== null) {
			visibleMs += performance.now() - visibleSince;
			visibleSince = null;
		}
		const ms = Math.round(visibleMs);
		// The server keeps the largest value per visit, so only send when it grew.
		if (ms - reportedMs >= 1000) {
			reportedMs = ms;
			track({ type: 'leave', value: ms });
		}
	}

	// Depth only counts once the visitor actually scrolls: a card shorter than
	// the screen is seen in full without anyone engaging with it.
	function onScroll() {
		const doc = document.documentElement;
		const scrollable = doc.scrollHeight - window.innerHeight;
		if (scrollable <= 0) return;
		const pct = ((window.scrollY + window.innerHeight) / doc.scrollHeight) * 100;
		for (const d of DEPTHS) {
			if (pct >= d - 2 && !depths.has(d)) {
				depths.add(d);
				track({ type: 'scroll', value: d });
			}
		}
	}

	function onVisibility() {
		if (document.visibilityState === 'hidden') {
			reportTime();
			flush();
		} else {
			visibleSince = performance.now();
		}
	}

	function onPageHide() {
		reportTime();
		flush();
	}

	/** Clicks on anything marked data-track (see the card blocks). */
	function onClick(e: MouseEvent) {
		const el = (e.target as Element | null)?.closest<HTMLElement>('[data-track]');
		if (!el) return;
		track({
			type: el.dataset.track as TrackType,
			target: el.dataset.trackTarget ?? '',
			label: el.dataset.trackLabel ?? ''
		});
	}

	return {
		session,
		source,
		/** Records the visit and starts watching; call once the card is shown. */
		start(root: HTMLElement | Document = document) {
			if (started) return;
			started = true;
			track({ type: 'view' });
			if (document.visibilityState === 'visible') visibleSince = performance.now();
			window.addEventListener('scroll', onScroll, { passive: true });
			document.addEventListener('visibilitychange', onVisibility);
			window.addEventListener('pagehide', onPageHide);
			root.addEventListener('click', onClick as EventListener, { capture: true });
			root.addEventListener('auxclick', onClick as EventListener, { capture: true });
			// Send the view straight away rather than after the batching delay.
			flush();
		},
		track,
		stop(root: HTMLElement | Document = document) {
			if (!started) return;
			reportTime();
			flush();
			started = false;
			window.removeEventListener('scroll', onScroll);
			document.removeEventListener('visibilitychange', onVisibility);
			window.removeEventListener('pagehide', onPageHide);
			root.removeEventListener('click', onClick as EventListener, { capture: true });
			root.removeEventListener('auxclick', onClick as EventListener, { capture: true });
		}
	};
}
