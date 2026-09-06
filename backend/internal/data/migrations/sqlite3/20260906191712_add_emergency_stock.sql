-- Add column "household_size" to table: "groups"
ALTER TABLE `groups` ADD COLUMN `household_size` integer NULL;
-- Add column "emergency_days" to table: "groups"
ALTER TABLE `groups` ADD COLUMN `emergency_days` integer NULL;
-- Add column "emergency_checklist" to table: "groups"
ALTER TABLE `groups` ADD COLUMN `emergency_checklist` text NULL;
-- Add column "net_weight" to table: "items"
ALTER TABLE `items` ADD COLUMN `net_weight` integer NULL;
-- Add column "emergency_category" to table: "items"
ALTER TABLE `items` ADD COLUMN `emergency_category` text NULL;
