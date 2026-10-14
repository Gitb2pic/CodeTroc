package app

// Requêtes partagées, enrichies au fil des features.

// balanceSQL calcule le solde disponible et les séquestres d'un membre.
// Les séquestres sont ajoutés avec F6 (échanges).
const balanceSQL = `
	SELECT c.solde, 0::bigint, 0::bigint
	FROM compte c WHERE c.id_utilisateur = $1`

// verifiedDeliveriesSQL compte les jalons libérés au profit de u (F8).
const verifiedDeliveriesSQL = `SELECT 0`

// reviewsCountSQL compte les avis reçus par u (F10).
const reviewsCountSQL = `SELECT 0`
