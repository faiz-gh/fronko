import { listOrgUsers, type OrgUser } from './api';

/**
 * Everyone in the organisation, for admins. Shared by the Users page and the
 * user pickers and filters on cards, leads and files.
 */
class OrgUsers {
	list = $state<OrgUser[] | null>(null);
	error = $state('');

	#owner: string | null = null;
	#loading: Promise<void> | null = null;

	/**
	 * Everyone, the owner included: counted on the Users page and offered when
	 * assigning cards, as in analytics. (The owner sees every file anyway, so
	 * file access lists only members.)
	 */
	get people(): OrgUser[] {
		return this.list ?? [];
	}

	byId(id: number | null | undefined): OrgUser | undefined {
		return id == null ? undefined : this.list?.find((u) => u.id === id);
	}

	/** Loads once per signed-in admin; pass `force` to refetch. */
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
				this.list = await listOrgUsers();
			} catch (e) {
				this.error = e instanceof Error ? e.message : 'Failed to load users';
				this.#loading = null;
			}
		})();
		return this.#loading;
	}

	/** Inserts a new user or replaces an existing one. */
	upsert(user: OrgUser) {
		if (!this.list) return;
		const i = this.list.findIndex((u) => u.id === user.id);
		if (i === -1) this.list.push(user);
		else this.list[i] = user;
	}

	remove(id: number) {
		if (this.list) this.list = this.list.filter((u) => u.id !== id);
	}

	/** Refetch after something changed users' totals (cards, leads, files). */
	refresh() {
		if (this.#owner) return this.load(this.#owner, true);
	}
}

export const orgUsers = new OrgUsers();
