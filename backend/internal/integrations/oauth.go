package integrations

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

// The OAuth flow:
//
//  1. The browser opens /connections/{id}/oauth/start. Fronko redirects it
//     to the provider with a signed state naming the connection and user, and
//     sets a short-lived cookie holding a nonce (and the PKCE verifier).
//  2. The provider sends the browser back to the callback with a code.
//     Fronko checks the state's signature, expiry, user and nonce, then
//     exchanges the code using the connection's own client ID and secret.
//  3. The token is sealed with the connection's secrets, and the browser is
//     sent to the integration's page.
const (
	oauthCallbackPath = "/api/integrations/oauth/callback"
	oauthCookieName   = "fronko_oauth"
	oauthStateTTL     = 10 * time.Minute
)

// ErrOAuthState is returned for a callback that doesn't match a flow this
// browser started (expired, tampered with, or another user's).
var ErrOAuthState = errors.New("the authorisation link expired or wasn't started here; try connecting again")

type oauthState struct {
	ConnectionID int64  `json:"c"`
	UserID       int64  `json:"u"`
	Nonce        string `json:"n"`
	Expires      int64  `json:"e"`
}

func (s *Service) oauthConfig(spec OAuthSpec, settings Settings) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     settings.String("client_id"),
		ClientSecret: settings.Secrets["client_secret"],
		Endpoint:     oauth2.Endpoint{AuthURL: spec.AuthURL, TokenURL: spec.TokenURL, AuthStyle: spec.AuthStyle},
		RedirectURL:  s.OAuthRedirectURL(),
		Scopes:       spec.Scopes,
	}
}

func (s *Service) signState(st oauthState) (string, error) {
	payload, err := json.Marshal(st)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.oauthKey)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) parseState(raw string) (oauthState, error) {
	var st oauthState
	body, sig, ok := strings.Cut(raw, ".")
	if !ok {
		return st, ErrOAuthState
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return st, ErrOAuthState
	}
	mac := hmac.New(sha256.New, s.oauthKey)
	mac.Write([]byte(body))
	if !hmac.Equal(got, mac.Sum(nil)) {
		return st, ErrOAuthState
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || json.Unmarshal(payload, &st) != nil {
		return st, ErrOAuthState
	}
	if time.Now().Unix() > st.Expires {
		return st, ErrOAuthState
	}
	return st, nil
}

// StartOAuth returns the provider URL to send the browser to, and the value
// of the cookie to set alongside it.
func (s *Service) StartOAuth(ctx context.Context, p auth.Principal, id int64) (authURL, cookie string, err error) {
	c, prov, err := s.get(ctx, p, id)
	if err != nil {
		return "", "", err
	}
	op, ok := prov.(OAuthProvider)
	if !ok || prov.Manifest().Auth != AuthOAuth2 {
		return "", "", userErr("this integration doesn't use OAuth")
	}
	if s.opts.PublicURL == "" {
		return "", "", userErr("PUBLIC_URL isn't set on the server, so there's no address to return to")
	}
	v, err := s.open(c)
	if err != nil {
		return "", "", err
	}
	settings := Settings{Values: c.Config, Secrets: v.Fields}
	if missing := missingFields(prov.Manifest(), settings); len(missing) > 0 {
		return "", "", userErr("fill in " + strings.Join(missing, ", ") + " first")
	}
	spec := op.OAuth(settings)

	nonce := rand.Text()
	state, err := s.signState(oauthState{ConnectionID: c.ID, UserID: p.UserID, Nonce: nonce, Expires: time.Now().Add(oauthStateTTL).Unix()})
	if err != nil {
		return "", "", err
	}
	var opts []oauth2.AuthCodeOption
	for k, v := range spec.AuthParams {
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}
	cookie = nonce
	if spec.PKCE {
		verifier := oauth2.GenerateVerifier()
		opts = append(opts, oauth2.S256ChallengeOption(verifier))
		cookie += "." + verifier
	}
	return s.oauthConfig(spec, settings).AuthCodeURL(state, opts...), cookie, nil
}

// FinishOAuth completes the flow from the provider's callback and returns
// the provider id, for the page to return to.
func (s *Service) FinishOAuth(ctx context.Context, p auth.Principal, rawState, code, cookie string) (string, error) {
	st, err := s.parseState(rawState)
	if err != nil {
		return "", err
	}
	nonce, verifier, _ := strings.Cut(cookie, ".")
	if st.UserID != p.UserID || subtle.ConstantTimeCompare([]byte(nonce), []byte(st.Nonce)) != 1 {
		return "", ErrOAuthState
	}
	c, prov, err := s.get(ctx, p, st.ConnectionID)
	if err != nil {
		return "", err
	}
	op, ok := prov.(OAuthProvider)
	if !ok {
		return "", userErr("this integration doesn't use OAuth")
	}
	if code == "" {
		return c.Provider, userErr("the provider didn't send an authorisation code")
	}
	v, err := s.open(c)
	if err != nil {
		return c.Provider, err
	}
	settings := Settings{Values: c.Config, Secrets: v.Fields}
	var opts []oauth2.AuthCodeOption
	if verifier != "" {
		opts = append(opts, oauth2.VerifierOption(verifier))
	}
	exchangeCtx, cancel := context.WithTimeout(context.WithValue(ctx, oauth2.HTTPClient, s.http), 20*time.Second)
	defer cancel()
	tok, err := s.oauthConfig(op.OAuth(settings), settings).Exchange(exchangeCtx, code, opts...)
	if err != nil {
		var re *oauth2.RetrieveError
		msg := "the provider refused the authorisation"
		if errors.As(err, &re) && re.ErrorDescription != "" {
			msg += ": " + re.ErrorDescription
		}
		s.logActivity(ctx, c.ID, &Activity{Kind: ActivitySetup, Outcome: OutcomeFailed, Summary: describeError(userErr(msg)), userID: p.UserID})
		return c.Provider, userErr(msg)
	}
	v.Token = tok
	if c.sealed, err = s.seal(c.ID, v); err != nil {
		return c.Provider, err
	}
	if ready(prov.Manifest(), settings, v, c) {
		c.Status, c.FailureCount, c.LastError, c.LastErrorAt = StatusActive, 0, nil, nil
	}
	if err := s.store.UpdateConnection(ctx, c); err != nil {
		return c.Provider, err
	}
	s.logActivity(ctx, c.ID, &Activity{Kind: ActivitySetup, Outcome: OutcomeSuccess, Summary: "Authorised", userID: p.UserID})
	return c.Provider, nil
}
