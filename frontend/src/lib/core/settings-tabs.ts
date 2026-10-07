import type { Component } from 'svelte';
import AccountSettings from '$lib/features/auth/components/account-settings.svelte';
import BrandingSection from '$lib/features/branding/components/branding-section.svelte';
import StorageSettings from '$lib/features/files/components/storage-settings.svelte';
import OrganisationSettings from '$lib/features/orgs/components/organisation-settings.svelte';
import { session } from './session.svelte';

/** One tab on the Settings page, opened with /dashboard/settings?tab={value}. */
export interface SettingsTab {
	value: string;
	label: string;
	component: Component;
	/** Who sees the tab; everyone when omitted. Read reactively. */
	visible?: () => boolean;
}

/**
 * The Settings tabs, left to right. The first is the default. A feature with
 * settings adds a component and an entry here.
 */
export const SETTINGS_TABS: SettingsTab[] = [
	{ value: 'account', label: 'Account', component: AccountSettings },
	{ value: 'organisation', label: 'Organisation', component: OrganisationSettings, visible: () => session.isAdmin },
	{ value: 'branding', label: 'Branding', component: BrandingSection, visible: () => session.isAdmin },
	{ value: 'storage', label: 'Storage', component: StorageSettings }
];
