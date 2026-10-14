package app

// balanceSQL calcule le solde disponible et les séquestres d'un membre.
// Les séquestres sont ajoutés avec F6 (échanges).
const balanceSQL = `
	SELECT c.solde, 0::bigint, 0::bigint
	FROM compte c WHERE c.id_utilisateur = $1`
