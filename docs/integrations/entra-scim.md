# Microsoft Entra ID provisioning (SCIM)

Team Member Import keeps Fronko's people in step with Microsoft Entra ID (formerly Azure AD). Fronko runs a SCIM 2.0 server; Entra's provisioning service calls it to add people when they're assigned to the Fronko app, update them when they change, and suspend or remove them when they leave. Entra groups become Fronko teams.

- **Who can connect it:** the owner and admins. In Entra you need the Cloud Application Administrator or Application Administrator role.
- **How many:** one directory connection per organisation.
- **Server needs:** `PUBLIC_URL` (the SCIM tenant URL is built from it). The site must be reachable from the internet over HTTPS, since Microsoft's servers call it.

Pair it with [Entra ID single sign-on](entra-saml.md) so people sign in with their Microsoft account and never need a Fronko password.

## 1. Create the connection and a token

1. In Fronko, open **Integrations → Microsoft Entra ID** (under Team Member Import), click **Connect** and save.
2. Click **Generate token**. Copy the token (it starts with `fronko_scim_`) right away: it's shown once. Fronko stores only a hash of it.
3. Note the **Tenant URL**, `<PUBLIC_URL>/scim/v2`, for example `https://cards.example.com/scim/v2`.

The connection becomes **Active** once it has a token.

## 2. Configure provisioning in Entra

1. In the [Microsoft Entra admin center](https://entra.microsoft.com), go to **Identity → Applications → Enterprise applications → New application → Create your own application**. Name it **Fronko** and choose **Non-gallery**. (Use the same app as single sign-on if you've set that up.)
2. Open **Provisioning** and click **Get started** (or **New configuration**). Set **Provisioning Mode** to **Automatic**.
3. Under **Admin Credentials**, paste the **Tenant URL** and the token as the **Secret Token**. Click **Test Connection**; it should succeed.
4. Save.

## 3. Check the mappings

Under **Mappings**, keep **Provision Microsoft Entra ID Users** and, if you want teams, **Provision Microsoft Entra ID Groups** switched on. The default user mappings work; the ones Fronko uses are:

| Entra attribute | SCIM attribute | Fronko |
| --------------- | -------------- | ------ |
| `userPrincipalName` | `userName` | Matched to find the person again; kept as their directory user name |
| `mail` | `emails[type eq "work"].value` | Their email (required; used for sign-in and invites). If someone has no `mail`, a `userName` that is an email address is used |
| `Switch([IsSoftDeleted], …)` | `active` | `false` suspends them, `true` restores them |
| `displayName` (or `givenName` + `surname`) | `displayName`, `name.givenName`, `name.familyName` | Their full name |
| `objectId` | `externalId` | Kept to match them later |

Other attributes Entra sends (job title, phone numbers, the enterprise extension) are accepted and ignored, so you don't need to remove them.

## 4. Assign people and start

1. Open **Users and groups** and assign the people and groups who should be in Fronko. Assigning a group provisions its members; with group provisioning on, it also becomes a team.
2. Back in **Provisioning**, set **Provisioning Status** to **On** and save. Entra's first cycle starts within a few minutes; later cycles run about every 40 minutes. **Provision on demand** tries a single person right away, which is handy for testing.

Every change Entra makes appears in the Fronko connection's activity log ("Added …", "Deactivated …", "Updated team …"), and so do requests Fronko refused, with the reason.

## What happens to people

| In Entra | In Fronko |
| -------- | --------- |
| Someone is assigned to the app | An account is created as a **member**, with a username made from their email. If the organisation has single sign-on, they get no password and an email telling them to sign in with SSO. Otherwise they're emailed a temporary password, which they must change at first sign-in |
| Their details change | Their email, name and directory ids are updated |
| They're unassigned, disabled or soft-deleted | Their account is **suspended**: they're signed out and can't sign in. Their cards, leads and files stay |
| They're re-enabled | Their account is restored |
| They're deleted for good | Their account is **deleted**. As when an admin deletes someone, their cards, files and leads are kept and pass to the organisation |

- An email on a [verified domain](saml.md#verify-your-email-domain) is marked as verified; others must confirm it with a code at first sign-in.
- People who already had a Fronko account (made by an admin, before provisioning) are matched when their `userPrincipalName` is their Fronko email, so Entra takes them over instead of creating a duplicate. If it isn't, Entra's create is refused with a `409` (the email already exists); remove the old account or change its email first.
- An email that belongs to another Fronko organisation is refused.
- The organisation's **owner** can't be deactivated or deleted from the directory, so a mistake in Entra can't lock everyone out.
- Roles aren't provisioned: everyone Entra creates is a member, and admins promote people in Fronko.

## What happens to groups

| In Entra | In Fronko |
| -------- | --------- |
| A group is assigned (with group provisioning on) | A **team** with the group's name, and its members |
| Members are added or removed | Team members are added or removed. People keep their team role (member or lead); new members join as members |
| The group is renamed | The team is renamed |
| The group is removed | The team is deleted; its files move to the organisation's files |

Team leads are set in Fronko, not in Entra.

## The SCIM API

For other SCIM clients, or for troubleshooting, Fronko's SCIM server supports:

- `GET /scim/v2/ServiceProviderConfig`, `/ResourceTypes`, `/Schemas`
- `GET|POST /scim/v2/Users`, `GET|PUT|PATCH|DELETE /scim/v2/Users/{id}`
- `GET|POST /scim/v2/Groups`, `GET|PUT|PATCH|DELETE /scim/v2/Groups/{id}`
- `eq` filters on `userName`, `externalId`, `emails` / `emails.value` and `id` for users, and `displayName`, `externalId` and `id` for groups
- Pagination with `startIndex` and `count` (up to 200 per page); `excludedAttributes=members` on groups

Bulk operations, sorting, ETags and password changes aren't supported. Every request needs `Authorization: Bearer <token>`. See [API reference: SCIM](../../backend/API.md#scim-20).

## Rotating the token

Click **Generate token** again to replace it; the old one stops working at once. Paste the new one into Entra's **Secret Token** and save. Pausing the connection in Fronko also stops the token working, which pauses provisioning.

## Troubleshooting

| Entra's provisioning log says | What to do |
| ----------------------------- | ---------- |
| `401` "the token is invalid, revoked, or its connection is paused" | Generate a new token and paste it into Entra, or switch the connection back on |
| `400` "a valid work email is required" | Give the person a `mail` address in Entra, or make their `userPrincipalName` their email |
| `409` "… already has a Fronko account in another organisation" | The email is used in another Fronko organisation |
| `409` "a user with this userName already exists" | Two Entra users map to the same Fronko person; check for duplicate addresses |
| Test Connection fails to reach the server | `PUBLIC_URL` must be the public HTTPS address, and `/scim/` must reach the backend through your proxy |
