import { apiClient } from '$lib/core/api';

export type FeedbackCategory = 'bug' | 'idea' | 'other';

export const CATEGORY_LABEL: Record<FeedbackCategory, string> = { bug: 'Bug', idea: 'Idea', other: 'Other' };

export interface NewFeedback {
	category: FeedbackCategory;
	/** 1–5, or null to leave it out. */
	rating: number | null;
	message: string;
	/** The page they were on, for context. */
	page_path: string;
}

/** Sends product feedback to the people who run Fronko. */
export function sendFeedback(feedback: NewFeedback): Promise<{ id: number }> {
	return apiClient<{ id: number }>('/api/me/feedback', { method: 'POST', body: JSON.stringify(feedback) });
}
