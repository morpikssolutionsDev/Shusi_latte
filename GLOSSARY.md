# Sushi Latte

A single-business, single-outlet HORECA (hotel, restaurant, café) web platform made of several modules that cover front-of-house, kitchen, management and staff administration.

## Platform

**Business**:
The one hospitality company that owns and operates the platform, running exactly one outlet.
_Avoid_: Tenant, client, merchant, branch

**Module**:
A functional area of the platform used by a specific audience, e.g. Admin Panel, Client Menu, Waiter, Kitchen, Cashier, HRIS, Product Display.
_Avoid_: Point, multi point, app

## People

**Management**:
The owner-level role that sets the rules: Products, prices, Prep Time, Dietary Tags, discounts, tax, staff records, and sees all reports.
_Avoid_: Owner, superadmin

**Outlet Admin**:
The role that runs the outlet day to day: Sold Out, tables, shifts, leave approval, and voids or refunds within limits.
_Avoid_: Manager

**Crew**:
An employee who works one or more Stations.
_Avoid_: Staff, worker

**Crew ID**:
The unique identifier every employee signs in with, paired with a personal PIN.
_Avoid_: Username, employee number

**Station**:
A front-of-house or back-of-house post a Crew member is permitted to work: Waiter, Cashier or Kitchen.
_Avoid_: Position, job

**Customer**:
A person ordering at a Table, identified only by the name they enter when scanning and bound to the Bill rather than to a device or account.
_Avoid_: Guest, client, user

**Rejoin**:
A Customer continuing on a Bill from a new scan by choosing their own name from the Table's occupant list; the latest device wins and the previous one is signed out.
_Avoid_: Re-login, recover session

## Product catalog

**Product**:
Anything the Business sells, defined once and shared by every Module; it carries name, description, photo, Category, Prep Time, Dietary Tags and Product Status, and has one or more Variants.
_Avoid_: Menu item, dish, SKU, item (on its own)

**Product Status**:
A Product's lifecycle state: Active (on the menu), Hidden (temporarily off the menu, e.g. seasonal) or Archived (retired but kept for history).
_Avoid_: Enabled/disabled, deleted

**Variant**:
A mutually exclusive version of a Product that carries its price and Sold Out flag, such as Hot/Iced or Regular/Large; a Product with no options has one default Variant.
_Avoid_: Option, size (as a generic term)

**Modifier**:
An optional, free or priced adjustment added to an Order Item, such as extra shot or no wasabi; it may add Dietary Tags.
_Avoid_: Add-on, topping, extra

**Category**:
A grouping of Products on the menu, such as Sushi, Coffee or Dessert, assigned to one Prep Area.

**Prep Area**:
A kitchen section with its own display that receives Order Items of the Categories assigned to it, such as Kitchen or Bar.
_Avoid_: Station (reserved for Crew posts), counter

**Prep Time**:
The target preparation duration Management sets for a Product, against which kitchen lateness is measured.
_Avoid_: SLA (when meaning the per-item target)

**Sold Out**:
A flag marking a Variant temporarily unorderable on every Module, shown greyed out on the Client Menu; set manually or automatically by a Cancel with reason Out of Stock, and cleared only by Outlet Admin or Management. A Product is greyed out when all its Variants are Sold Out. It does not track stock quantities.
_Avoid_: Unavailable, disabled

## Dietary

**Dietary Tag**:
An entry in the single Management-maintained list of ingredients or properties, such as Dairy, Gluten, Raw Fish, Pork or Alcohol, used both for what a Product contains and what a Customer avoids.
_Avoid_: Allergen (too narrow), label

**Dietary Restriction**:
The Dietary Tags a Customer chooses to avoid, recorded per Customer.
_Avoid_: Medical fill, allergy profile, preference

**Dietary Note**:
Optional free-text health or dietary information a Customer adds for anything not covered by Dietary Tags.
_Avoid_: Medical fill, allergy field

**Dietary Warning**:
The notice raised when a Product, with its chosen Modifiers, contains a Dietary Tag in the ordering Customer's Dietary Restriction; it never blocks ordering and is carried onto the Order Item for staff.
_Avoid_: Alert, block

## Ordering

**Table**:
A physical seating spot with its own QR entry point to the Client Menu.

**Bill**:
The record of one Table visit, opened when the Table is first used and closed when paid.
_Avoid_: Check, tab, session, order (for the whole visit)

**Settling**:
The state of a Bill once payment has started, during which Customers cannot add Orders until a Waiter unlocks it.
_Avoid_: Locked, checkout

**Unpaid Bill**:
A Bill closed without full Payment because the Customer left, recorded with a reason under an Outlet Admin PIN Override.
_Avoid_: Walkout, write-off, bad debt

**Post-outage Entry**:
A Bill re-entered after an internet outage from its paper record, with original times, items, Payments and server, bypassing the Kitchen.
_Avoid_: Backfill, manual bill

**Order**:
One round of Order Items placed on a Bill and sent to the kitchen as a unit; a Bill holds many Orders.
_Avoid_: Order note, ticket

