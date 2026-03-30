# Partner Demo — ChristJesus Platform, March 2026

A walkthrough of what's been built this past week. Each section maps to something you can click through live.

---

## 1. Finding Needs Near You (Geospatial Search)

**What to show:** Browse page → enter a ZIP code + pick a radius (5/15/25/50 mi) → results filter to nearby needs with distance shown on each card ("~3 mi away").

**Why it matters:** Donors are much more likely to give when a need feels local and real. Before this week, there was no way to search by location at all. Now every need has a validated address and donors can find what's near them in seconds.

**Under the hood (if asked):** We loaded 33,000 US ZIP code centroids from Census data and use PostGIS spatial queries to do the radius filtering. Addresses are validated and standardized against the USPS API when a recipient submits a need.

---

## 2. The Donor Experience — From Sign-Up to Donation

Walk through the full donor journey end-to-end:

**a. Onboarding with a progress bar**
- Show the 3-step donor onboarding (Welcome → Preferences → Confirmation) with the step indicator ("Step 1 of 3").
- At the preferences step, point out the **Skip for now** link — donors who arrived to give to a specific need aren't forced to set preferences first.

**b. Personalized home page**
- After setting preferences (categories + location), the home page shows **"Recommended for you"** instead of generic featured needs. The platform is now surfacing relevant content based on what the donor told us they care about.

**c. Browse with preferences pre-applied**
- Visit the browse page. The donor's saved ZIP, radius, and categories are automatically applied as filters.
- Show the **"Using your preferences — Disable"** toggle so they can easily switch to unfiltered browsing.

**d. Smart donation amounts**
- Open a need that's close to fully funded (e.g. $73 remaining). The preset buttons ($100, $250) that would overshoot are removed. A prominent gold **"Fund the remaining $73"** button appears instead.
- This nudges donors to complete a need rather than accidentally over- or under-donating.

**e. Saving needs for later**
- On a need detail page, show the **"Save for Later"** bookmark button in the sidebar.
- Visit the donor profile — the saved need appears in a **"Saved Needs"** section with View and Remove actions.

**f. After donating — "Needs like this"**
- Complete a donation. The confirmation page now shows up to 3 similar needs from the same category — keeping the donor engaged and making it easy to give again.

---

## 3. Donation Receipts via Email

**What to show:** Complete a test donation → check email for a receipt.

**Why it matters:** This is table stakes for any donation platform and a legal/trust requirement. Donors now automatically receive a receipt as soon as their payment clears.

**Extra context:** We built the full email infrastructure from scratch this week — a provider-agnostic layer backed by Resend, with a database audit trail for every email sent, delivery/bounce/complaint tracking via webhooks, and an automatic suppression list so we never email someone who asked not to be contacted.

---

## 4. Profile Management

**What to show:** Log in → navigate to profile.

- **Edit Profile**: display name, email address, and password reset are now all editable inline (behind an Edit button — not always-on forms).
- **My Preferences**: donors have a dedicated preferences page where they can update their categories, ZIP/radius, and donation range without going through onboarding again.
- **Submit a Need CTA**: recipients with no needs see a clear empty-state prompt to get started.

---

## 5. Admin — Urgency & User Management

**What to show:** Log in as admin.

**Urgency control:**
- When approving a need, the admin now sets its urgency level (Low / Medium / High / Urgent). This drives the sort order on the browse page.
- On any active need's review page, urgency can be updated at any time inline.

**User management:**
- `/admin/users` — searchable, filterable list of all users.
- Click a recipient: see their submitted needs with status and funding progress.
- Click a donor: see their full donation history.

---

## One-Liner Summary

> In one week: donors can find needs near them, get personalized recommendations, complete a need with smart presets, save ones they're not ready to fund yet, receive email receipts automatically, and manage their preferences — while admins have full user visibility and control over how needs are prioritized.
