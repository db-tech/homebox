# Pantry

The pantry features turn Homebox into something you can run a kitchen or a
drinks cupboard from: things that go off, things that run out, and a record of
what you actually get through.

Everything here is optional. An item with no expiry date, no minimum stock and
no barcode behaves exactly as it always did, so your tools and electronics are
unaffected.

## The three item fields

Open an item, go to **Edit**, and you will find a **Pantry** card:

| Field | What it does |
| --- | --- |
| **Expiry Date** | When this item goes off. Leave empty for anything that doesn't. |
| **Minimum Stock** | The quantity you want to keep around. `0` means "don't track this". |
| **Barcode** | The EAN/UPC printed on the packaging, so the scanner can find it. |

## Expiring Soon

**Pantry → Expiring Soon** lists every item with an expiry date falling inside
the selected window, soonest first. You can switch between 7, 14, 30 and 90
days.

Items that have *already* expired stay on the list rather than dropping off it,
so nothing quietly disappears while it is still sitting in your cupboard. The
badge tells you which is which:

- grey — more than a week away
- amber — within the next seven days
- red — already expired

Items without an expiry date never appear here.

## Below Minimum Stock

**Pantry → Below Minimum Stock** lists items whose quantity has fallen to or
below their minimum. Sitting exactly *at* the minimum counts as low — that is
the point at which you want to buy more, not after you've gone under.

**Copy shopping list** puts the whole list on your clipboard as
`2x Tinned Tomatoes` lines, ready to paste into whatever you shop with.

Items with a minimum stock of `0` are never listed; that is how you mark
something as "not a consumable".

## Consumption log

Every item has a **Consumption** tab recording stock movements:

| Type | Effect on quantity |
| --- | --- |
| **Take out** | decreases |
| **Restock** | increases |
| **Correction** | unchanged — the entry only annotates the log |

The two buttons at the top of the tab are the common case: one tap to take one
out, one tap to put one back. For anything else, set an amount, add a note and
pick the type.

Two things worth knowing:

- You cannot take out more than there is. The request is refused and the stock
  is left alone, so the log never claims a movement that did not happen.
- Deleting a log entry does **not** move the stock back. The entry is a
  historical record; correcting a typo in it should not silently change what is
  in your cupboard. Use a *Correction* entry if the count itself is wrong.

**Pantry → Consumption Statistics** aggregates this per item over 7, 14, 30 or
90 days and shows a per-week average, which is a decent guide to how long your
current stock will last.

## Scanning barcodes

The **Scanner** page handles both kinds of code:

- A **Homebox QR code** works as before and takes you to that item or location.
- Anything else is treated as a **product barcode** and looked up against the
  `barcode` field of your items.

Everything happens on your own server. No barcode is ever sent to an external
product database.

### After scanning

The **After scanning** setting decides what happens when a barcode matches
exactly one item:

- **Ask me** — shows the item with buttons, you decide.
- **Take one out** — immediately records one taken out.
- **Add one** — immediately records one restocked.

With several matches, or with none, you are always asked. The automatic modes
are for working through a shopping bag or a shelf without touching the screen
between items.

### Filling a pantry from scratch

The scanner is built around unpacking a shopping bag or a box, so the loop is
kept as short as possible:

1. Pick the **location** once at the top of the page. It stays set for the whole
   session.
2. Leave **After scanning** on *Add one*.
3. Scan every single package, including duplicates.

A code the pantry already knows is counted up on the spot, with no interaction
at all. A code it does not know opens a small form right there — name, best
before date, minimum stock — and Enter creates the item and hands the camera
back. Homebox does not jump to the new item, because that would break the
rhythm.

The minimum stock stays filled in between items, since a box of tins usually
wants the same one. The date does not, because it differs per product.

### Entering a best-before date

The date is entered by tapping, not typing: on a phone the keyboard covers half
the screen for the sake of four digits. Three taps and you are done.

1. **Day** — the numbers 1 to 31. Most packaging gives no day, so there is a
   **No day given (end of month)** button that skips straight past this.
