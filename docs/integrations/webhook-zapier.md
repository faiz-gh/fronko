# Webhook, Zapier, Make and n8n

The **Webhook** integration sends each new lead as JSON to a URL you choose: your own code, or an automation tool that passes it on to thousands of other apps. **Zapier**, **Make** and **n8n** are the same integration with setup steps written for each tool.

- **Who can connect it:** admins (for every lead in the organisation) or anyone (for the leads on the cards they hold).
- **How many:** as many as you like.
- **Server needs:** `SECRETS_KEY`, when you use a signing secret.

## Set up a webhook

1. Create an endpoint that accepts `POST` requests with a JSON body. It must be reachable over `https` from your Fronko server.
2. In Fronko, open **Integrations → Webhook**, click **Connect**, and choose whether it's for the organisation or for you.
3. Paste the URL into **Payload URL**.
4. Optionally click **Generate** next to **Signing secret**, and copy the secret into your endpoint's configuration. It's shown only until you save.
5. Save, then click **Send test lead**. Your endpoint should answer with any `2xx` status within 20 seconds.

The activity log shows each delivery with the status your endpoint returned and the start of its response.

## The request

```http
POST /hooks/fronko HTTP/1.1
Content-Type: application/json
User-Agent: Fronko-Webhooks/1
X-Fronko-Event: lead.created
X-Fronko-Delivery: lead-1042-7
X-Fronko-Signature: t=1791561600,v1=5f2b…c9
```

```json
{
  "event": "lead.created",
  "version": 1,
  "test": false,
  "data": {
    "id": 1042,
    "created_at": "2026-10-08T09:20:00Z",
    "name": "Ada Lovelace",
    "email": "ada@example.com",
    "phone": "+447700900123",
    "notes": "Loved the talk, let's talk pricing.",
    "source": "nfc",
    "card": {
      "id": 3,
      "slug": "maya-chen",
      "name": "Maya Chen",
      "url": "https://cards.example.com/p/lumen/maya-chen"
    },
    "owner": { "id": 12, "username": "maya", "email": "maya@lumen.example" },
    "organisation": { "id": 1, "name": "Lumen Labs", "handle": "lumen" }
  }
}
```

| Field | Notes |
| ----- | ----- |
| `event` | Always `lead.created` for now |
| `version` | The payload version. Fields may be added within a version; a change that would break receivers gets a new version |
| `test` | `true` for **Send test lead**. Test leads have `id: 0` and made-up details |
| `data.phone` | E.164 (`+447700900123`), or `""` when the visitor gave none |
| `data.notes` | The visitor's message, or `""` |
| `data.source` | How the visitor reached the card: `nfc`, `qr`, `link`, or `""` |
| `data.card.url` | The card's public link; `""` when the server has no `PUBLIC_URL` |
| `data.owner` | Who held the card when the lead arrived, or `null` when the organisation held it |

**Headers.**

| Header | Value |
| ------ | ----- |
| `X-Fronko-Event` | The event, `lead.created` |
| `X-Fronko-Delivery` | `lead-<lead id>-<connection id>`: the same on every retry of one delivery, so you can ignore repeats. Test deliveries get `test-<random>` |
| `X-Fronko-Signature` | Only when a signing secret is set; see below |

## Responses and retries

| Your endpoint answers | Fronko |
| --------------------- | ------ |
| `2xx` | Done |
| `408`, `429`, `5xx`, a timeout or a connection error | Retries with growing waits (about 30 s, 1 m, 2 m, 4 m …), up to 10 attempts over about four hours |
| `3xx` | Stops: redirects aren't followed. Use the final URL |
| Any other `4xx` | Stops: the lead was rejected and sending it again won't help |

After three leads in a row fail for good, the connection is marked **Needs attention** and stops getting new leads until you fix it (save its settings, or switch it off and on). Deliveries are at least once, so use `X-Fronko-Delivery` to drop the occasional repeat.

## Checking the signature

With a signing secret set, every request carries:

```
X-Fronko-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256>
```

`v1` is the HMAC-SHA256, keyed with the secret, of the timestamp, a dot, and the raw request body: `"<t>.<body>"`. To check it:

