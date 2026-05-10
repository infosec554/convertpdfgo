-- Revert: drop CASCADE, restore plain RESTRICT (the postgres default).

DO $$
DECLARE
    r RECORD;
    cols TEXT;
BEGIN
    FOR r IN
        SELECT
            con.conrelid::regclass AS tbl,
            cls.relname            AS table_name,
            con.conname,
            con.conkey
        FROM pg_constraint con
        JOIN pg_class cls ON cls.oid = con.conrelid
        WHERE con.contype = 'f'
          AND con.confrelid = 'files'::regclass
          AND pg_get_constraintdef(con.oid) ILIKE '%ON DELETE CASCADE%'
    LOOP
        SELECT string_agg(quote_ident(att.attname), ',') INTO cols
        FROM unnest(r.conkey) AS k(num)
        JOIN pg_attribute att
          ON att.attrelid = r.tbl
         AND att.attnum   = k.num;

        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I',
                       r.table_name, r.conname);
        EXECUTE format(
            'ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%s) REFERENCES files(id)',
            r.table_name, r.conname, cols);
    END LOOP;
END $$;
