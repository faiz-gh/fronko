# Okta single sign-on (SAML)

Sign in to Fronko with Okta, and create Fronko accounts for new people the first time they sign in. Read [SAML single sign-on](saml.md) for how sign-in, account creation, required SSO and email domains work; this page has the Okta steps.

- **Who can connect it:** the owner and admins. Okta admin rights are needed for steps 2–5.
- **Server needs:** `FRONTEND_URL` (or `PUBLIC_URL`) and `SECRETS_KEY`.

## 1. Start in Fronko

Open **Integrations → Okta** (under SAML SSO), click **Connect** and **Save** without filling anything in yet. The connection shows Fronko's sign-in URLs: the **ACS URL**, the **Entity ID** and the **Sign-in URL**. Keep the page open. Copy them from there: the examples below assume the API is on the site's domain, and when it has its own (`api.cards.example.com`), the ACS URL and Entity ID are on that.

## 2. Create the app in Okta

1. In the Okta Admin Console, go to **Applications → Applications → Create App Integration**.
2. Choose **SAML 2.0** and click **Next**.
3. Name it **Fronko** (optionally upload a logo), and click **Next**.

## 3. Configure SAML

Under **SAML Settings**:

| Okta setting | Value |
| ------------ | ----- |
| Single sign-on URL | Fronko's **ACS URL** (`https://cards.example.com/auth/saml/<id>/acs`). Leave "Use this for Recipient URL and Destination URL" ticked |
| Audience URI (SP Entity ID) | Fronko's **Entity ID** (`https://cards.example.com/auth/saml/<id>/metadata`) |
| Name ID format | `EmailAddress` |
| Application username | `Email` |

Under **Attribute Statements**, add:

| Name | Value |
| ---- | ----- |
| `email` | `user.email` |
| `firstName` | `user.firstName` |
| `lastName` | `user.lastName` |

Click **Next**, answer the feedback question, and **Finish**.

## 4. Give Fronko the metadata URL

On the app's **Sign On** tab, under **SAML 2.0**, copy the **Metadata URL**. Paste it into Fronko's **Okta metadata URL** field, choose the options you want, and save:

- **Create accounts on first sign-in** (on by default)
- **Require single sign-on** (off by default; turn it on once you've tested sign-in)
- **Allow sign-in from the identity provider's app launcher** (off by default; see below)

Click **Test**. Fronko should report the metadata is valid and when Okta's signing certificate expires.

## 5. Assign people and try it

1. On the app's **Assignments** tab, assign the people or groups who should use Fronko.
2. In a private window, open Fronko's sign-in page, click **Sign in with SSO**, and enter your organisation's handle. You should land in the dashboard.
3. [Verify your email domain](saml.md#verify-your-email-domain) so people can sign in with just their work email.

## Okta dashboard tile

Okta shows assigned apps on people's dashboards. Clicking the tile is an IdP-initiated sign-in, which Fronko refuses unless **Allow sign-in from the identity provider's app launcher** is on. If you'd rather keep it off (recommended), make the tile start at Fronko instead: on the app's **General** tab, set **Application visibility** to hide the generated tile, then add a **Bookmark App** pointing at Fronko's **Sign-in URL** (`https://cards.example.com/login/sso/<handle>`).

## Provisioning

This integration signs people in. To also create, update and deactivate people from Okta before they first sign in, you'll want SCIM; Okta SCIM is coming soon. Until then, people are created on first sign-in, and admins suspend or delete them in Fronko when they leave.
