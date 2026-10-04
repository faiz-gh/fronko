import { apiClient } from './client';

export type StorageProvider = 'r2' | 'b2' | 's3' | 'minio' | 'other';

/**
 * What the server tells us about the organisation's bucket. Key material is
 * never returned, and only the owner gets the bucket details.
 */
export interface StorageStatus {
	/** False when the server has no SECRETS_KEY, so uploads are switched off. */
	enabled: boolean;
	configured: boolean;
	/** The signed-in user's personal files... */
	used_bytes: number;
	/** ...against their limit; null is unlimited. */
	quota_bytes: number | null;
	provider?: StorageProvider;
	endpoint?: string;
	region?: string;
	bucket?: string;
	path_style: boolean;
	/** Last 4 characters of the access key ID. */
	access_key_hint?: string;
	verified_at?: string;
	file_count: number;
}

export interface StorageInput {
	provider: StorageProvider;
	endpoint: string;
	region: string;
	bucket: string;
	path_style: boolean;
	/** Leave blank when editing to keep the saved key. */
	access_key_id: string;
	/** Leave blank when editing to keep the saved secret. */
	secret_access_key: string;
}

export function getStorage(): Promise<StorageStatus> {
	return apiClient<StorageStatus>('/api/me/storage');
}

/** Checks the settings against the bucket, then saves them. */
export function saveStorage(input: StorageInput): Promise<StorageStatus> {
	return apiClient<StorageStatus>('/api/me/storage', { method: 'PUT', body: JSON.stringify(input) });
}

export function testStorage(input: StorageInput): Promise<{ ok: boolean }> {
	return apiClient<{ ok: boolean }>('/api/me/storage/test', { method: 'POST', body: JSON.stringify(input) });
}

export function disconnectStorage(): Promise<void> {
	return apiClient<void>('/api/me/storage', { method: 'DELETE' });
}
