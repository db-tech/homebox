-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "expiry_date" timestamptz NULL, ADD COLUMN "min_stock" bigint NOT NULL DEFAULT 0, ADD COLUMN "barcode" character varying NULL;
-- Create index "item_barcode" to table: "items"
CREATE INDEX "item_barcode" ON "items" ("barcode");
-- Create index "item_expiry_date" to table: "items"
CREATE INDEX "item_expiry_date" ON "items" ("expiry_date");
-- Create "consumption_entries" table
CREATE TABLE "consumption_entries" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "date" timestamptz NOT NULL, "amount" bigint NOT NULL, "type" character varying NOT NULL, "note" character varying NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "consumption_entries_items_consumption_entries" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "consumptionentry_item_id_date" to table: "consumption_entries"
CREATE INDEX "consumptionentry_item_id_date" ON "consumption_entries" ("item_id", "date");
