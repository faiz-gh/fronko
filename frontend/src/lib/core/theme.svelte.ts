export type ThemePreference = 'light' | 'dark' | 'system';

/** Also read by the inline script in app.html, which applies the theme before first paint. */
export const THEME_STORAGE_KEY = 'fronko-theme';

function readPreference(): ThemePreference {
	try {
		const stored = localStorage.getItem(THEME_STORAGE_KEY);
		if (stored === 'light' || stored === 'dark' || stored === 'system') return stored;
	} catch {
		// Storage unavailable; fall back to the system setting.
	}
	return 'system';
}

/**
 * The dashboard's colour theme. Only the dashboard applies it: public card
 * pages follow each card's own light/dark setting instead.
 */
class Theme {
	preference = $state<ThemePreference>(readPreference());
	#systemDark = $state(false);

	constructor() {
		const query = matchMedia('(prefers-color-scheme: dark)');
		this.#systemDark = query.matches;
		query.addEventListener('change', (e) => (this.#systemDark = e.matches));
	}

	get dark() {
		return this.preference === 'dark' || (this.preference === 'system' && this.#systemDark);
	}

	set(preference: ThemePreference) {
		this.preference = preference;
		try {
			localStorage.setItem(THEME_STORAGE_KEY, preference);
		} catch {
			// Storage unavailable; the choice lasts for this page load.
		}
	}
}

export const theme = new Theme();