2. **Month** — 1 to 12.
3. **Year** — this year and the next five.

**Back** returns one step if you hit the wrong number, and the part chosen so
far is shown as you go. A day that the chosen month does not have — 31 followed
by February — resolves to the end of that month rather than spilling into the
next one.

If you would rather type, **type it instead** switches to a text field that
takes the short forms printed on packaging:

| You type | Homebox stores |
| --- | --- |
| `0327` | 31.03.2027 |
| `032027` | 31.03.2027 |
| `03.27` or `03/2027` | 31.03.2027 |
| `12.03.2027` | 12.03.2027 |
| `12032027` | 12.03.2027 |
| `2027-03-12` | 12.03.2027 |

A month without a day resolves to the **last day of that month**, which is what
"mindestens haltbar bis Ende März" means. The field shows how it read your input
before you commit, and refuses to save anything it could not parse — a wrong
best-before date is worse than none.

Scan six identical tins and you type once: the first creates the item, the other
five each add one to it. You never enter a quantity by hand.

### Product name suggestions

When a scanned code is unknown locally, Homebox can ask
[OpenFoodFacts](https://world.openfoodfacts.org) what the product is and
pre-fill the name field, which you then confirm or overwrite.

This is the only place where Homebox talks to a third party, and it is worth
being precise about what that means:

- Only the barcode digits leave your server. No item names, no quantities, no
  location, nothing tied to you or your group.
- It happens only when you scan a code that no local item carries — never in the
  background, never for codes you already have.
- Codes that are not plausible EAN/UPC digits are rejected before any request is
  made, so a stray QR payload cannot be forwarded by accident.
- If OpenFoodFacts is slow or unreachable the scan still works; you just type the
  name yourself.

Set `HBOX_OPTIONS_PRODUCT_LOOKUP=false` to switch it off entirely. Every scan
then stays on your own server.

### Using a handheld scanner

The camera is not the only option. A USB or Bluetooth barcode scanner — the
pistol-grip kind used at a supermarket till — works here with no setup at all.

Such a scanner presents itself to the phone or tablet as a **keyboard**: it
types the digits of the code and presses Enter. The scanner page watches for
that and treats it exactly like a camera read, so lookups, counting up and
creating new items all behave the same.

- **Pairing** is done in the device's Bluetooth settings, like any keyboard. USB
  models work on Android through an OTG adapter.
- The scanner must **send Enter after each scan**. That is the default on most
  models; if not, the manual has a configuration barcode for it.
- Typing into a text field is never intercepted, so a scan fired while you are
  editing a name lands in that field where you can see and correct it.
- Once a handheld scan is recognised the page says so, and you can **turn the
  camera off** to save battery and screen space.

One thing worth knowing: while a Bluetooth keyboard is connected, Android and
iPadOS usually hide the on-screen keyboard. That is mostly welcome here — the
date is chosen by tapping and product names are filled in from the lookup — but
it does make correcting a name awkward until the scanner is disconnected.

### Registering a barcode by hand

You can also type a barcode into an item's **Pantry** card in the edit form. The
same product in two places is fine — barcodes are not required to be unique, and
a scan that matches several items simply lists them all.

## Adding an item by speaking it

A scanner only helps with things that carry a barcode. For everything else,
**Speak it** on the add-item dialog records a few seconds and fills the form in
from what you said. *"Drei Packungen Varta AA Batterien, liegen in der
Werkstattkiste"* becomes a name, a quantity and a description.

Nothing is created by speaking. The fields are filled and you confirm them,
which is not politeness: speech recognition mishears brand names more than
anything else, and an item saved from a misheard name is worse than one typed
slowly. What was heard is shown underneath in quotes, so a mishearing is visible
rather than buried inside a tidied-up name.

### What it can and cannot do

The transcript is read by a language model with **no internet access**. It can
correct *"warta a a"* to *"Varta AA"* and turn *"drei Packungen"* into a
quantity of three from what it already knows. It cannot look the product up, so
it cannot confirm that it exists, and it is instructed not to add details you
did not say. A description mentioning a size or a colour you never mentioned
would be invented, which has no place in an inventory.

### Switching it on

Two keys, because they do two different things and no single provider does both:

- **Profile → Voice entry** takes an **OpenAI** key, which transcribes the
  recording. Anthropic's models do not accept audio at all, which is why the key
  you already have is not enough.
- **Profile → Meal suggestions** takes the **Anthropic** key, which turns the
  transcript into field values. Voice entry reuses it rather than asking for a
  third.

With only the OpenAI key stored the settings page says so. The microphone does
not appear until both are there.

Roughly 0.6 cents per minute of speech for the transcription, plus a fraction of
a cent for reading it. Recordings stop themselves after 30 seconds, and the
server refuses anything over 8 MB, so a microphone left open cannot run up a
bill.

### What leaves your server

- The **recording** goes to OpenAI. Nothing else — no item names, no inventory,
  no identifiers.
- The **transcript** then goes to Anthropic. The audio does not.
- Only when you press the button. Nothing is recorded in the background, and
  with no key stored the browser never asks for the microphone at all.

`HBOX_VOICE_ENABLED=false` forbids the feature outright, whatever any group has
stored. `HBOX_VOICE_API_KEY` is an optional server-wide transcription key for
deployments that would rather keep the secret out of the database.

## Emergency stock

Germany's federal government publishes how much food a household should keep at
home. **Pantry → Emergency stock** (`/emergency`) measures your actual pantry
against it, and carries the official checklist for everything that is not food.

### The figures

Per person for ten days, at 2,200 kcal a day:

| | |
| --- | --- |
| Drinks | 20 l |
| Vegetables, mushrooms | 4.0 kg |
| Grains, bread, potatoes | 3.3 kg |
| Fruit | 2.5 kg |
| Milk and dairy | 2.5 kg |
| Eggs, meat, fish | 1.2 kg |
| Fats and oil | 330 g |

Set how many people and how many days at the top; the targets scale. Ten days is
what the advice builds to, but three is described as already worth having, which
is why the number is adjustable.

Source: the *Ernährungsvorsorge* portal of the Federal Ministry of Food and
Agriculture, and the BBK guide *Vorsorgen für Krisen und Katastrophen*. The
twenty litres are 1.5 l to drink plus 0.5 l to cook with, per person per day.

### Counting a cupboard in kilograms

The recommendation is in kilograms and litres; Homebox counts packages. Two
fields bridge that, both on the item's **Pantry** card:

- **Package size in grams** — what one tin or bottle holds. Millilitres count as
  grams: water is a gram per millilitre and for a stockpile target the rest is
  close enough that a conversion table would be false precision.
- **Emergency stock group** — which of the seven the item counts towards, or
  *not part of the emergency stock*, which is the default and what every tool and
  appliance stays on.

Scanning fills the package size in from the product database, which publishes it
for most barcodes. On the scanner page the group is suggested too and both can
be corrected before the item is created. In the terminal only the size is taken
automatically, because it is the manufacturer's stated content rather than a
guess; a new batch of a product already in the pantry inherits its group.

An item with **no package size counts as nothing**, and so does one with no
group. Neither is estimated. The page lists both sets under **Still to sort out**
with a link straight to each item, because a figure you cannot trust is worse
than a figure that admits what it is missing.

### The checklist

The second half of the page is the BBK's own checklist — medicine cabinet,
hygiene, light and warmth, information, fire safety, go bag, documents. Ticks are
stored per group and saved as you go.

The German wording is quoted from the official publication rather than rewritten;
the English alongside is a plain reading aid, not an official translation.

### Living out of the stockpile

The advice is explicitly to *use* the stock and rotate it rather than seal it
away. That is what the expiry warnings already do, so a stockpile built here
keeps itself current as long as you scan things out of it.

## What can I cook?

**Pantry → What can I cook?** turns what is actually in the cupboard into two or
three concrete dishes, ordered so that the one using the most soon-to-expire
items comes first. Each says which of *your* items it uses and what you would
have to buy.

This is a recipe suggestion only in passing. The point is the other direction:
you have cream that goes off on Thursday, and this tells you what to do about it
tonight.

### Switching it on

It needs an Anthropic API key, and the normal place for it is **Profile → Meal
suggestions**. Paste the key, save, and the button on the pantry page starts
working. **Remove** deletes it again, and with no key stored nothing is ever
sent anywhere.

The key is stored in your database in plain text, the same as notifier URLs
already are. Homebox has no secret store, and encrypting it with a key sitting
in the same database would look like protection without being any. It is never
sent back to the browser: the app only ever learns *whether* there is one.

Anyone in your group can set, replace and use the key — it is a shared
household setting, and using it spends money on that key.

Two environment variables remain, for deployments that would rather not have the
secret in the database at all:

```
HBOX_RECIPES_API_KEY=sk-ant-...              # used by groups that have no key of their own
HBOX_RECIPES_MODEL=claude-haiku-4-5-20251001 # optional
HBOX_RECIPES_ENABLED=false                   # forbid the feature outright
```

`HBOX_RECIPES_ENABLED` is a veto rather than an on switch. Left alone nothing
happens until somebody stores a key; set to `false` the feature disappears
whatever any group has saved.

### What leaves your server

This is the only part of Homebox that sends anything about **what you own** to a
third party, so it is worth being exact:

- Only the **name, quantity and days left** of pantry items. No locations, no
  prices, no descriptions, no notes, no account or group identifier, no barcodes.
- Only **pantry items** — anything carrying a best-before date, a minimum stock
  or a barcode. Tools, furniture and electronics are never included, and neither
  is anything whose quantity is zero.
- Only when you **press the button**. Never on a page load, never in the
  background, never on a schedule.
- With an empty pantry no request is made at all.

The suggestions go to Anthropic's API with the key you configured. Each press
costs a fraction of a cent on the default model. Pressing again asks again
rather than replaying the last answer, because *give me a different idea* is a
reasonable thing to want.

### What the model is and is not allowed to do

It does not work anything out. What is in stock and how many days each item has
left is decided on your server before the request goes out, so a suggestion can
never rest on the model having got the arithmetic of a date wrong.

It also cannot claim you have something you do not. Every ingredient it lists
under *Uses* is checked against the list it was given; anything else is moved to
*You would need*. A dish that quietly assumes an ingredient you do not have is
worse than no suggestion, because it reads as "everything is here" when it is
not.

Quantities in Homebox are counts of packages, not weights, so you get dishes and
a sentence about why — not a recipe with grams it would have to invent.

## The pantry terminal

A tablet on the wall next to the cupboard with a handheld scanner beside it,
working in both directions: unpacking a box into the pantry, and taking things
back out of it. That is what **Pantry → Open the pantry terminal** (`/kiosk`) is
for.

It is deliberately not the scanner page. That page is a form you scroll through,
which is fine on a phone you are holding and wrong here: with a handheld scanner
the result lands below the fold and you would scroll up after every single tin.
Everything on the terminal fits one screen and never moves.

The other rule is that a text field is never focused on its own. Focus in a
field means the next scan is typed into it instead of being booked — quiet
nonsense of exactly the kind a wall device must not produce.

Tap **Start the terminal** once. That single tap is what lets the browser keep
the screen awake and make a sound — neither is granted without one. **Take out**
and **Put in** at the top switch direction.

### Taking things out

The scanned barcode is looked up and **one is taken out**, immediately. There is
no confirmation step, because a confirmation step is the thing you asked to be
rid of.

The screen then says what happened, and so does a sound, so in the normal case
you never look at it:

| | Screen | Sound |
| --- | --- | --- |
| Booked | green, item name and what is left | one short high blip |
| Booked, but now low or empty | amber, plus *below minimum* | two mid beeps |
| Unknown code, or the server did not answer | red | one long low tone |

This matters more than it sounds. The scanner beeps when it *reads* a code, not
when the stock actually moved — two different events, and the difference only
shows up when it hurts. The terminal gives the booking its own voice.

### Putting things in

Pick a **place** once at the top; everything created goes there. Then scan.

The awkward part of filling a pantry is that one product can have two
best-before dates and an item can only hold one. Six tins until March and two
until November are therefore two items — merging them would throw one of the
dates away, and the dates are the reason the pantry view exists at all.

That makes a scan ambiguous on its own: it could be another tin of a batch
already there, or the first of a new one. Only the date settles it, so the date
is the one thing the terminal asks for:

- **A product it has never seen.** The name comes from the product lookup and is
  shown, not focused. Then the date, then it is created.
- **A product it knows.** The existing batches are offered as buttons — *Best
  before 31.03.2027 · 4* — plus **A different date**. Tap one and the tin joins
  that batch.
- **Anything scanned again in the same session** goes straight into the batch
  you settled on, with no tap at all.

So a box of twenty identical tins costs three taps for the first one and one
scan for each of the other nineteen. If one tin in the box has a different date,
**A different date** on the result screen puts the last scan back and asks
again, keeping the product — you never have to say what it is a second time.

While the terminal is waiting for an answer, the button at the bottom left reads
**Back** and steps out of the question rather than reversing a booking. It only
means *Undo* when nothing is pending. One button with two meanings is how you
end up reversing a tin you were happy with because you wanted to correct the one
in your hand.

Batches settled this way are forgotten when you leave or switch direction. The
next box of the same product is a new date, and silently adding it to last
month's batch would put a wrong best-before date on real food.

A batch created by a scan carries **no minimum stock**. The minimum belongs to
the product rather than to one batch of it — see below.

### Undo

**Undo** is always on screen and undoes the last booking, repeatedly if you keep
pressing it.

That is on purpose instead of a guard against scanning the same thing twice:
taking three tins out *is* three scans of the same code, so counting them down
is correct. Scanning one tin twice by accident is the rarer case, and it is
better fixed by a button than prevented by a rule that would break the common
one. A run of the same item is shown as *3 in a row* so a slip is visible.

Putting the item back is recorded as a restock and then both entries are
removed, so the consumption log does not fill up with pairs that cancel out.

### Which batch a scan takes from

Taking out cannot ask which batch you meant — the whole point is that it costs
one scan — so it uses the rule you would follow at the shelf anyway: **the one
that goes off first**.

Items with no best-before date come last, and items already at zero are skipped
rather than blocking the ones that still have stock. When there was more than
one candidate the screen says so.

### Minimum stock across batches

A minimum belongs to the **product**, not to one batch of it. Four tins until
March plus two until November are six tins in the cupboard, so with a minimum of
five that is not a shortage.

**Below Minimum Stock** therefore adds up every item sharing a barcode and
reports the product once, as the batch that runs out first, with the total
alongside. A minimum set on any one batch counts for the whole product, which is
why batches created by a scan do not need one of their own.

Items without a barcode have nothing to group by and are judged on their own
quantity, exactly as before.

### Unresolved scans

A code that no item carries is **not** silently dropped. The tin has left the
cupboard either way, and stock that quietly disagrees with the shelf is worse
than no stock figure at all.

Such codes are kept on the device and counted at the bottom of the screen. Open
the list, add the products on your phone at some point, and tick them off. The
same happens when the stock was already at zero, or when the server could not be
reached.

### Leaving it running

Two things end a wall terminal quietly, and both are handled:

- **The screen locking.** The terminal asks the tablet to keep the screen on
  while it is open. If you would rather let the screen sleep, set the tablet's
  screen lock to **none** — otherwise the lock screen swallows the first scan of
  every visit and you have to scan twice.
- **The session expiring.** A Homebox session lasts a week, four with *stay
  logged in*. The terminal extends its own session while it is open, so it does
  not log itself out mid-month.

If the scanner battery is flat or a code is damaged, **Type a code** at the
bottom takes the digits by hand through exactly the same path.
