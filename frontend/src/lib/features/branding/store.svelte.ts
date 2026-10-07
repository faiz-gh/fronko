import { getBranding, type OrgBranding } from '$lib/features/orgs/api';

/** The organisation's logo and signature settings, shared by Settings, the editor and Signatures. */
class BrandingState {
	value = $state<OrgBranding | null>(null);
	error = $state('');

	#owner: string | null = null;
	#loading: Promise<void> | null = null;

	load(owner: string, force = false): Promise<void> {
		if (owner !== this.#owner) {
			this.#owner = owner;
			this.value = null;
			this.#loading = null;
		}
		if (force) this.#loading = null;
		this.#loading ??= (async () => {
			this.error = '';
			try {
				this.value = await getBranding();
			} catch (e) {
				this.error = e instanceof Error ? e.message : 'Failed to load branding';
				this.#loading = null;
			}
		})();
		return this.#loading;
	}

	set(value: OrgBranding) {
		this.value = value;
	}
}

export const branding = new BrandingState();
