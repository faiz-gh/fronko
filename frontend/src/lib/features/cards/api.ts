import { apiClient } from '$lib/core/api';
import { cardPath, type CardData } from './card';
import type { PublicFile } from '$lib/features/files/api';

export interface Profile {
	id: number;
	user_id: number;
	/** The one user who works on this card; null means the organisation holds it. */
	assigned_user: { id: number; username: string } | null;
	slug: string;
	data: Partial<CardData> | null;
	created_at: string;
	updated_at: string;
	/** For members, only the leads that arrived while they held the card. */
	lead_count: number;
}

export interface PublicProfile {
	id: number;
	slug: string;
	/** The organisation's part of the card's link, /p/{org_handle}/{slug}. */
	org_handle: string;
	data: Partial<CardData> | null;
	/** Library files the card references (photo, brochures), resolved by the server. */
	files: PublicFile[];
	/** The card's organisation and its logo. */
	org: PublicOrg | null;
}

export interface PublicOrg {
	name: string;
	logo_file: string | null;
	logo_policy: 'required' | 'optional';
}

/** The card behind /p/{org}/{slug}. */
export function getPublicProfile(org: string, slug: string): Promise<PublicProfile> {
	return apiClient<PublicProfile>(`/api/profiles/${cardPath(org, slug)}`);
}

export function getMyProfiles(): Promise<Profile[]> {
	return apiClient<Profile[]>('/api/me/profiles');
}

export function getMyProfile(id: number): Promise<Profile> {
	return apiClient<Profile>(`/api/me/profiles/${id}`);
}

/** Admins only. */
export function createProfile(slug: string, data: CardData, assignedUserId: number | null = null): Promise<Profile> {
	return apiClient<Profile>('/api/me/profiles', {
		method: 'POST',
		body: JSON.stringify({ slug, data, assigned_user_id: assignedUserId })
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
