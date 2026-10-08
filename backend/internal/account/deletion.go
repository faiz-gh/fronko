package account

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// OrgStorage is the part of file storage that account deletion needs (the
// files package): moving the bucket keys to a new owner, and emptying the
// bucket of an organisation that's being deleted.
type OrgStorage interface {
	RekeyFor(fromID, toID int64) func(idEnc, secretEnc []byte) ([]byte, []byte, error)
	DeleteOrgObjects(ctx context.Context, orgID int64) (failed int, err error)
}

// Error codes the dashboard acts on.
const (
	codeTransferRequired = "ownership_transfer_required"
	codeManagedByIdP     = "managed_by_idp"
)

// DeletionInfo tells the dashboard what deleting the account would do, so it
// can warn before anything happens.
type DeletionInfo struct {
	Username    string `json:"username"`
	Role        string `json:"role"`
	HasPassword bool   `json:"has_password"`
	// Managed is set for people the organisation's identity provider manages
	// (SCIM): they're removed there, not here.
	Managed   bool   `json:"managed"`
	OrgName   string `json:"org_name"`
	OrgHandle string `json:"org_handle"`
	// OwnerUsername is who the member's cards and files pass to.
	OwnerUsername string `json:"owner_username,omitempty"`
	// SoleMember is set when the owner is the only person in the
	// organisation: deleting the account deletes the organisation.
	SoleMember bool                   `json:"sole_member"`
	Summary    *users.DeletionSummary `json:"summary"`
	// TransferCandidates are who an owner can hand the organisation to.
	TransferCandidates []auth.UserRef `json:"transfer_candidates"`
}

