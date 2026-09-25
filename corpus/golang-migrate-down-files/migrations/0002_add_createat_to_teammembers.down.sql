-- Reduced from Mattermost v10.11.0, 000092_add_createat_to_teammembers.down.sql:
-- the down migration drops the column from the wrong table (Reactions, not
-- TeamMembers). A forward migrate never runs it, but read in name order it
-- ran just before its own up file and deleted reactions.createat, which the
-- sink reads.
ALTER TABLE reactions DROP COLUMN IF EXISTS createat;
