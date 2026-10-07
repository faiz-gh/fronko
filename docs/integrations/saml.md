# SAML single sign-on

SAML SSO lets people sign in to Fronko with your company's identity provider (IdP) instead of a Fronko password. Fronko is the SAML 2.0 **service provider** (SP). This page covers how it works and how to set it up with any SAML 2.0 identity provider; there are step-by-step guides for [Okta](okta-saml.md) and [Microsoft Entra ID](entra-saml.md).

- **Who can connect it:** the owner and admins.
- **How many:** one single sign-on connection per organisation.
- **Server needs:** `FRONTEND_URL` or `PUBLIC_URL` (the sign-in URLs are built from it) and `SECRETS_KEY` (each connection's private key is stored encrypted).

## How people sign in

People can start single sign-on three ways:

- **From the sign-in page.** **Sign in with SSO** asks for their work email or the organisation's handle. An email works once its domain is [verified](#verify-your-email-domain); the handle always works.
- **From the organisation's sign-in link**, `<site>/login/sso/<handle>`, for example `https://cards.example.com/login/sso/acme`. Share it, or bookmark it.
- **From the identity provider's app launcher** (IdP-initiated), only if you switch on **Allow sign-in from the identity provider's app launcher**. It's off by default because it's less safe: there's no request from Fronko for the response to answer.

Fronko sends the person to the identity provider with a signed request. When the identity provider posts back a signed response, Fronko checks the signature, the audience, the time window and that it answers the request this browser started, then signs the person in.

## Who gets in

Fronko finds the person by **email address**. The identity provider must send it, either as the NameID or as an attribute. These attribute names are recognised (case doesn't matter):

| Value | Attribute names |
| ----- | --------------- |
| Email | `email`, `mail`, `emailaddress`, `user.email`, `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress`, `urn:oid:0.9.2342.19200300.100.1.3`; otherwise the NameID, if it's an email |
| Display name | `displayName`, `fullname`, `http://schemas.microsoft.com/identity/claims/displayname`, `urn:oid:2.16.840.1.113730.3.1.241` |
| First and last name | `firstName` / `givenName` / `given_name`, and `lastName` / `surname` / `sn` / `familyName`, plus their claim URIs and OIDs |

Then:

- **Someone with an account in your organisation** is signed in.
- **Someone with no account** gets one, as a member, if **Create accounts on first sign-in** is on (the default). Their username comes from their email, and they have no password. If it's off, they're told to ask an admin.
- **Someone whose email belongs to another Fronko organisation** is refused.
- Suspended people and suspended organisations are refused.

When the email is on a **verified domain**, Fronko trusts the identity provider about it and marks the address as verified. Otherwise the person signs in but is asked to confirm their email with a code first, as with any new account.

## Require single sign-on

With **Require single sign-on** on, nobody can sign in or reset a password with a password: the sign-in page sends them to your identity provider instead. The API answers `403` with code `sso_required`.

**The owner is the exception**, so someone can always get in to fix things if the identity provider breaks. People who never had a password (created by SSO or SCIM) always use single sign-on, whether or not it's required.

To switch single sign-on off in an emergency, the owner signs in with their password and pauses or removes the connection; everyone with a password can then use it again.

## Verify your email domain

A verified domain lets people sign in by typing just their work email, and lets SSO and SCIM mark addresses on it as verified.

1. Under the SSO connection, open **Email domains**, and add your domain (for example `acme.com`).
2. Add the TXT record Fronko shows to the domain's DNS: name `acme.com`, value `fronko-verification=…`.
3. Click **Verify**. DNS changes can take a while to appear; try again later if it isn't found yet.

An organisation can add up to 20 domains. A domain can be verified by one organisation only: the first to verify it. You can remove the TXT record afterwards, but keeping it does no harm.

## Set up any SAML 2.0 identity provider

This works with OneLogin, JumpCloud, Google Workspace, ADFS, Ping, Keycloak and others.

1. In Fronko, open **Integrations → Other SAML provider**, click **Connect** and save. Fronko makes the connection's own key pair and shows its service provider details:

   | Fronko shows | Your identity provider calls it |
   | ------------ | ------------------------------- |
   | **ACS URL** `…/auth/saml/<id>/acs` | Assertion Consumer Service URL, Reply URL, Single sign-on URL |
   | **Entity ID** `…/auth/saml/<id>/metadata` | Audience URI, SP Entity ID, Identifier |
   | **SP metadata URL** (the same address) | Some identity providers can read all of this from it |
   | **Sign-in URL** `…/login/sso/<handle>` | Sign on URL, Login URL (optional) |

   The ACS URL, Entity ID and metadata URL are on the API's address (`api.cards.example.com` when the API has its own domain); the sign-in URL is on the site.

2. In your identity provider, create a SAML 2.0 application with those values. Send the email address as the NameID (format EmailAddress) or as an `email` attribute, and optionally `firstName` and `lastName`, or `displayName`. Responses must be signed.
3. Copy the identity provider's **metadata URL** into Fronko. If it doesn't publish one, paste the metadata XML instead. The URL is better: Fronko re-reads it every hour, so certificate rollovers just work.
4. Save, then click **Test**: Fronko fetches the metadata and reports its entity ID, sign-in URL, and when its signing certificate expires (with a warning within 30 days).
5. Assign people to the application in your identity provider, then try **Sign in with SSO**.

The service provider URLs name the connection, not your organisation's handle, so changing the handle doesn't break SSO. Deleting and recreating the connection does: the new one has a new id and key pair, so update your identity provider.

## Troubleshooting

Every sign-in, successful or not, is in the connection's activity log; expand a failed one for the reason.

| Someone sees | Likely cause |
| ------------ | ------------ |
| "Start signing in from Fronko's sign-in page …" | They started from the identity provider's app launcher while IdP-initiated sign-in is off, or took more than 10 minutes to sign in |
| "Your identity provider didn't send your email address" | Add an email attribute, or use EmailAddress as the NameID format |
| "This email address belongs to a Fronko account in another organisation" | The address is already used elsewhere; an admin there has to remove or change it |
| "You don't have a Fronko account yet" | **Create accounts on first sign-in** is off; add them first (or with [SCIM](entra-scim.md)) |
| "Sign-in failed …" with "audience" or "destination" in the log | The Entity ID or ACS URL in the identity provider doesn't match Fronko's exactly |
| "Sign-in failed …" with "signature" in the log | The response or assertion isn't signed, or the metadata's certificate is out of date |
| "Your identity provider can't be reached right now" | Fronko couldn't fetch the metadata URL (it keeps using the last copy for up to a day) |