// Protected: GET /api/me/account/deletion
func (h *AuthHandler) DeletionInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	info, err := h.deletionInfo(r.Context(), user)
	if err != nil {
		log.Printf("deletion info: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, info)
}

func (h *AuthHandler) deletionInfo(ctx context.Context, user *users.User) (*DeletionInfo, error) {
	org, err := h.orgs.GetOrganization(ctx, user.OrgID)
	if err != nil {
		return nil, err
	}
	summary, err := h.users.GetDeletionSummary(ctx, user.ID, user.OrgID)
	if err != nil {
		return nil, err
	}
	info := &DeletionInfo{
		Username:           user.Username,
		Role:               user.Role,
		HasPassword:        user.HasPassword(),
		Managed:            user.ProvisionedBy != nil && *user.ProvisionedBy == users.ProvisionedSCIM,
		OrgName:            org.Name,
		OrgHandle:          org.Handle,
		SoleMember:         user.Role == auth.RoleOwner && summary.OrgMembers <= 1,
		Summary:            summary,
		TransferCandidates: []auth.UserRef{},
	}
	if user.Role == auth.RoleOwner {
		if info.TransferCandidates, err = h.users.ListTransferCandidates(ctx, user.OrgID, user.ID); err != nil {
			return nil, err
		}
	} else {
		owner, err := h.users.GetOrgOwner(ctx, user.OrgID)
		if err != nil {
			return nil, err
		}
		info.OwnerUsername = owner.Username
	}
	return info, nil
}

// Protected: POST /api/me/account/deletion/code. Emails a code that confirms
// deleting (or handing over) an account that has no password.
func (h *AuthHandler) SendDeletionCode(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if user.HasPassword() {
		httpx.WriteError(w, http.StatusBadRequest, "confirm with your password instead")
		return
	}
	wait, err := h.codes.Issue(r.Context(), user, auth.PurposeDeleteAccount, false)
	if err != nil {
		log.Printf("issue delete account code: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to send the code")
		return
	}
	if wait > 0 {
		writeRetryAfter(w, wait)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// reauthenticate checks that the person at the keyboard is the account's
// owner: their password, or for accounts without one, an emailed code.
// It writes the error response and returns false when they aren't.
func (h *AuthHandler) reauthenticate(w http.ResponseWriter, r *http.Request, user *users.User, password, code string) bool {
	if user.HasPassword() {
		if !h.authService.CheckPasswordHash(password, user.PasswordHash) {
			httpx.WriteError(w, http.StatusBadRequest, "password is incorrect")
			return false
		}
		return true
	}
	ok, err := h.codes.Consume(r.Context(), user.ID, auth.PurposeDeleteAccount, code)
	if err != nil {
		log.Printf("check delete account code: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return false
	}
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, errInvalidCode)
		return false
	}
	return true
}

type TransferOwnershipRequest struct {
	UserID   int64  `json:"user_id"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

// Protected (owner): POST /api/me/ownership/transfer. Hands the organisation
// to another member; the owner becomes an admin. Both are signed out
// everywhere, so this session gets a fresh cookie for the new role.
func (h *AuthHandler) TransferOwnership(w http.ResponseWriter, r *http.Request) {
	var req TransferOwnershipRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if user.Role != auth.RoleOwner {
		httpx.WriteError(w, http.StatusForbidden, "only the organisation's owner can hand it over")
		return
	}
	if !h.reauthenticate(w, r, user, req.Password, req.Code) {
		return
	}
	target, err := h.users.GetUserByID(r.Context(), req.UserID)
	if err != nil || target.OrgID != user.OrgID {
		httpx.WriteError(w, http.StatusBadRequest, "choose someone in your organisation")
		return
	}

	err = h.users.TransferOwnership(r.Context(), user.OrgID, user.ID, target.ID, h.storage.RekeyFor(user.ID, target.ID))
	switch {
	case errors.Is(err, database.ErrNotFound):
		httpx.WriteError(w, http.StatusBadRequest, "the new owner must be active and have a verified email")
		return
	case err != nil:
		log.Printf("transfer ownership: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "couldn't hand over the organisation")
		return
	}

	if target.Email != nil {
		org, err := h.orgs.GetOrganization(r.Context(), user.OrgID)
		if err == nil {
			h.codes.SendAsync(*target.Email, mail.OwnershipTransferredNotice(org.Name, user.Username))
		}
	}
	user, ok = h.currentUser(w, r)
	if !ok || !h.signIn(w, user) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
}

type DeleteAccountRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
	// Confirm is the username typed out, or the organisation's handle when
	// deleting the organisation.
	Confirm string `json:"confirm"`
	// DeleteOrg asks an owner to delete the whole organisation with the account.
	DeleteOrg bool `json:"delete_org"`
}

// Protected: DELETE /api/me/account. Permanently deletes the signed-in
// account, straight away:
//   - A member's or admin's cards and files pass to the owner and their leads
//     stay with the organisation, as when an admin removes them.
//   - The owner must first hand the organisation to someone else, or delete
//     it (and everything in it) along with the account.
//
// The person's password (or an emailed code) and the typed confirmation are
// both required. Answers 204 and clears the session cookie.
func (h *AuthHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	var req DeleteAccountRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	info, err := h.deletionInfo(r.Context(), user)
	if err != nil {
		log.Printf("delete account info: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if info.Managed {
		httpx.WriteJSON(w, http.StatusConflict, map[string]string{
			"error": "your account is managed by your organisation's identity provider; ask them to remove you",
			"code":  codeManagedByIdP,
		})
		return
	}
	owner := user.Role == auth.RoleOwner
	deleteOrg := owner && (req.DeleteOrg || info.SoleMember)
	if owner && !deleteOrg {
		httpx.WriteJSON(w, http.StatusConflict, map[string]string{
			"error": "hand the organisation to someone else first, or delete it with your account",
			"code":  codeTransferRequired,
		})
		return
	}

	want := user.Username
	if deleteOrg {
		want = info.OrgHandle
	}
	if !strings.EqualFold(strings.TrimSpace(req.Confirm), want) {
		httpx.WriteError(w, http.StatusBadRequest, "type "+want+" to confirm")
		return
	}
	if !h.reauthenticate(w, r, user, req.Password, req.Code) {
		return
	}

	if deleteOrg {
		// The bucket is outside the database, so empty it while the owner's
		// keys still exist; the organisation's rows go next.
		if failed, err := h.storage.DeleteOrgObjects(r.Context(), user.OrgID); err != nil {
			log.Printf("delete org %d objects: %v", user.OrgID, err)
		} else if failed > 0 {
			log.Printf("delete org %d: %d objects couldn't be removed from the bucket", user.OrgID, failed)
		}
		err = h.orgs.DeleteOrganization(r.Context(), user.OrgID)
	} else {
		err = h.users.DeleteOrgUser(r.Context(), user.ID, user.OrgID)
	}
	if err != nil {
		log.Printf("delete account %d: %v", user.ID, err)
		httpx.WriteError(w, http.StatusInternalServerError, "couldn't delete your account; nothing was removed")
		return
	}
	log.Printf("account %d deleted itself (organisation deleted: %t)", user.ID, deleteOrg)

	if user.Email != nil {
		orgName := ""
		if deleteOrg {
			orgName = info.OrgName
		}
		h.codes.SendAsync(*user.Email, mail.AccountDeletedNotice(user.Username, orgName))
	}
	auth.ClearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// Protected: GET /api/me/export. Downloads everything Fronko holds about the
// signed-in person as JSON (right of access and data portability).
func (h *AuthHandler) Export(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	doc, err := h.users.ExportUserData(r.Context(), user.ID)
	if err != nil {
		log.Printf("export user data: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	export := map[string]any{
		"exported_at": time.Now().UTC(),
		"format":      "fronko-export/1",
		"data":        doc,
	}
	w.Header().Set("Content-Disposition", `attachment; filename="fronko-`+user.Username+`-data.json"`)
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, export)
}
