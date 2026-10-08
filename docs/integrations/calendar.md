# Calendar booking

Calendar Booking puts a **Book a meeting** button on cards, linking to a booking page. Booking pages are connected in Integrations, then each card picks the one it shows.

- **Who can connect it:** anyone, for their own cards; admins, for the organisation.
- **How many:** as many as you like, across any providers (for example a Calendly "30-min intro" and a Cal.com "Product demo"). Give each a name; the name is what you pick from on a card.
- **Server needs:** nothing.

## Which page a card shows

Each card chooses one page, or none, in the card editor under **Booking**. It can choose from:

1. The card holder's own booking pages.
2. The organisation's booking pages.

A card nobody is assigned to can only choose the organisation's pages. A card that hasn't chosen a page shows no button.

If the chosen page stops being available, the card shows no button until someone picks another. That happens when the page is deleted or paused, or when the card is assigned to someone else and the page belonged to the previous holder. The editor flags this. Deleting a booking page in Integrations says how many cards use it first.

Choosing a page also brings back the card's **Book a meeting** block if it was hidden or missing. Email signatures link to the same page.

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
2. In Fronko, open **Integrations**, choose the provider under **Calendar Booking** and click **Connect**. Admins choose **Personal** or **Organisation** (a page any card can show).
3. Name it (for example "30-min intro"), paste the link and save. **Test** checks the page opens. Use **Add another booking page** for more.
4. Open each card, go to **Booking**, and choose the page it should show.

## Moving from the old booking field

Before Integrations, each card had its own booking link (`calendar_url` in the card data). That field has been removed: cards now choose a booking page connected in Integrations, as described above.

Until October 2026 a card showed its holder's booking page, or else the organisation's default, without choosing. Cards now show a page only once one is chosen under **Booking**, so cards from before then show no button until someone picks one.
