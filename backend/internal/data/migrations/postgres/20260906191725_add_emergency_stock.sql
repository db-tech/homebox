-- Modify "groups" table
ALTER TABLE "groups" ADD COLUMN "household_size" bigint NULL, ADD COLUMN "emergency_days" bigint NULL, ADD COLUMN "emergency_checklist" text NULL;
-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "net_weight" bigint NULL, ADD COLUMN "emergency_category" character varying NULL;
