import type { FileArea, FileQuery } from './api';
import type { TeamRef } from '$lib/features/teams/api';

/**
 * Where files live, as the library shows them. A location filters the
 * listing and, if the user may add files there, says where uploads go.
 */
export interface FileLocation {
	key: string;
	label: string;
	description: string;
	query: Pick<FileQuery, 'area' | 'teamId'>;
	/** Where uploads made here go; null when the user can't add files here. */
	upload: { area: FileArea; teamId?: number } | null;
	/** Teams show their colour. */
	team?: TeamRef;
}

/** A place a file can be moved to. */
export interface MoveTarget {
	key: string;
	label: string;
	area: FileArea;
	teamId?: number;
	team?: TeamRef;
}

interface Who {
	isAdmin: boolean;
	orgName: string;
	/** Every team for admins; the user's own teams (with their role) otherwise. */
	teams: TeamRef[];
	ledTeamIds: number[];
}

export function fileLocations(who: Who): FileLocation[] {
	const teams = who.teams.map<FileLocation>((t) => ({
		key: `team:${t.id}`,
		label: t.name,
		description:
			who.isAdmin || who.ledTeamIds.includes(t.id)
				? `${t.name}’s files. Everyone in the team can use them; leads and admins look after them.`
				: `${t.name}’s files, for everyone in the team.`,
		query: { area: 'team', teamId: t.id },
		upload: who.isAdmin || who.ledTeamIds.includes(t.id) ? { area: 'team', teamId: t.id } : null,
		team: t
	}));
	if (who.isAdmin) {
		return [
			{
				key: 'all',
				label: 'All files',
				description: `Everything in ${who.orgName}’s storage.`,
				query: {},
				upload: { area: 'org' }
			},
			{
				key: 'org',
				label: 'Organisation',
				description: 'Private to admins. Share single files with people or teams as needed.',
				query: { area: 'org' },
				upload: { area: 'org' }
			},
			{
				key: 'shared',
				label: 'Shared',
				description: 'Everyone in the organisation can use these on their cards.',
				query: { area: 'shared' },
				upload: { area: 'shared' }
			},
			...teams,
			{
				key: 'personal',
				label: 'People’s files',
				description: 'What each person uploaded for themselves.',
				query: { area: 'personal' },
				upload: null
			}
		];
	}
	return [
		{ key: 'all', label: 'All files', description: 'Every file you can use.', query: {}, upload: { area: 'personal' } },
		{
			key: 'personal',
			label: 'My files',
			description: 'Your photos and brochures. Only you and your admins see them.',
			query: { area: 'personal' },
			upload: { area: 'personal' }
		},
		{
			key: 'shared',
			label: 'Shared',
			description: `From ${who.orgName}, for everyone to use.`,
			query: { area: 'shared' },
			upload: null
		},
		...teams,
		{
			key: 'granted',
			label: 'Shared with me',
			description: `Files ${who.orgName} gave you or your teams access to.`,
			query: { area: 'granted' },
			upload: null
		}
	];
}

/** Where the user may move files: anywhere but someone's personal files for admins, their teams for leads. */
export function moveTargets(who: Who): MoveTarget[] {
	const teams = who.teams
		.filter((t) => who.isAdmin || who.ledTeamIds.includes(t.id))
		.map<MoveTarget>((t) => ({ key: `team:${t.id}`, label: t.name, area: 'team', teamId: t.id, team: t }));
	if (!who.isAdmin) return teams;
	return [
		{ key: 'org', label: 'Organisation', area: 'org' },
		{ key: 'shared', label: 'Shared', area: 'shared' },
		...teams
	];
}

/** The key of the location a file is in, to compare with move targets. */
export function locationKeyOf(f: { area: FileArea; team?: { id: number } }): string {
	return f.area === 'team' && f.team ? `team:${f.team.id}` : f.area;
}
