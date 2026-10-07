import { apiClient, apiUrl } from '$lib/core/api';
import type { Activity, Catalog, Connection, OrgDomain, Scope, SettingsChange, TestResult } from './types';

export function getCatalog(): Promise<Catalog> {
	return apiClient<Catalog>('/api/integrations/catalog');
}

/** The connections the user manages, optionally for one provider. */
export function listConnections(provider?: string): Promise<Connection[]> {
	const q = provider ? `?${new URLSearchParams({ provider })}` : '';
	return apiClient<Connection[]>(`/api/integrations/connections${q}`);
}

export function getConnection(id: number): Promise<Connection> {
	return apiClient<Connection>(`/api/integrations/connections/${id}`);
}

export function createConnection(
	input: { provider: string; scope: Scope; name?: string } & SettingsChange
): Promise<Connection> {
	return apiClient<Connection>('/api/integrations/connections', { method: 'POST', body: JSON.stringify(input) });
}

export function updateConnection(
	id: number,
	input: Partial<SettingsChange> & { name?: string; enabled?: boolean }
): Promise<Connection> {
	return apiClient<Connection>(`/api/integrations/connections/${id}`, { method: 'PATCH', body: JSON.stringify(input) });
}

export function deleteConnection(id: number): Promise<void> {
	return apiClient<void>(`/api/integrations/connections/${id}`, { method: 'DELETE' });
}

export function testConnection(id: number): Promise<TestResult> {
	return apiClient<TestResult>(`/api/integrations/connections/${id}/test`, { method: 'POST' });
}

export function listActivity(id: number, before?: number, limit = 20): Promise<Activity[]> {
	const q = new URLSearchParams({ limit: String(limit) });
	if (before) q.set('before', String(before));
	return apiClient<Activity[]>(`/api/integrations/connections/${id}/activity?${q}`);
}

/** Where the browser goes to authorise an OAuth connection; the server redirects on. */
export function oauthStartUrl(id: number): string {
	return apiUrl(`/api/integrations/connections/${id}/oauth/start`);
}

/** Generates the token a provider calls Fronko with, revoking the previous one. Shown only now. */
export function rotateToken(id: number): Promise<{ token: string; connection: Connection }> {
	return apiClient(`/api/integrations/connections/${id}/tokens`, { method: 'POST' });
}

export function listDomains(): Promise<OrgDomain[]> {
	return apiClient<OrgDomain[]>('/api/org/domains');
}

export function addDomain(domain: string): Promise<OrgDomain> {
	return apiClient<OrgDomain>('/api/org/domains', { method: 'POST', body: JSON.stringify({ domain }) });
}

/** Checks the TXT record; fails with a message when it isn't there yet. */
export function verifyDomain(id: number): Promise<OrgDomain> {
	return apiClient<OrgDomain>(`/api/org/domains/${id}/verify`, { method: 'POST' });
}

export function deleteDomain(id: number): Promise<void> {
	return apiClient<void>(`/api/org/domains/${id}`, { method: 'DELETE' });
}
