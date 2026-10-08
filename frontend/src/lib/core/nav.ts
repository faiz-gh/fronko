import type { Component } from 'svelte';
import ChartLineIcon from '@lucide/svelte/icons/chart-line';
import FolderIcon from '@lucide/svelte/icons/folder';
import IdCardIcon from '@lucide/svelte/icons/id-card';
import InboxIcon from '@lucide/svelte/icons/inbox';
import LayoutGridIcon from '@lucide/svelte/icons/layout-grid';
import PlugIcon from '@lucide/svelte/icons/plug';
import SignatureIcon from '@lucide/svelte/icons/signature';
import UsersIcon from '@lucide/svelte/icons/users';
import UsersRoundIcon from '@lucide/svelte/icons/users-round';
import { cards } from '$lib/features/cards/store.svelte';
import { orgUsers } from '$lib/features/orgs/users.svelte';
import { teams } from '$lib/features/teams/store.svelte';
import { session } from './session.svelte';

/** One entry in the dashboard sidebar. */
export interface NavItem {
	/** The sidebar heading the entry sits under. */
	group: 'Cards' | 'Tools' | 'Organisation';
	href: string;
	label: string;
	icon: Component<{ class?: string }>;
	/** Who sees the entry; everyone signed in when omitted. Read reactively. */
	visible?: () => boolean;
	/** Whether the entry is the current page; an exact path match when omitted. */
	active?: (path: string, routeId: string | null) => boolean;
	/** A badge count; zero or omitted shows none. Read reactively. */
	count?: () => number;
}

/**
 * The dashboard sidebar, top to bottom. A feature that adds a page adds its
 * entry here. Settings is pinned to the bottom of the sidebar separately.
 */
export const DASHBOARD_NAV: NavItem[] = [
	{ group: 'Cards', href: '/dashboard', label: 'Overview', icon: LayoutGridIcon },
	// Admins and team leads look after other people's cards, so cards get
	// their own page; members keep their (few) cards listed in the sidebar.
	{
		group: 'Cards',
		href: '/dashboard/cards',
		label: 'Cards',
		icon: IdCardIcon,
		visible: () => session.seesOthers,
		active: (path, routeId) => path === '/dashboard/cards' || routeId === '/dashboard/[id]',
		count: () => cards.list?.length ?? 0
	},
	{
		group: 'Cards',
		href: '/dashboard/leads',
		label: 'Leads',
		icon: InboxIcon,
		count: () => cards.list?.reduce((sum, p) => sum + p.lead_count, 0) ?? 0
	},
	{ group: 'Cards', href: '/dashboard/analytics', label: 'Analytics', icon: ChartLineIcon },
	{ group: 'Tools', href: '/dashboard/signatures', label: 'Email signatures', icon: SignatureIcon },
	{ group: 'Tools', href: '/dashboard/files', label: 'Files', icon: FolderIcon },
	{
		group: 'Tools',
		href: '/dashboard/integrations',
		label: 'Integrations',
		icon: PlugIcon,
		active: (path) => path.startsWith('/dashboard/integrations')
	},
	{
		group: 'Organisation',
		href: '/dashboard/users',
		label: 'People',
		icon: UsersIcon,
		visible: () => session.isAdmin,
		active: (path) => path.startsWith('/dashboard/users'),
		count: () => orgUsers.people.length
	},
	{
		group: 'Organisation',
		href: '/dashboard/teams',
		label: 'Teams',
		icon: UsersRoundIcon,
		visible: () => session.isAdmin || session.teams.length > 0,
		active: (path) => path.startsWith('/dashboard/teams'),
		count: () => teams.list?.length ?? 0
	}
];
