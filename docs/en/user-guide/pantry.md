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
at all. A code it does not know opens a single name field right there — type the
name, press Enter, and the camera is ready for the next one. Homebox does not
jump to the new item, because that would break the rhythm.

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

### Registering a barcode by hand

You can also type a barcode into an item's **Pantry** card in the edit form. The
same product in two places is fine — barcodes are not required to be unique, and
a scan that matches several items simply lists them all.
