import { apiClient } from './client';
import type { CardData } from '$lib/card/card';
import type { PublicFile } from './files';

export interface Profile {
	id: number;
	user_id: number;
	slug: string;
	data: Partial<CardData> | null;
	created_at: string;
	updated_at: string;
	lead_count: number;
}

export interface PublicProfile {
	id: number;
	slug: string;
	data: Partial<CardData> | null;
	/** Library files the card references (photo, brochures), resolved by the server. */
	files: PublicFile[];
}

export function getProfileBySlug(slug: string): Promise<PublicProfile> {
	return apiClient<PublicProfile>(`/api/profiles/${encodeURIComponent(slug)}`);
}

export function getMyProfiles(): Promise<Profile[]> {
	return apiClient<Profile[]>('/api/me/profiles');
}

export function getMyProfile(id: number): Promise<Profile> {
	return apiClient<Profile>(`/api/me/profiles/${id}`);
}

export function createProfile(slug: string, data: CardData): Promise<Profile> {
	return apiClient<Profile>('/api/me/profiles', {
		method: 'POST',
		body: JSON.stringify({ slug, data })
	});
}

export function updateProfile(id: number, slug: string, data: CardData): Promise<Profile> {
	return apiClient<Profile>(`/api/me/profiles/${id}`, {
		method: 'PUT',
		body: JSON.stringify({ slug, data })
	});
}

export function deleteProfile(id: number): Promise<void> {
	return apiClient<void>(`/api/me/profiles/${id}`, { method: 'DELETE' });
}
