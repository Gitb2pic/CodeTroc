-- F0 — socle. Toutes les sessions de la base travaillent en UTC ; les dates
-- sont converties à l'affichage. Les tables métier arrivent avec F1 et F2.
DO $$
BEGIN
  EXECUTE format('ALTER DATABASE %I SET timezone TO %L', current_database(), 'UTC');
END
$$;
