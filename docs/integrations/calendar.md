# Calendar booking

Calendar Booking puts a **Book a meeting** button on cards, linking to a booking page. Booking pages are connected in Integrations rather than typed into each card, so a person sets theirs once and every card they hold uses it, and an organisation can give every card a default.

- **Who can connect it:** anyone, for their own cards; admins, for the organisation's default.
- **How many:** one booking page per person, plus one organisation default.
- **Server needs:** nothing.

## Which page a card shows

1. The card holder's own booking page, if they've connected one.
2. Otherwise the organisation's default booking page.
3. Otherwise no button.

Cards that aren't assigned to anyone always use the organisation default. The card editor's **Contact** section shows which page the card uses and links to Integrations to change it. The same link is offered in email signatures.

## Supported booking pages

| Provider | Link looks like | Visitor details filled in |
| -------- | --------------- | ------------------------- |
| Calendly | `https://calendly.com/your-name/30min` | Name and email |
| Chili Piper | `https://your-company.chilipiper.com/me/your-name` | |
| Microsoft Bookings | `https://outlook.office.com/book/YourPage@your-company.com/` | |
| HubSpot Meetings | `https://meetings.hubspot.com/your-name` | First name, last name and email |
| Google Calendar | `https://calendar.app.google/…` (appointment schedule) | |
| Other booking link | Anything else: Cal.com, Zoho Bookings, SavvyCal, TidyCal … | |

Fronko checks the link is on the provider's own domain, so a Calendly connection can't hold some other address. **Other booking link** accepts any `https` link.

**Filled-in details.** When a visitor has already sent the card's contact form during their visit, the button opens Calendly or HubSpot Meetings with their name and email filled in, so they don't type them twice.

## Set it up

1. Copy your booking page's link from your scheduling tool:
   - **Calendly:** open the event type visitors should book and click **Copy link**.
   - **Chili Piper:** open your personal or team meeting link and copy it.
   - **Microsoft Bookings:** open your booking page (or personal booking page) and copy its link.
   - **HubSpot Meetings:** in HubSpot, go to **Sales → Meetings** and copy your scheduling page's link.
   - **Google Calendar:** open your appointment schedule, click **Share**, and copy the booking page link.
2. In Fronko, open **Integrations**, choose the provider under **Calendar Booking** and click **Connect**. Admins choose **Personal** or **Organisation** (the default for everyone).
3. Paste the link and save. **Test** checks the page opens.

To switch providers, remove the current booking page and connect the new one.

## Moving from the old booking field

Before Integrations, each card had its own booking link (`calendar_url` in the card data). That field has been removed: cards now get their booking page from Integrations, as described above. Connect a personal booking page (or an organisation default) to bring the button back.