**Order Item**:
One Variant of a Product within an Order, with its own quantity, Modifiers, notes, Dietary Warnings and kitchen progress.
_Avoid_: Line, dish

**Takeaway Bill**:
A Bill opened at the Cashier that is not tied to a Table; carries a Customer name for pickup call-out.

**Sub-bill**:
A part of a split Bill, divided by item or equally, carrying its own tax, service charge, Payments and Receipt.
_Avoid_: Split check, child bill

**Merge**:
Combining the Bills of several Tables into one Bill covering all of them; irreversible except by splitting.
_Avoid_: Join tables, combine

**Waiter Call**:
A Customer asking for a Waiter from the Client Menu, or a "This isn't my table" alert from a scan; all on-duty Waiters are notified, one answers it, and an unanswered call re-notifies and then escalates to the Outlet Admin.
_Avoid_: Service request, bell

**Claim**:
A Waiter taking ownership of a Customer-placed Order; the Order reaches the kitchen only after the Waiter confirms it with the Customer.

**Cancel**:
Stopping an Order Item before it is Ready, by a Customer (own Order, before Claim), a Waiter or Kitchen Crew, with a reason and no PIN Override.
_Avoid_: Void, delete

**Remake**:
A no-charge replacement of a served Order Item that was wrong or bad, made with a reason under an Outlet Admin PIN Override and prioritised on the Kitchen display; the original stays on the Bill.
_Avoid_: Redo, replacement order

**Cancel Reason**:
The mandatory reason chosen from a fixed list when Cancelling: Out of Stock, Customer Request, Wrong Order, or Other with free text. Only Out of Stock triggers Sold Out.

**Void**:
Removing a Ready or Served but unpaid Order Item from a Bill, always requiring a PIN Override and a reason.
_Avoid_: Cancel, delete

**Estimated Wait**:
The queue-based time a Customer is shown until their Order is expected to be ready.
_Avoid_: SLA (when meaning the customer-facing estimate)

## Payment

**Payment**:
A recorded settlement against a Bill by one method (cash, QRIS or card on an external terminal); a Bill may have several. No payment gateway is involved.
_Avoid_: Transaction

**Receipt**:
The printed proof of a closed Bill, produced in two copies: one for the Customer and one kept internally as payment reference. A later copy is marked REPRINT.
_Avoid_: Invoice, struk (in docs)

**PIN Override**:
Approval of a restricted action by entering an authorised person's PIN: Management for manual discounts and Refunds above the cap; Outlet Admin or Management for Voids, Remakes, Unpaid Bills and unverified QRIS.
_Avoid_: Supervisor PIN, manager approval

**Refund**:
Returning money against a paid Bill; capped per Bill for Outlet Admin, above which Management must approve.
_Avoid_: Return, reversal

**Cash Drawer**:
The single till at the Cashier holding all the outlet's cash; every cash Payment and cash Refund ends up here.
_Avoid_: Till, register, pouch

**Opening Float**:
The cash placed in the Cash Drawer at the start of a Cashier shift, entered by the Cashier.
_Avoid_: Starting cash, modal

**Cash Handover**:
A Waiter passing cash taken at the Table to the Cashier, who confirms the amount received; until then the cash is Held by Waiter.
_Avoid_: Deposit, setoran (in docs)

**Held by Waiter**:
Cash a Waiter has taken for a Payment and not yet passed to the Cashier in a Cash Handover; the Waiter cannot clock out while holding any.
_Avoid_: Pocket cash, unsettled cash

**Pending QRIS**:
A QRIS Payment recorded before staff could see it in the merchant app, approved by PIN Override; it must be verified or written off with a reason before the Shift Closing.
_Avoid_: Unverified payment, QRIS hold

**Shift Closing**:
The Cashier's end-of-shift count of the Cash Drawer against expected cash: Opening Float plus Cash Handovers and counter cash, minus change and cash Refunds.
_Avoid_: Cash up, end of day

**Anonymisation**:
Replacing Customer names with "Customer" and deleting Dietary Restrictions and Dietary Notes once the retention period ends, leaving sales figures and Product-side Dietary Warnings intact.
_Avoid_: Deletion (for this specific act)

## HRIS

**Shift**:
A scheduled working period for one Crew member, set by the Outlet Admin or Management; a clock-in up to 2 hours before it belongs to it.
_Avoid_: Roster slot, jadwal (in docs)

**Attendance**:
A Crew member's clock-in and clock-out, made only on the shared HRIS tablet with Crew ID and PIN.
_Avoid_: Timesheet, absen (in docs), presence

**Auto clock-out**:
The system clocking out a Crew member still clocked in 2 hours after their Shift ended; flagged "auto" until the Outlet Admin corrects it with a reason.
_Avoid_: Forced logout, system checkout

**Leave Request**:
A Crew member's request for days off, approved or rejected (with a reason) by an Outlet Admin or Management other than themselves; approved leave shows on the schedule.
_Avoid_: Time-off ticket, cuti (in docs)

**Employee Record**:
The HR details kept beside a Crew member's sign-in account — employment type, start date, phone, emergency contact — maintained by Management.
_Avoid_: Profile, personnel file
