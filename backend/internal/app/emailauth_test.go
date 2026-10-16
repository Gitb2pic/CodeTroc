package app

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureMailer garde les emails envoyés.
type captureMailer struct {
	mu   sync.Mutex
	sent []Email
}

func (m *captureMailer) Send(_ context.Context, e Email) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, e)
	return nil
}

func (m *captureMailer) last(t *testing.T) Email {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	require.NotEmpty(t, m.sent, "aucun email envoyé")
	return m.sent[len(m.sent)-1]
}

func (m *captureMailer) count() int { m.mu.Lock(); defer m.mu.Unlock(); return len(m.sent) }

var jetonRe = regexp.MustCompile(`jeton=([A-Za-z0-9_-]+)`)

func linkToken(t *testing.T, e Email) string {
	t.Helper()
	m := jetonRe.FindStringSubmatch(e.Text)
	require.NotNil(t, m, "pas de lien dans %q", e.Text)
	return m[1]
}

func newMailEnv(t *testing.T) (*env, *captureMailer) {
	m := &captureMailer{}
	return newEnv(t, func(d *Deps) { d.Mailer = m; d.AppURL = "https://codetroc.test" }), m
}

// registerConfirmed inscrit et confirme un compte email, puis le connecte.
func registerConfirmed(t *testing.T, e *env, m *captureMailer, email string) Session {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, e.svc.Register(ctx, RegisterInput{Email: email, Password: "un mot de passe solide"}))
	require.NoError(t, e.svc.ConfirmEmail(ctx, linkToken(t, m.last(t))))
	s, err := e.svc.LoginWithPassword(ctx, email, "un mot de passe solide")
	require.NoError(t, err)
	return s
}

func TestEmailAuth_DisabledWithoutMailer(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	assert.False(t, e.svc.EmailAuthEnabled())
	requireKind(t, e.svc.Register(ctx, RegisterInput{Email: "a@b.fr", Password: "un mot de passe solide"}), KindUnavailable, "inscription-email-indisponible")
	_, err := e.svc.LoginWithPassword(ctx, "a@b.fr", "x")
	requireKind(t, err, KindUnavailable, "inscription-email-indisponible")
}

func TestEmailAuth_RegisterConfirmLogin(t *testing.T) {
	e, m := newMailEnv(t)
	ctx := context.Background()

	require.NoError(t, e.svc.Register(ctx, RegisterInput{Email: "Lea.Martin@Example.com", Password: "un mot de passe solide"}))
	mail := m.last(t)
	assert.Equal(t, "lea.martin@example.com", mail.To)
	assert.Contains(t, mail.Text, "https://codetroc.test/confirmer-email?jeton=")

	// Non confirmé : connexion refusée, pas de bonus.
	_, err := e.svc.LoginWithPassword(ctx, "lea.martin@example.com", "un mot de passe solide")
	requireKind(t, err, KindForbidden, "email-non-confirme")

	tok := linkToken(t, mail)
	require.NoError(t, e.svc.ConfirmEmail(ctx, tok))
	requireKind(t, e.svc.ConfirmEmail(ctx, tok), KindInvalid, "lien-invalide")

	s, err := e.svc.LoginWithPassword(ctx, "LEA.MARTIN@example.com", "un mot de passe solide")
	require.NoError(t, err)
	assert.Equal(t, "lea-martin", s.User.Pseudo)
	assert.True(t, s.User.EmailVerified)
	assert.False(t, s.User.HasGitHub)
	assert.NotEmpty(t, s.RefreshToken)
	a := e.actor(t, s)
	assert.Equal(t, int64(50), balanceOf(t, e, a).Available, "bonus versé à la confirmation")

	_, err = e.svc.LoginWithPassword(ctx, "lea.martin@example.com", "mauvais mot de passe")
	requireKind(t, err, KindUnauthenticated, "identifiants-invalides")
	_, err = e.svc.LoginWithPassword(ctx, "inconnu@example.com", "un mot de passe solide")
	requireKind(t, err, KindUnauthenticated, "identifiants-invalides")
	requireLedgerConsistent(t, e)
}

func TestEmailAuth_RegisterValidation(t *testing.T) {
	e, m := newMailEnv(t)
	ctx := context.Background()
	requireKind(t, e.svc.Register(ctx, RegisterInput{Email: "pas-une-adresse", Password: "un mot de passe solide"}), KindInvalid, "")
	requireKind(t, e.svc.Register(ctx, RegisterInput{Email: "a@b.fr", Password: "court"}), KindInvalid, "mot-de-passe-faible")
	requireKind(t, e.svc.Register(ctx, RegisterInput{Email: "a@b.fr", Password: "un mot de passe solide", Pseudo: "-x"}), KindInvalid, "")
	assert.Zero(t, m.count())
}

