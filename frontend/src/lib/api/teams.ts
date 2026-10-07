import { apiClient } from './client';
import type { Role } from './auth';
import type { OrgUser } from './org';

/** Leads look after the team's files and see their teammates' cards and leads. */
export type TeamRole = 'lead' | 'member';

export const TEAM_ROLE_LABEL: Record<TeamRole, string> = { lead: 'Lead', member: 'Member' };

/** A team named alongside someone's role in it, or a file in it. */
export interface TeamRef {
	id: number;
	name: string;
	/** #rrggbb, or '' for the default. */
	color: string;
	role?: TeamRole;
}

export interface Team {
	id: number;
	name: string;
	description: string;
	color: string;
	member_count: number;
	lead_count: number;
	file_count: number;
	created_at: string;
	updated_at: string;
}

export interface TeamMember {
	id: number;
	username: string;
	email: string | null;
	/** Their role in the organisation. */
	org_role: Role;
	role: TeamRole;
	added_at: string;
}

export interface TeamDetail extends Team {
	members: TeamMember[];
}

export interface TeamMembership {
	team_id?: number;
	user_id?: number;
	role: TeamRole;
}

/** Colours offered for teams; any #rrggbb works. */
export const TEAM_COLORS = ['#2563eb', '#16a34a', '#e11d48', '#d97706', '#7c3aed', '#0891b2', '#db2777', '#4b5563'];

/** A team's colour, or a neutral default. */
export function teamColor(t: Pick<TeamRef, 'color'> | null | undefined): string {
	return t?.color || '#71717a';
}

/** Admins get every team; everyone else the teams they're in. */
export function listTeams(): Promise<Team[]> {
	return apiClient<Team[]>('/api/org/teams');
}

export function getTeam(id: number): Promise<TeamDetail> {
	return apiClient<TeamDetail>(`/api/org/teams/${id}`);
}

export interface NewTeam {
	name: string;
	description?: string;
	color?: string;
	members?: { user_id: number; role: TeamRole }[];
}

export function createTeam(team: NewTeam): Promise<TeamDetail> {
	return apiClient<TeamDetail>('/api/org/teams', { method: 'POST', body: JSON.stringify(team) });
}

export function updateTeam(id: number, team: { name: string; description: string; color: string }): Promise<TeamDetail> {
	return apiClient<TeamDetail>(`/api/org/teams/${id}`, { method: 'PATCH', body: JSON.stringify(team) });
}

/** The team's files move to the organisation's files. */
export function deleteTeam(id: number): Promise<void> {
	return apiClient<void>(`/api/org/teams/${id}`, { method: 'DELETE' });
}

export function setTeamMembers(id: number, members: { user_id: number; role: TeamRole }[]): Promise<TeamDetail> {
	return apiClient<TeamDetail>(`/api/org/teams/${id}/members`, { method: 'PUT', body: JSON.stringify({ members }) });
}

export function setUserTeams(userId: number, teams: { team_id: number; role: TeamRole }[]): Promise<OrgUser> {
	return apiClient<OrgUser>(`/api/org/users/${userId}/teams`, { method: 'PUT', body: JSON.stringify({ teams }) });
}
