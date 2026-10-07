import { getStorage, type StorageStatus } from './storage-api';

/** The signed-in user's storage connection, shared by Settings, Files and the editor. */
class StorageState {
	status = $state<StorageStatus | null>(null);
	error = $state('');

	#owner: string | null = null;
	#loading: Promise<void> | null = null;

	/** True once a bucket is connected and the server has storage enabled. */
	get ready() {
		return !!this.status?.enabled && !!this.status.configured;
	}

	load(owner: string, force = false): Promise<void> {
		if (owner !== this.#owner) {
			this.#owner = owner;
			this.status = null;
			this.#loading = null;
		}
		if (force) this.#loading = null;
		this.#loading ??= (async () => {
			this.error = '';
			try {
				this.status = await getStorage();
			} catch (e) {
				this.error = e instanceof Error ? e.message : 'Failed to load storage settings';
				this.#loading = null;
			}
		})();
		return this.#loading;
	}

	set(status: StorageStatus) {
		this.status = status;
	}
}

export const storage = new StorageState();
