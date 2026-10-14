import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { consumeLoginOutcome, useSession } from '../../api/session';
import { ErrorBanner, Icon, Logo } from '../../components/ui';
import { PERSONAS, SANDBOX, sandboxLogin } from '../../sandbox/runtime';
import { LIVE_ROUTES as R } from '../routes';

export function LoginPage() {
  const { status, login, reload } = useSession();
  const [outcome, setOutcome] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const loginAs = async (who: string) => {
    setBusy(true);
    try {
      await sandboxLogin(who);
      await reload();
    } catch (e) {
      setOutcome((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => setOutcome(consumeLoginOutcome()), []);

  if (status === 'authenticated') return <Navigate to={R.dash} replace />;
  return (
    <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 24 }}>
      <div style={{ width: '100%', maxWidth: 440, display: 'flex', flexDirection: 'column', gap: 22, alignItems: 'flex-start' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 9, font: '600 16px var(--sans)' }}>
          <Logo />
          CodeTroc
        </span>
        <h1 className="h1" style={{ fontSize: 34, lineHeight: 1.1 }}>Échange tes compétences de dev contre des crédits, libérés sur preuve.</h1>
        <p className="muted" style={{ margin: 0, fontSize: 15, lineHeight: 1.5 }}>
          Chaque livraison passe par une pull request vérifiée sur GitHub. Les crédits restent en séquestre jusqu'à ta validation.
        </p>
        {outcome && <ErrorBanner style={{ width: '100%', boxSizing: 'border-box' }} onRetry={login}>{outcome}</ErrorBanner>}
        {SANDBOX ? (
          <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: 8 }}>
            <span className="kicker">Bac à sable · choisis qui tu joues (connexion GitHub simulée)</span>
            {PERSONAS.map((p) => (
              <button key={p.login} className="btn btn-secondary hov-card" disabled={busy} onClick={() => loginAs(p.login)}
                style={{ height: 'auto', padding: '10px 14px', justifyContent: 'flex-start', gap: 12, textAlign: 'left' }}>
                <Icon name="github" />
                <span style={{ flex: 1, display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
                  <span style={{ fontWeight: 600 }}>{p.login} <span className="muted" style={{ fontWeight: 400 }}>· {p.role}</span></span>
                  <span className="faint" style={{ fontSize: 12 }}>{p.hint}</span>
                </span>
              </button>
            ))}
          </div>
        ) : (
          <button className="btn btn-primary" onClick={login} disabled={status === 'loading'} style={{ height: 44, fontSize: 14.5, gap: 10, padding: '0 18px' }}>
            <Icon name="github" />
            Se connecter avec GitHub
          </button>
        )}
        <span className="faint" style={{ fontSize: 12.5 }}>
          Lecture seule de ton profil public et de ton e-mail vérifié. 50 crédits offerts à l'inscription.
        </span>
      </div>
    </div>
  );
}