func TestEmailAuth_ExistingAddressIsNotRevealed(t *testing.T) {
	e, m := newMailEnv(t)
	ctx := context.Background()
	require.NoError(t, e.svc.Register(ctx, RegisterInput{Email: "sami@example.com", Password: "un mot de passe solide"}))
	require.NoError(t, e.svc.Register(ctx, RegisterInput{Email: "SAMI@example.com", Password: "un autre mot de passe"}))
	assert.Equal(t, 2, m.count())
	assert.Equal(t, "Ton compte CodeTroc existe déjà", m.last(t).Subject)
	assert.NotContains(t, m.last(t).Text, "jeton=")

	var n int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM utilisateur WHERE lower(email) = 'sami@example.com'`).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestEmailAuth_PseudoCollisionAndChoice(t *testing.T) {
	e, m := newMailEnv(t)
	ctx := context.Background()
	e.login(t, "noe")
	require.NoError(t, e.svc.Register(ctx, RegisterInput{Email: "noe@example.com", Password: "un mot de passe solide"}))
	require.NoError(t, e.svc.Register(ctx, RegisterInput{Email: "x@example.com", Password: "un mot de passe solide", Pseudo: "choisi"}))
	require.Equal(t, 2, m.count())
	var pseudos []string
	rows, err := e.db.Query(ctx, `SELECT pseudo FROM utilisateur WHERE mot_de_passe_hash IS NOT NULL ORDER BY email`)
	require.NoError(t, err)
	for rows.Next() {
		var p string
		require.NoError(t, rows.Scan(&p))
		pseudos = append(pseudos, p)
	}
	require.Len(t, pseudos, 2)
	assert.Regexp(t, `^noe-[0-9a-f]{4}$`, pseudos[0])
	assert.Equal(t, "choisi", pseudos[1])
}

func TestEmailAuth_ConfirmationExpiresAndResend(t *testing.T) {
	e, m := newMailEnv(t)
	ctx := context.Background()
	require.NoError(t, e.svc.Register(ctx, RegisterInput{Email: "theo@example.com", Password: "un mot de passe solide"}))
	first := linkToken(t, m.last(t))

	require.NoError(t, e.svc.ResendConfirmation(ctx, "theo@example.com"))
	second := linkToken(t, m.last(t))
	requireKind(t, e.svc.ConfirmEmail(ctx, first), KindInvalid, "lien-invalide")

	e.clock.Advance(49 * time.Hour)
	requireKind(t, e.svc.ConfirmEmail(ctx, second), KindInvalid, "lien-invalide")

	require.NoError(t, e.svc.ResendConfirmation(ctx, "theo@example.com"))
	require.NoError(t, e.svc.ConfirmEmail(ctx, linkToken(t, m.last(t))))

	before := m.count()
	require.NoError(t, e.svc.ResendConfirmation(ctx, "theo@example.com"), "déjà confirmé : sans effet")
	require.NoError(t, e.svc.ResendConfirmation(ctx, "inconnu@example.com"))
	assert.Equal(t, before, m.count())
}

func TestEmailAuth_PasswordReset(t *testing.T) {
	e, m := newMailEnv(t)
	ctx := context.Background()
	s := registerConfirmed(t, e, m, "nina@example.com")

	before := m.count()
	require.NoError(t, e.svc.RequestPasswordReset(ctx, "inconnu@example.com"))
	assert.Equal(t, before, m.count(), "adresse inconnue : aucun email, même réponse")

	require.NoError(t, e.svc.RequestPasswordReset(ctx, "nina@example.com"))
	tok := linkToken(t, m.last(t))
	assert.Contains(t, m.last(t).Text, "/reinitialiser-mot-de-passe?jeton=")

	requireKind(t, e.svc.ResetPassword(ctx, tok, "court"), KindInvalid, "mot-de-passe-faible")
	require.NoError(t, e.svc.ResetPassword(ctx, tok, "nouveau mot de passe"))
	requireKind(t, e.svc.ResetPassword(ctx, tok, "encore un autre"), KindInvalid, "lien-invalide")

	_, err := e.svc.Refresh(ctx, s.RefreshToken)
	require.Error(t, err, "les sessions ouvertes sont fermées")
	_, err = e.svc.LoginWithPassword(ctx, "nina@example.com", "un mot de passe solide")
	requireKind(t, err, KindUnauthenticated, "identifiants-invalides")
	_, err = e.svc.LoginWithPassword(ctx, "nina@example.com", "nouveau mot de passe")
	require.NoError(t, err)

	// Le lien expire au bout d'une heure.
	require.NoError(t, e.svc.RequestPasswordReset(ctx, "nina@example.com"))
	tok = linkToken(t, m.last(t))
	e.clock.Advance(61 * time.Minute)
	requireKind(t, e.svc.ResetPassword(ctx, tok, "nouveau mot de passe 2"), KindInvalid, "lien-invalide")
}

func TestEmailAuth_ResetConfirmsUnverifiedAddressOnce(t *testing.T) {
	e, m := newMailEnv(t)
	ctx := context.Background()
	require.NoError(t, e.svc.Register(ctx, RegisterInput{Email: "zoe@example.com", Password: "un mot de passe solide"}))
	confirm := linkToken(t, m.last(t))
	require.NoError(t, e.svc.RequestPasswordReset(ctx, "zoe@example.com"))
	require.NoError(t, e.svc.ResetPassword(ctx, linkToken(t, m.last(t)), "nouveau mot de passe"))
	require.NoError(t, e.svc.ConfirmEmail(ctx, confirm), "lien encore valide, sans second bonus")

	s, err := e.svc.LoginWithPassword(ctx, "zoe@example.com", "nouveau mot de passe")
	require.NoError(t, err)
	assert.Equal(t, int64(50), balanceOf(t, e, e.actor(t, s)).Available)
	requireLedgerConsistent(t, e)
}

func TestEmailAuth_WithoutGitHubCannotOfferOrApply(t *testing.T) {
	e, m := newMailEnv(t)
	ctx := context.Background()
	a := e.actor(t, registerConfirmed(t, e, m, "client@example.com"))

	_, err := e.svc.CreateOffer(ctx, a, OfferInput{PackID: packID(t, e, "Pipeline")})
	requireKind(t, err, KindForbidden, "github-requis")

	// Il peut publier une annonce (demandeur).
	credit(t, e, a.ID, 100)
	ann, err := e.svc.PublishAnnouncement(ctx, a, annInput(t, e))
	require.NoError(t, err)

	other := e.actor(t, registerConfirmed(t, e, m, "autre@example.com"))
	_, err = e.svc.Apply(ctx, other, ann.ID, appInput(100))
	requireKind(t, err, KindForbidden, "github-requis")

	// Un membre GitHub peut candidater.
	dev := e.actor(t, e.login(t, "dev"))
	_, err = e.svc.Apply(ctx, dev, ann.ID, appInput(100))
	require.NoError(t, err)
}

func TestEmailAuth_LinkGitHub(t *testing.T) {
	m := &captureMailer{}
	e := newEnv(t, func(d *Deps) { d.Mailer = m; d.Settings.MinGitHubAccountAge = 30 * 24 * time.Hour })
	ctx := context.Background()
	s := registerConfirmed(t, e, m, "lina@example.com")
	a := e.actor(t, s)

	link := func(id GitHubIdentity) (Session, error) {
		code := "lier-" + id.Login
		e.oauth.add(code, id)
		_, state, err := e.svc.StartGitHubLink(ctx, a)
		require.NoError(t, err)
		return e.svc.CompleteLogin(ctx, state, state, code)
	}

	// Compte GitHub déjà utilisé par un autre membre : refusé.
	taken := GitHubIdentity{ID: 9001, Login: "pris", CreatedAt: e.clock.Now().AddDate(-2, 0, 0)}
	e.loginAs(t, taken)
	_, err := link(taken)
	requireKind(t, err, KindConflict, "github-deja-lie")

	// Compte GitHub trop récent : refusé comme à la connexion.
	_, err = link(GitHubIdentity{ID: 9002, Login: "neuf", CreatedAt: e.clock.Now()})
	requireKind(t, err, KindForbidden, "")

	linked, err := link(GitHubIdentity{ID: 9003, Login: "lina-gh", CreatedAt: e.clock.Now().AddDate(-2, 0, 0)})
	require.NoError(t, err)
	assert.Equal(t, a.ID, linked.User.ID, "même compte, GitHub lié")
	assert.True(t, linked.User.HasGitHub)
	assert.Equal(t, "lina-gh", linked.User.GitHubLogin)
	assert.Equal(t, int64(50), balanceOf(t, e, a).Available, "pas de second bonus")

	_, _, err = e.svc.StartGitHubLink(ctx, a)
	requireKind(t, err, KindConflict, "github-deja-present")

	// Connexion GitHub ensuite : même compte ; le mot de passe marche toujours.
	viaGH := e.loginAs(t, GitHubIdentity{ID: 9003, Login: "lina-gh", CreatedAt: e.clock.Now().AddDate(-2, 0, 0)})
	assert.Equal(t, a.ID, viaGH.User.ID)
	_, err = e.svc.LoginWithPassword(ctx, "lina@example.com", "un mot de passe solide")
	require.NoError(t, err)

	_, err = e.svc.CreateOffer(ctx, a, OfferInput{PackID: packID(t, e, "Pipeline")})
	require.Error(t, err)
	var ae *Error
	if assert.ErrorAs(t, err, &ae) {
		assert.NotEqual(t, "github-requis", ae.Code)
	}
}

func TestEmailAuth_DeletedAccountGetsNoSecondBonus(t *testing.T) {
	e, m := newMailEnv(t)
	ctx := context.Background()
	a := e.actor(t, registerConfirmed(t, e, m, "re@example.com"))
	require.NoError(t, e.svc.DeleteAccount(ctx, a))

	s := registerConfirmed(t, e, m, "re@example.com")
	assert.NotEqual(t, a.ID, s.User.ID)
	assert.Zero(t, balanceOf(t, e, e.actor(t, s)).Available, "même adresse : pas de nouveau bonus")
	requireLedgerConsistent(t, e)
}
