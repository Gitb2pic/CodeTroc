import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, HashRouter } from 'react-router-dom';
import { App } from './App';
import { bootSandbox, SANDBOX } from './sandbox/runtime';
import './styles/global.css';
import './styles/icons.css';

// Les builds « artifact » et « sandbox » sont servis comme des pages statiques : routes par le hash.
const Router = import.meta.env.MODE === 'artifact' || SANDBOX ? HashRouter : BrowserRouter;

/** Bac à sable : démarre PostgreSQL et le backend dans le navigateur avant l'application. */
function SandboxBoot() {
  const [step, setStep] = useState('Préparation…');
  const [error, setError] = useState<string | null>(null);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    bootSandbox(setStep).then(() => setReady(true), (e: Error) => setError(e.message));
  }, []);
  if (ready) return <Router><App /></Router>;
  return (
    <div className="app-root" style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 24, background: 'var(--bg)', color: 'var(--tx)' }}>
      <div role="status" aria-live="polite" style={{ maxWidth: 440, display: 'flex', flexDirection: 'column', gap: 10 }}>
        <strong style={{ fontSize: 18 }}>CodeTroc · bac à sable</strong>
        <span className="muted">Le backend Go et PostgreSQL tournent dans ton navigateur : rien n'est envoyé à un serveur.</span>
        {error ? (
          <>
            <span style={{ color: 'var(--fail)' }}>Démarrage impossible : {error}</span>
            <span className="muted" style={{ fontSize: 13 }}>
              Ce navigateur bloque peut-être WebAssembly ou IndexedDB. Le prototype sans backend reste consultable :{' '}
              <a href="https://claude.ai/artifact/PJ3kBoaq6PQYttHdbAcRGB" target="_blank" rel="noreferrer noopener">prototype CodeTroc</a>.
            </span>
          </>
        ) : (
          <span className="mono faint" style={{ fontSize: 13 }}>{step}</span>
        )}
      </div>
    </div>
  );
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {SANDBOX ? (
      <SandboxBoot />
    ) : (
      <Router>
        <App />
      </Router>
    )}
  </StrictMode>,
);
