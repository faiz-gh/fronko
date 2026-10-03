// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}

	interface Window {
		/** Set by /config.js at runtime; see docker/40-runtime-config.sh. */
		__FRONKO_CONFIG__?: { apiUrl?: string };
	}
}

export {};
