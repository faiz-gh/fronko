import { getTeam, listTeams, type Team, type TeamDetail } from './api';

/**
 * The organisation's teams: every team for admins, the teams they're in for
 * everyone else. Shared by the Teams pages, pickers and filters.
 */
class Teams {
	list = $state<Team[] | null>(null);
	error = $state('');
	/** Teams loaded with their members, by id. */
	details = $state<Record<number, TeamDetail>>({});

	#owner: string | null = null;
	#loading: Promise<void> | null = null;

	/** Loads a team with its members (cached; pass `force` to refetch). */
	async loadDetail(id: number, force = false): Promise<TeamDetail> {
		if (!force && this.details[id]) return this.details[id];
		const detail = await getTeam(id);
		this.details[id] = detail;
		return detail;
	}

	/** Ids of the people in a team, once its detail is loaded. */
	memberIds(id: number): Set<number> | null {
		const d = this.details[id];
		return d ? new Set(d.members.map((m) => m.id)) : null;
	}

	byId(id: number | null | undefined): Team | undefined {
		return id == null ? undefined : this.list?.find((t) => t.id === id);
	}

	/** Loads once per signed-in user; pass `force` to refetch. */
	load(owner: string, force = false): Promise<void> {
		if (owner !== this.#owner) {
			this.#owner = owner;
			this.list = null;
			this.details = {};
			this.#loading = null;
		}
		if (force) this.#loading = null;
		this.#loading ??= (async () => {
			this.error = '';
			try {
				this.list = await listTeams();
			} catch (e) {
				this.error = e instanceof Error ? e.message : 'Failed to load teams';
				this.#loading = null;
			}
		})();
		return this.#loading;
	}

	upsert(team: Team) {
		if (!this.list) return;
		const i = this.list.findIndex((t) => t.id === team.id);
		if (i === -1) this.list = [...this.list, team].sort((a, b) => a.name.localeCompare(b.name));
		else this.list[i] = team;
	}

	remove(id: number) {
		if (this.list) this.list = this.list.filter((t) => t.id !== id);
		delete this.details[id];
	}

	refresh() {
		if (this.#owner) return this.load(this.#owner, true);
	}
}

export const teams = new Teams();
