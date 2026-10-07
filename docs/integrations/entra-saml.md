# Microsoft Entra ID single sign-on (SAML)

Sign in to Fronko with Microsoft Entra ID (formerly Azure AD), and create Fronko accounts for new people the first time they sign in. Read [SAML single sign-on](saml.md) for how sign-in, account creation, required SSO and email domains work; this page has the Entra steps.

To also add, update and remove people automatically, set up [Entra ID SCIM provisioning](entra-scim.md) as well. The two work together: SCIM creates the accounts (with no password while SSO is on), SAML signs people in.

- **Who can connect it:** the owner and admins. In Entra you need the Cloud Application Administrator or Application Administrator role.
- **Server needs:** `PUBLIC_URL` and `SECRETS_KEY`.

## 1. Start in Fronko

Open **Integrations → Microsoft Entra ID** (under SAML SSO), click **Connect** and **Save** without filling anything in yet. The connection shows Fronko's **ACS URL**, **Entity ID** and **Sign-in URL**. Keep the page open.

## 2. Create the enterprise application

1. In the [Microsoft Entra admin center](https://entra.microsoft.com), go to **Identity → Applications → Enterprise applications → New application**.
2. Click **Create your own application**, name it **Fronko**, choose **Integrate any other application you don't find in the gallery (Non-gallery)**, and click **Create**.

If you've already created a Fronko app for SCIM, use the same one.

## 3. Configure SAML

1. In the app, open **Single sign-on** and choose **SAML**.
2. Under **Basic SAML Configuration**, click **Edit**:

   | Entra setting | Value |
   | ------------- | ----- |
   | Identifier (Entity ID) | Fronko's **Entity ID** (`https://cards.example.com/auth/saml/<id>/metadata`). Tick **Default** |
   | Reply URL (Assertion Consumer Service URL) | Fronko's **ACS URL** (`https://cards.example.com/auth/saml/<id>/acs`) |
   | Sign on URL | Fronko's **Sign-in URL** (`https://cards.example.com/auth/sso/<handle>`) |

   Save.
3. Leave **Attributes & Claims** at the defaults: `emailaddress` (`user.mail`), `givenname`, `surname` and `name`. Fronko reads the email claim, so make sure everyone who'll sign in has a **mail** address in Entra. If yours are only in `userPrincipalName`, change the `emailaddress` claim's source to `user.userprincipalname`.

## 4. Give Fronko the metadata URL

Under **SAML Certificates**, copy the **App Federation Metadata Url**. Paste it into Fronko's **App Federation Metadata URL** field, choose the options you want, and save:

- **Create accounts on first sign-in** (on by default)
- **Require single sign-on** (off by default; turn it on once you've tested sign-in)
- **Allow sign-in from the identity provider's app launcher** (off by default)

Click **Test**. Fronko should report the metadata is valid and when Entra's signing certificate expires. Because Fronko reads the metadata URL every hour, a certificate Entra rolls over is picked up on its own.

## 5. Assign people and try it

1. In the app, open **Users and groups → Add user/group**, and assign the people or groups who should use Fronko.
2. In a private window, open Fronko's sign-in page, click **Sign in with SSO**, and enter your organisation's handle. You should land in the dashboard.
3. [Verify your email domain](saml.md#verify-your-email-domain) so people can sign in with just their work email.

## My Apps

The Fronko tile in My Apps (myapps.microsoft.com) opens the **Sign on URL** you set in step 3, so it starts at Fronko and works with IdP-initiated sign-in left off.
