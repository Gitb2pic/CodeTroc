ALTER TABLE mouvement DROP CONSTRAINT fk_mouvement_jalon;
ALTER TABLE mouvement DROP CONSTRAINT fk_mouvement_echange;
ALTER TABLE compte    DROP CONSTRAINT fk_compte_echange;
DROP TABLE cle_idempotence;
DROP TABLE historique_transition;
DROP TABLE jalon;
DROP TABLE echange;
DROP TYPE statut_jalon;
DROP TYPE statut_echange;
