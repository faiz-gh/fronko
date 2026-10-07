import { getCatalog } from './api';
import type { Catalog, CatalogEntry } from './types';

/**
 * The integrations catalog with the user's connection badges. Shared by the
 * catalog and provider pages; refresh it after connections change.
 */
class Integrations {
	catalog = $state<Catalog | null>(null);
	error = $state('');

	#owner: string | null = null;
	#loading: Promise<void> | null = null;

	byId(id: string): CatalogEntry | undefined {
		return this.catalog?.providers.find((p) => p.id === id);
	}

	/** Loads once per signed-in user; pass `force` to refetch. */
	load(owner: string, force = false): Promise<void> {
		if (owner !== this.#owner) {
			this.#owner = owner;
			this.catalog = null;
			this.#loading = null;
		}
		if (force) this.#loading = null;
		this.#loading ??= (async () => {
			this.error = '';
			try {
				this.catalog = await getCatalog();
			} catch (e) {
				this.error = e instanceof Error ? e.message : 'Failed to load integrations';
				this.#loading = null;
			}
		})();
		return this.#loading;
	}

	refresh() {
		if (this.#owner) return this.load(this.#owner, true);
	}
}

export const integrations = new Integrations();
