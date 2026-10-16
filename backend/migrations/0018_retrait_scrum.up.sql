-- Retrait du cadre Scrum (F16) : la plateforme reste un échange de
-- compétences ; les projets, sprints et stories ne font plus partie du produit.
DELETE FROM notification WHERE type IN ('SPRINT_TERMINE', 'JALON_A_RISQUE', 'JALON_EN_RETARD', 'AVENANT_PROPOSE', 'AVENANT_DECIDE');
DROP TABLE outbox_github;
DROP TABLE avenant;
DROP TABLE story;
DROP TABLE sprint;
ALTER TABLE jalon DROP COLUMN github_milestone_no;
DROP TABLE projet_scrum;
ALTER TABLE echange DROP COLUMN duree_sprint_jours;
ALTER TABLE offre DROP COLUMN duree_sprint_jours;
DROP TYPE statut_avenant;
DROP TYPE statut_synchro;
DROP TYPE statut_sprint;
DROP TYPE statut_story;
ALTER TABLE annonce DROP COLUMN duree_sprint_jours;
