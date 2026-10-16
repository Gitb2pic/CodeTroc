import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { consumeLoginOutcome, useSession } from '../../api/session';
import { Landing } from '../../components/Landing';
import { ErrorBanner, Icon } from '../../components/ui';
import { PERSONAS, SANDBOX, sandboxLogin } from '../../sandbox/runtime';
import { LIVE_ROUTES as R } from '../routes';
import { EmailAuthCard, useAuthMethods } from './EmailAuth';

/** Landing publique du mode API : connexion GitHub, ou choix d'un persona dans le bac à sable. */
export function LoginPage() {
  const { status, login, reload } = useSession();
  const [outcome, setOutcome] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const methods = useAuthMethods();
  const emailOpen = !SANDBOX && methods?.email === true;
  useEffect(() => {
    setOutcome(consumeLoginOutcome());
  }, []);

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

  if (status === 'authenticated') return <Navigate to={R.dash} replace />;

  const primary = (big?: boolean) =>
    SANDBOX ? (
      <button className="btn btn-primary" onClick={() => document.getElementById('essayer')?.scrollIntoView({ behavior: 'smooth' })}
        style={big ? { height: 44, padding: '0 18px', borderRadius: 9, fontSize: 15, gap: 8 } : { gap: 8 }}>
        <Icon name="play" size={big ? 18 : undefined} />
        Essayer le bac à sable
      </button>
    ) : (
      <button className="btn btn-primary" onClick={login} disabled={status === 'loading'}
        style={big ? { height: 44, padding: '0 18px', borderRadius: 9, fontSize: 15, gap: 8 } : { gap: 8 }}>
        <Icon name="github" size={big ? 18 : undefined} />
        Se connecter avec GitHub
      </button>
    );

  const extra =
    outcome || SANDBOX || emailOpen ? (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        {outcome && <ErrorBanner onRetry={SANDBOX ? undefined : login}>{outcome}</ErrorBanner>}
        {emailOpen && <EmailAuthCard />}
        {SANDBOX && (
          <div className="card" style={{ borderRadius: 14, padding: 20, display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              <span className="lp-eyebrow">// bac à sable</span>
              <h2 style={{ margin: 0, fontSize: 20, fontWeight: 600 }}>Choisis qui tu joues</h2>
              <span className="muted" style={{ fontSize: 13.5 }}>
                Le vrai backend et PostgreSQL tournent dans ton navigateur ; la connexion GitHub est simulée. Change de persona à tout moment pour jouer les deux côtés d'un échange.
              </span>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(230px,1fr))', gap: 8 }}>
              {PERSONAS.map((p) => (
                <button key={p.login} className="btn btn-secondary hov-card" disabled={busy} onClick={() => loginAs(p.login)}
                  style={{ height: 'auto', padding: '10px 14px', justifyContent: 'flex-start', gap: 12, textAlign: 'left' }}>
                  <Icon name="github" />
                  <span style={{ flex: 1, display: 'flex', flexDirection: 'column', lineHeight: 1.3, minWidth: 0 }}>
                    <span style={{ fontWeight: 600 }}>{p.login} <span className="muted" style={{ fontWeight: 400 }}>· {p.role}</span></span>
                    <span className="faint" style={{ fontSize: 12 }}>{p.hint}</span>
                  </span>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    ) : null;

  const bonus = SANDBOX ? { amount: 300, note: "offerts dans le bac à sable pour multiplier les essais (50 en production)" } : undefined;
  return <Landing primary={primary} bonus={bonus}>{extra}</Landing>;
}
