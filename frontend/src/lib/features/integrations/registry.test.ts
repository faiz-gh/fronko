import { describe, expect, it } from 'vitest';
import { catalogBadge, initialsOf, logoFor, matchesSearch } from './registry';
import type { CatalogEntry, ConnectionSummary } from './types';

function entry(over: Partial<CatalogEntry> = {}): CatalogEntry {
	return {
		id: 'webhook',
		name: 'Webhook',
		category: 'lead_sync',
		description: 'Send leads anywhere',
		scopes: ['org', 'user'],
		auth: 'none',
		status: 'available',
		multiple: true,
		fields: [],
		keywords: ['zapier'],
		unavailable: '',
		testable: true,
		connections: [],
		...over
	};
}

const conn = (over: Partial<ConnectionSummary> = {}): ConnectionSummary => ({
	id: 1,
	name: 'x',
	scope: 'org',
	status: 'active',
	enabled: true,
	...over
});

describe('catalogBadge', () => {
	it('shows nothing for an available provider with no connections', () => {
		expect(catalogBadge(entry())).toBeNull();
	});

	it('flags coming soon and unavailable providers', () => {
		expect(catalogBadge(entry({ status: 'coming_soon' }))?.label).toBe('Coming soon');
		expect(catalogBadge(entry({ unavailable: 'Needs PUBLIC_URL' }))?.label).toBe('Unavailable');
		expect(catalogBadge(entry({ status: 'beta' }))?.label).toBe('Beta');
	});

	it('puts problems first, then counts what works', () => {
		expect(catalogBadge(entry({ connections: [conn(), conn({ id: 2, status: 'error' })] }))).toEqual({
			label: 'Needs attention',
			tone: 'danger'
		});
		expect(catalogBadge(entry({ connections: [conn()] }))?.label).toBe('Connected');
		expect(catalogBadge(entry({ connections: [conn(), conn({ id: 2 })] }))?.label).toBe('2 active');
		expect(catalogBadge(entry({ connections: [conn({ status: 'pending' })] }))?.label).toBe('Setup incomplete');
		expect(catalogBadge(entry({ connections: [conn({ enabled: false })] }))?.label).toBe('Paused');
		expect(catalogBadge(entry({ connections: [conn({ enabled: false, status: 'error' })] }))?.label).toBe('Paused');
	});
});

describe('search and logos', () => {
	it('matches names, descriptions and keywords', () => {
		expect(matchesSearch(entry(), '')).toBe(true);
		expect(matchesSearch(entry(), 'ZAP')).toBe(true);
		expect(matchesSearch(entry(), 'anywhere')).toBe(true);
		expect(matchesSearch(entry(), 'salesforce')).toBe(false);
	});

	it('falls back to initials on a neutral tile', () => {
		expect(initialsOf('Microsoft Dynamics 365')).toBe('MD');
		expect(initialsOf('monday.com')).toBe('MC');
		expect(initialsOf('Okta')).toBe('OK');
		expect(logoFor('unknown').color).toBe('#71717a');
		expect(logoFor('hubspot').path).toBeTruthy();
	});
});
