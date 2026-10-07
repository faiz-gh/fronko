# HubSpot

The **HubSpot** integration creates or updates a HubSpot contact for every lead your cards collect, matched on email, and adds a note saying which card it came from.

- **Who can connect it:** the owner and admins, for the whole organisation.
- **How many:** one per organisation.
- **Server needs:** `PUBLIC_URL` (for the OAuth redirect) and `SECRETS_KEY` (the client secret and tokens are stored encrypted).

Fronko doesn't ship a shared HubSpot app. Your organisation registers its own, so its credentials, rate limits and permissions are yours alone ([why](../adr/0004-per-organisation-oauth-apps.md)).

## 1. Create a HubSpot app

1. Sign in to a [HubSpot developer account](https://developers.hubspot.com/) (free), and create an app (**Apps → Create app**). Name it after your organisation, for example "Acme Fronko".
2. Open the app's **Auth** settings:
   - Under **Redirect URLs**, add the redirect URL shown on Fronko's HubSpot page. It's `<PUBLIC_URL>/api/integrations/oauth/callback`, for example `https://cards.example.com/api/integrations/oauth/callback`.
   - Under **Scopes**, add `oauth`, `crm.objects.contacts.read` and `crm.objects.contacts.write`.
3. Save, and keep the page open: you need its **Client ID** and **Client secret**.

## 2. Connect it in Fronko

1. Open **Integrations → HubSpot** and click **Connect**.
2. Paste the **Client ID** and **Client secret**, and save. The connection shows **Setup incomplete** until it's authorised.
3. Click **Authorise with HubSpot**. HubSpot asks which account (portal) should receive leads; choose it and approve.
4. Back in Fronko, the connection is **Active**. Click **Send test lead**: a contact called Test Lead (`test.lead@example.com`) appears in HubSpot. You can delete it.

If you change the client ID or secret later, the old authorisation stops working; click **Authorise with HubSpot** again.

## How leads map to contacts

Each lead upserts one contact through HubSpot's batch upsert API, matched on email, so a visitor who fills in two cards updates the same contact rather than creating a duplicate.

| Lead | HubSpot contact property |
| ---- | ------------------------ |
| Email | `email` (the match key) |
| Name, first word | `firstname` |
| Name, the rest | `lastname` |
| Phone (E.164) | `phone`, when given |

With **Add a note to the contact** on (the default), Fronko also creates a note on the contact's timeline with the card's name and link, how the visitor found it (tapped the NFC card, scanned the QR code, or opened the link), the card holder, and the visitor's message. If the note can't be added, the contact is still saved and the activity log says so.

The mapping is fixed. To map to other properties, or to create deals and tasks, use the [Webhook or Zapier](webhook-zapier.md) integration instead.

## Tokens

HubSpot access tokens last about 30 minutes. Fronko refreshes them when they expire and stores the new token encrypted. If the refresh token is revoked (someone uninstalled the app in HubSpot, or the app was deleted), deliveries stop with "the authorisation was revoked or expired"; authorise again to fix it.

## Troubleshooting

| Activity log says | What to do |
| ----------------- | ---------- |
| "HubSpot refused the authorisation; authorise the connection again" | The token was revoked. Click **Authorise with HubSpot** |
| "the HubSpot app is missing a scope …" | Add `crm.objects.contacts.write` (and `crm.objects.contacts.read`) under the app's scopes, then authorise again |
| "HubSpot rejected the request: …" | HubSpot refused the data, for example an invalid email. Expand the entry for HubSpot's message |
| "HubSpot answered 429" or "5xx" | Temporary; Fronko retries on its own |
| The OAuth page says the redirect URL doesn't match | The redirect URL in the HubSpot app must exactly match the one Fronko shows, including `https` and the path |
