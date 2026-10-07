import { getMyProfiles, type Profile } from './api';

/**
 * The signed-in user's cards, shared by the dashboard sidebar and pages so a
 * rename or delete shows up everywhere without refetching.
 */
class Cards {
	list = $state<Profile[] | null>(null);
	error = $state('');
	/** Opens the global "New card" dialog. */
	createOpen = $state(false);

	#owner: string | null = null;
	#loading: Promise<void> | null = null;

	/** Loads once per signed-in user; pass `force` to refetch. */
	load(owner: string, force = false): Promise<void> {
		if (owner !== this.#owner) {
			this.#owner = owner;
			this.list = null;
			this.#loading = null;
		}
		if (force) this.#loading = null;
		this.#loading ??= (async () => {
			this.error = '';
			try {
				this.list = await getMyProfiles();
			} catch (e) {
				this.error = e instanceof Error ? e.message : 'Failed to load your cards';
				this.#loading = null;
			}
		})();
		return this.#loading;
	}

	/** Inserts a new card or replaces an existing one, keeping its lead count. */
	upsert(profile: Profile) {
		if (!this.list) return;
		const i = this.list.findIndex((p) => p.id === profile.id);
		if (i === -1) this.list.unshift(profile);
		else this.list[i] = { ...profile, lead_count: this.list[i].lead_count };
	}

	remove(id: number) {
		if (this.list) this.list = this.list.filter((p) => p.id !== id);
	}
}

export const cards = new Cards();
