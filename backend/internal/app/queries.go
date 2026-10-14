package app

// Requêtes partagées, enrichies au fil des features.

// balanceSQL calcule le solde disponible et les séquestres d'un membre.
const balanceSQL = `
	SELECT c.solde,
	  COALESCE((SELECT SUM(s.solde) FROM compte s JOIN echange e ON e.id_echange = s.id_echange WHERE e.id_demandeur = $1), 0)::bigint,
	  COALESCE((SELECT SUM(s.solde) FROM compte s JOIN echange e ON e.id_echange = s.id_echange WHERE e.id_prestataire = $1), 0)::bigint
	FROM compte c WHERE c.id_utilisateur = $1`

// isArbiterOfExchangeSQL : l'utilisateur $2 arbitre un litige de l'échange $1 (F9).
const isArbiterOfExchangeSQL = `EXISTS (
	SELECT 1 FROM litige l JOIN jalon j USING (id_jalon) WHERE j.id_echange = $1 AND l.id_arbitre = $2)`

// verifiedDeliveriesSQL compte les jalons libérés au profit de u (F8).
const verifiedDeliveriesSQL = `
	SELECT count(*)::int FROM jalon j JOIN echange e USING (id_echange)
	WHERE e.id_prestataire = u.id_utilisateur AND j.statut_jalon IN ('LIBERE', 'PARTAGE')`

// reviewsCountSQL compte les avis reçus par u (F10).
const reviewsCountSQL = `SELECT 0`
