import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { api, ApiError, call } from '../../api/client';
import { useSession } from '../../api/session';
import { Logo } from '../../components/ui';
import { Field, FormError, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';

// F18 — connexion, inscription et mot de passe oublié par email.

type Mode = 'connexion' | 'inscription' | 'oubli';

const TABS: [Mode, string][] = [
  ['connexion', 'Se connecter'],
  ['inscription', 'Créer un compte'],
  ['oubli', 'Mot de passe oublié'],
];

const PW_HINT = '10 caractères au moins ; une phrase de passe fait très bien l’affaire.';

/** Formulaires email de l'écran d'accueil. */
export function EmailAuthCard() {
  const { signIn } = useSession();
  const [params] = useSearchParams();
  const [mode, setMode] = useState<Mode>(params.get('vue') === 'oubli' ? 'oubli' : 'connexion');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [pseudo, setPseudo] = useState('');
  const [sent, setSent] = useState<string | null>(null);
  const [unverified, setUnverified] = useState(false);
  const action = useAction();

  const switchTo = (m: Mode) => {
    setMode(m);
    setSent(null);
    setUnverified(false);
    action.clear();
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setUnverified(false);
    const body = { email: email.trim() };
    if (mode === 'connexion') {
      try {
        const t = await call(() => api.POST('/auth/connexion', { body: { ...body, mot_de_passe: password } }));
        signIn(t);
      } catch (err) {
        if (err instanceof ApiError && err.code === 'email-non-confirme') setUnverified(true);
        await action.run(() => Promise.reject(err));
      }
      return;
    }
    if (mode === 'inscription') {
      const ok = await action.run(async () => { await call(() => api.POST('/auth/inscription', { body: { ...body, mot_de_passe: password, ...(pseudo.trim() ? { pseudo: pseudo.trim() } : {}) } })); return true; });
      if (ok) setSent(`Un lien de confirmation a été envoyé à ${body.email}. Clique dessus pour activer ton compte et recevoir tes crédits de bienvenue.`);
      return;
    }
    const ok = await action.run(async () => { await call(() => api.POST('/auth/mot-de-passe/oubli', { body })); return true; });
    if (ok) setSent(`Si un compte existe pour ${body.email}, un lien de réinitialisation vient d’y être envoyé (valable 1 heure).`);
  };

  const resend = async () => {
    const ok = await action.run(async () => { await call(() => api.POST('/auth/email/renvoyer', { body: { email: email.trim() } })); return true; });
    if (ok) {
      setUnverified(false);
      setSent(`Nouveau lien envoyé à ${email.trim()}.`);
    }
  };

  return (
    <div className="card" style={{ borderRadius: 14, padding: 20, display: 'flex', flexDirection: 'column', gap: 14, maxWidth: 460, width: '100%', marginInline: 'auto' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        <span className="lp-eyebrow">// sans GitHub</span>
        <h2 style={{ margin: 0, fontSize: 20, fontWeight: 600 }}>Par email</h2>
        <span className="muted" style={{ fontSize: 13.5 }}>
          Pour demander de l’aide. Tu pourras lier GitHub plus tard depuis ton profil pour proposer tes services.
        </span>
      </div>
      <div role="tablist" style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
        {TABS.map(([m, label]) => (
          <button key={m} type="button" role="tab" aria-selected={mode === m} onClick={() => switchTo(m)}
            className={mode === m ? 'btn btn-secondary' : 'btn btn-link'} style={{ height: 30, padding: '0 10px', fontSize: 12.5 }}>
            {label}
          </button>
        ))}
      </div>
      {sent ? (
        <div role="status" style={{ display: 'flex', flexDirection: 'column', gap: 10, fontSize: 13.5 }}>
          <span>{sent}</span>
          <button type="button" className="btn btn-link" style={{ alignSelf: 'flex-start', height: 30, fontSize: 12.5 }} onClick={() => switchTo('connexion')}>Retour à la connexion</button>
        </div>
      ) : (
        <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Field label="Email">
            <input className="field" type="email" autoComplete="email" required maxLength={254} value={email} onChange={(e) => setEmail(e.target.value)} />
          </Field>
          {mode !== 'oubli' && (
            <Field label="Mot de passe" hint={mode === 'inscription' ? PW_HINT : undefined}>
              <input className="field" type="password" required minLength={mode === 'inscription' ? 10 : 1} maxLength={128}
                autoComplete={mode === 'inscription' ? 'new-password' : 'current-password'} value={password} onChange={(e) => setPassword(e.target.value)} />
            </Field>
          )}
          {mode === 'inscription' && (
            <Field label="Pseudo (facultatif)" hint="Déduit de ton adresse si vide.">
              <input className="field" minLength={3} maxLength={50} pattern="[A-Za-z0-9][A-Za-z0-9_\-]{2,49}" value={pseudo} onChange={(e) => setPseudo(e.target.value)} />
            </Field>
          )}
          <FormError>{action.error}</FormError>
          {unverified && (
            <button type="button" className="btn btn-link" disabled={action.busy} onClick={resend} style={{ alignSelf: 'flex-start', height: 30, fontSize: 12.5 }}>
              Renvoyer le lien de confirmation
            </button>
          )}
          <button type="submit" className="btn btn-primary" disabled={action.busy}>
            {mode === 'connexion' ? 'Se connecter' : mode === 'inscription' ? 'Créer mon compte' : 'Recevoir un lien'}
          </button>
        </form>
      )}
    </div>
  );
}

/** Les modes de connexion ouverts sur ce serveur ; null pendant le chargement. */
export function useAuthMethods() {
  const [methods, setMethods] = useState<{ github: boolean; email: boolean } | null>(null);
  useEffect(() => {
    let live = true;
    call(() => api.GET('/auth/methodes'))
      .then((m) => live && setMethods(m))
      .catch(() => live && setMethods({ github: true, email: false }));
    return () => {
      live = false;
    };
  }, []);
  return methods;
}

function PublicShell({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <main style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 16, background: 'var(--bg)' }}>
      <div className="card" style={{ padding: 24, width: '100%', maxWidth: 440, display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Link to={R.landing} aria-label="Accueil CodeTroc" style={{ alignSelf: 'flex-start' }}><Logo /></Link>
        <h1 style={{ margin: 0, fontSize: 20, fontWeight: 600 }}>{title}</h1>
        {children}
      </div>
    </main>
  );
}

/** /confirmer-email?jeton=… : confirme l'adresse dès l'ouverture du lien. */
export function ConfirmEmailPage() {
  const [params] = useSearchParams();
  const jeton = params.get('jeton') ?? '';
  const [state, setState] = useState<'pending' | 'ok' | string>('pending');
  const started = useRef(false);
  useEffect(() => {
    if (started.current) return; // StrictMode : un jeton ne sert qu'une fois
    started.current = true;
    call(() => api.POST('/auth/email/confirmer', { body: { jeton } }))
      .then(() => setState('ok'))
      .catch((e: Error) => setState(e.message));
  }, [jeton]);
  return (
    <PublicShell title="Confirmation de l’adresse">
      {state === 'pending' && <span aria-busy="true" className="muted">Vérification du lien…</span>}
      {state === 'ok' && (
        <>
          <span role="status">Adresse confirmée : tes crédits de bienvenue t’attendent. Tu peux te connecter.</span>
          <Link className="btn btn-primary" to={R.landing}>Se connecter</Link>
        </>
      )}
      {state !== 'pending' && state !== 'ok' && (
        <>
          <FormError>{state}</FormError>
          <Link className="btn btn-secondary" to={R.landing}>Retour à l’accueil</Link>
        </>
      )}
    </PublicShell>
  );
}

/** /reinitialiser-mot-de-passe?jeton=… : choix d'un nouveau mot de passe. */
export function ResetPasswordPage() {
  const [params] = useSearchParams();
  const jeton = params.get('jeton') ?? '';
  const navigate = useNavigate();
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [done, setDone] = useState(false);
  const action = useAction();
  if (!jeton) return <Navigate to={R.landing} replace />;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (password !== confirm) {
      await action.run(() => Promise.reject(new Error('Les deux mots de passe ne correspondent pas.')));
      return;
    }
    const ok = await action.run(async () => { await call(() => api.POST('/auth/mot-de-passe/reinitialiser', { body: { jeton, mot_de_passe: password } })); return true; });
    if (ok) setDone(true);
  };

  return (
    <PublicShell title="Nouveau mot de passe">
      {done ? (
        <>
          <span role="status">Mot de passe changé. Tes autres sessions ont été fermées.</span>
          <button type="button" className="btn btn-primary" onClick={() => navigate(R.landing)}>Se connecter</button>
        </>
      ) : (
        <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Field label="Nouveau mot de passe" hint={PW_HINT}>
            <input className="field" type="password" autoComplete="new-password" required minLength={10} maxLength={128} value={password} onChange={(e) => setPassword(e.target.value)} />
          </Field>
          <Field label="Confirmer">
            <input className="field" type="password" autoComplete="new-password" required minLength={10} maxLength={128} value={confirm} onChange={(e) => setConfirm(e.target.value)} />
          </Field>
          <FormError>{action.error}</FormError>
          <button type="submit" className="btn btn-primary" disabled={action.busy}>Enregistrer</button>
        </form>
      )}
    </PublicShell>
  );
}

/** Remplace un formulaire réservé aux comptes liés à GitHub (D25). */
export function NeedsGitHub({ what }: { what: string }) {
  return (
    <div className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 10 }}>
      <h2 className="h2">Compte GitHub requis</h2>
      <span className="muted" style={{ fontSize: 13.5 }}>
        Pour {what}, lie ton compte GitHub depuis ton profil : c’est lui qui prouve tes livraisons.
      </span>
      <Link className="btn btn-primary" to={R.profile} style={{ alignSelf: 'flex-start' }}>Aller à mon profil</Link>
    </div>
  );
}
