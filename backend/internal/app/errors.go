package app

import "fmt"

// Kind classe une erreur métier ; la couche HTTP en déduit le code de statut.
type Kind int

const (
	KindInvalid         Kind = iota + 1 // 400 : requête mal formée
	KindUnauthenticated                 // 401
	KindForbidden                       // 403 : ressource visible, action interdite
	KindNotFound                        // 404 : inexistante ou invisible pour l'appelant (D11)
	KindConflict                        // 409 : incompatible avec l'état courant
	KindUnprocessable                   // 422 : règle métier non respectée
	KindGone                            // 410 : ressource définitivement indisponible
	KindUnavailable                     // 503
)

// Error est une erreur métier présentable au client.
type Error struct {
	Kind   Kind
	Code   string // slug du type de problème, ex. "solde-insuffisant"
	Title  string
	Detail string
}

func (e *Error) Error() string { return fmt.Sprintf("%s : %s", e.Code, e.Detail) }

func newErr(k Kind, code, title, detail string) *Error {
	return &Error{Kind: k, Code: code, Title: title, Detail: detail}
}

// Erreurs communes à plusieurs features.
var (
	ErrNotFound        = newErr(KindNotFound, "introuvable", "Ressource introuvable", "")
	ErrUnauthenticated = newErr(KindUnauthenticated, "non-authentifie", "Authentification requise", "Jeton absent, expiré ou invalide.")
)

func invalid(code, detail string) *Error {
	return newErr(KindInvalid, code, "Requête invalide", detail)
}

func conflict(code, detail string) *Error {
	return newErr(KindConflict, code, "Action impossible dans l'état actuel", detail)
}

func unprocessable(code, detail string) *Error {
	return newErr(KindUnprocessable, code, "Règle métier non respectée", detail)
}

func forbidden(code, detail string) *Error {
	return newErr(KindForbidden, code, "Action non autorisée", detail)
}