1. Read the raw body **before** parsing it as JSON (re-serialised JSON won't match).
2. Compute the HMAC of `t + "." + body` with your secret, as lowercase hex.
3. Compare it with `v1` in constant time.
4. Reject requests whose `t` is more than 5 minutes from your clock, so old requests can't be replayed.

**Node.js (Express)**

```js
import crypto from 'node:crypto';
import express from 'express';

const secret = process.env.FRONKO_SIGNING_SECRET;
const app = express();

app.post('/hooks/fronko', express.raw({ type: 'application/json' }), (req, res) => {
  const header = req.get('X-Fronko-Signature') ?? '';
  const parts = Object.fromEntries(header.split(',').map((p) => p.trim().split('=')));
  const expected = crypto.createHmac('sha256', secret).update(`${parts.t}.`).update(req.body).digest('hex');
  const fresh = Math.abs(Date.now() / 1000 - Number(parts.t)) < 300;
  const valid =
    parts.v1?.length === expected.length &&
    crypto.timingSafeEqual(Buffer.from(parts.v1), Buffer.from(expected));
  if (!fresh || !valid) return res.sendStatus(401);

  const { event, data } = JSON.parse(req.body);
  // … store data, keyed by req.get('X-Fronko-Delivery') to ignore repeats
  res.sendStatus(204);
});
```

**Python (Flask)**

```python
import hashlib, hmac, os, time
from flask import Flask, abort, request

SECRET = os.environ["FRONKO_SIGNING_SECRET"].encode()
app = Flask(__name__)

@app.post("/hooks/fronko")
def fronko():
    parts = dict(p.strip().split("=", 1) for p in request.headers.get("X-Fronko-Signature", "").split(",") if "=" in p)
    body = request.get_data()
    expected = hmac.new(SECRET, parts.get("t", "").encode() + b"." + body, hashlib.sha256).hexdigest()
    if abs(time.time() - int(parts.get("t", 0))) > 300 or not hmac.compare_digest(expected, parts.get("v1", "")):
        abort(401)
    lead = request.get_json()["data"]
    # … store lead
    return "", 204
```

**Go.** The backend's own `webhook.Verify(secret, header, body, 5*time.Minute, time.Now())` does exactly this; copy it from [`webhook.go`](../../backend/internal/integrations/providers/webhook/webhook.go).

## Zapier

1. In Zapier, create a Zap. For the trigger choose **Webhooks by Zapier → Catch Hook**, and copy the webhook URL it shows (`https://hooks.zapier.com/hooks/catch/…`).
2. In Fronko, open **Integrations → Zapier**, paste the URL and save. Fronko checks it's a `hooks.zapier.com` address.
3. Click **Send test lead**, then in Zapier click **Test trigger**. The lead's fields are under `data` (`data__name`, `data__email`, `data__card__name` and so on).
4. Add your actions, for example **Create contact** in your CRM, mapping the fields you need, and publish the Zap.

Zapier's Catch Hook doesn't check signatures, so the Zapier integration has no signing secret. Keep the hook URL private: anyone with it can send data to your Zap.

## Make

1. In Make, create a scenario starting with **Webhooks → Custom webhook**, add a webhook and copy its address (`https://hook.<region>.make.com/…`).
2. In Fronko, open **Integrations → Make**, paste the address and save. Fronko checks it's a `make.com` address.
3. In Make, click **Run once**, then in Fronko click **Send test lead**, so Make learns the lead's structure.
4. Add the modules you want and switch the scenario on.

## n8n

1. In n8n, start a workflow with a **Webhook** node. Set **HTTP Method** to `POST` and copy the **Production URL** (the test URL only works while you're watching the editor).
2. In Fronko, open **Integrations → n8n**, paste the URL, optionally generate a signing secret, and save. Any URL is accepted, since n8n can be self-hosted.
3. Activate the workflow, then click **Send test lead**. The lead is in the body under `data`.
4. To check the signature, set the Webhook node's **Raw Body** option, then use a **Code** node with the steps from [Checking the signature](#checking-the-signature) (`crypto` is available in n8n's Code node).

## Troubleshooting

| Activity log says | What to do |
| ----------------- | ---------- |
| "the payload URL points at a private or local address" | The URL resolves to an internal address, which Fronko never calls in production. Use a public URL |
| "the receiver … answered with a redirect" | Use the final URL (often the `https://` or trailing-slash version) |
| "the receiver … rejected the lead (4xx)" | Your endpoint refused the request. Expand the entry to see its response |
| "couldn't reach …" | DNS, TLS or network problem. Check the address works from outside your network |
