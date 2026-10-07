package integrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/events"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
)

// JobPushLead sends one lead to one lead sync connection.
const JobPushLead = "integrations.push_lead"

type pushLeadPayload struct {
	ConnectionID int64 `json:"connection_id"`
	LeadID       int64 `json:"lead_id"`
}

// Subscribe queues lead sync for every new lead.
func (s *Service) Subscribe(b *events.Bus) {
	events.Subscribe(b, s.onLeadCreated)
}

// onLeadCreated runs inside the transaction that stored the lead: it queues
// one job per connection that should get it, the organisation's and the
// card holder's own, so the jobs exist exactly when the lead does.
func (s *Service) onLeadCreated(ctx context.Context, q database.Querier, e leads.Created) error {
	ids, err := s.store.leadSyncTargets(ctx, q, e.OrgID, e.AssignedUserID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		_, err := jobs.Enqueue(ctx, q, JobPushLead, pushLeadPayload{ConnectionID: id, LeadID: e.LeadID}, jobs.Options{
			OrgID:     e.OrgID,
			DedupeKey: fmt.Sprintf("%d:%d", id, e.LeadID),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// JobHandlers runs the jobs this module queues.
func (s *Service) JobHandlers() map[string]jobs.Handler {
	return map[string]jobs.Handler{JobPushLead: s.pushLead}
}

func (s *Service) pushLead(ctx context.Context, job *jobs.Job) error {
	var p pushLeadPayload
	if err := job.Decode(&p); err != nil {
		return err
	}
	c, err := s.store.getConnectionByID(ctx, p.ConnectionID)
	if errors.Is(err, database.ErrNotFound) {
		return nil // deleted since; nothing to do
	}
	if err != nil {
		return err
	}
	// Paused or broken since the lead arrived: skip it. Leads aren't
	// replayed when the connection comes back.
	if !c.Enabled || c.Status != StatusActive {
		return nil
	}
	prov, ok := s.registry.Get(c.Provider)
	if !ok {
		return jobs.Permanent(fmt.Errorf("unknown provider %q", c.Provider))
	}
	pusher, ok := prov.(LeadPusher)
	if !ok {
		return jobs.Permanent(fmt.Errorf("%s doesn't take leads", prov.Manifest().Name))
	}
	details, err := s.leads.SyncDetails(ctx, p.LeadID)
	if errors.Is(err, database.ErrNotFound) {
		return nil // the lead was deleted
	}
	if err != nil {
		return err
	}
	lead := s.toLead(details)

	call, err := s.newCall(ctx, c, prov)
	var res Result
	if err == nil {
		res, err = pusher.PushLead(ctx, call, lead)
	}
	attempt := job.Attempt
	if err == nil {
		if res.Summary == "" {
			res.Summary = "Sent " + lead.Email
		}
		s.logActivity(ctx, c.ID, &Activity{Kind: ActivityPushLead, Outcome: OutcomeSuccess, Summary: res.Summary,
			Detail: res.Detail, LeadID: &lead.ID, Attempt: &attempt})
		return s.store.markSuccess(ctx, c.ID, true)
	}

	final := jobs.IsPermanent(err) || job.Attempt >= job.MaxAttempts
	outcome := OutcomeRetrying
	if final {
		outcome = OutcomeFailed
	}
	s.logActivity(ctx, c.ID, &Activity{Kind: ActivityPushLead, Outcome: outcome, Summary: describeError(err),
		Detail: errorDetail(err), LeadID: &lead.ID, Attempt: &attempt})
	if final {
		status, markErr := s.store.markFailure(context.WithoutCancel(ctx), c.ID, describeError(err), errorThreshold)
		if markErr != nil {
			logf("recording failure of connection %d: %v", c.ID, markErr)
		} else if status == StatusError {
			logf("connection %d (%s) stopped after %d failures in a row", c.ID, c.Provider, errorThreshold)
		}
	}
	return err
}
