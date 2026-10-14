import type { MouseEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Icon, Logo } from '../components/ui';
import { WELCOME_BONUS } from '../data/fixtures';
import { ROUTES } from '../routes';

const wrap = { maxWidth: 1200, margin: '0 auto', padding: '72px 32px' } as const;
const eyebrow = { font: '500 12px/1 var(--mono)', color: 'var(--ac-tx)' } as const;
const h2 = { margin: 0, fontSize: 34, letterSpacing: '-.03em', fontWeight: 600, textWrap: 'balance' } as const;

function GithubButton({ big, onClick }: { big?: boolean; onClick: () => void }) {
  return (
    <button className="btn btn-primary" onClick={onClick} style={big ? { height: 44, padding: '0 18px', borderRadius: 9, fontSize: 15, gap: 8 } : { gap: 8 }}>
      <Icon name="github" size={big ? 18 : undefined} />
      Se connecter avec GitHub
    </button>
  );
}

function Check({ name, children }: { name: string; children: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '8px 10px', border: '1px solid var(--bd)', borderRadius: 8, font: '12px var(--mono)', color: 'var(--tx2)' }}>
      <Icon name="check" size={14} style={{ color: 'var(--ok)' }} />
      {name}
      <span style={{ marginLeft: 'auto', color: 'var(--tx3)' }}>{children}</span>
    </div>
  );
}

const STEPS = [
  ['01', 'package', 'Propose un pack', "Un livrable, des critères d'acceptation, un prix en crédits. Pas de négo sans fin : ce qui est écrit est ce qui est livré."],
  ['02', 'git-pull-request', 'Livre avec preuve', 'Tu ouvres une PR. Quand elle est mergée et que la CI passe, le jalon est prêt à être validé et les crédits se débloquent.'],
  ['03', 'coins', 'Dépense tes crédits', "Tes crédits valent chez n'importe quel membre : infra, front, tests, revue de sécurité. Ils ne s'achètent pas, ils se gagnent."],
];

export function LandingPage() {
  const navigate = useNavigate();
  const toOnboarding = () => navigate(ROUTES.onboarding);
  const navLink = { color: 'var(--tx2)' };
  // Défilement explicite : un lien #ancre casserait le routage par hash.
  const scrollTo = (id: string) => (e: MouseEvent) => {
    e.preventDefault();
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <div>
      <header style={{ position: 'sticky', top: 0, zIndex: 10, background: 'color-mix(in oklab,var(--bg) 82%,transparent)', backdropFilter: 'blur(12px)', borderBottom: '1px solid var(--bd)' }}>
        <div style={{ maxWidth: 1200, margin: '0 auto', padding: '0 32px', height: 60, display: 'flex', alignItems: 'center', gap: 32 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 9, fontWeight: 600, fontSize: 15, letterSpacing: '-.01em' }}>
            <Logo />
            CodeTroc
          </div>
          <nav style={{ display: 'flex', gap: 22, fontSize: 13.5, whiteSpace: 'nowrap' }}>
            <a href="#etapes" onClick={scrollTo('etapes')} style={navLink}>Comment ça marche</a>
            <a href="#preuves" onClick={scrollTo('preuves')} style={navLink}>Preuves GitHub</a>
            <a href="#coffre" onClick={scrollTo('coffre')} style={navLink}>Coffre d'accès</a>
            <a href={ROUTES.cat} onClick={(e) => { e.preventDefault(); navigate(ROUTES.cat); }} style={navLink}>Catalogue</a>
          </nav>
          <div style={{ flex: 1 }} />
          <GithubButton onClick={toOnboarding} />
        </div>
      </header>

      <section style={{ maxWidth: 1200, minWidth: 1100, margin: '0 auto', padding: '88px 32px 72px', display: 'grid', gridTemplateColumns: 'minmax(0,1.05fr) minmax(0,1fr)', gap: 64, alignItems: 'center', boxSizing: 'border-box' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          <div style={{ display: 'inline-flex', alignSelf: 'flex-start', alignItems: 'center', gap: 8, height: 26, padding: '0 10px', border: '1px solid var(--bd2)', borderRadius: 999, font: '500 12px/1 var(--mono)', color: 'var(--tx2)' }}>
            <span style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--ok)' }} />0 € échangé depuis le lancement. C'est voulu.
          </div>
          <h1 style={{ margin: 0, fontSize: 58, lineHeight: 1.02, letterSpacing: '-.04em', fontWeight: 600, textWrap: 'balance' }}>Échange ton code contre du code.</h1>
          <p style={{ margin: 0, fontSize: 18, lineHeight: 1.55, color: 'var(--tx2)', maxWidth: 520, textWrap: 'pretty' }}>
            Tu livres un pack, tu gagnes des crédits, tu les dépenses chez n'importe quel membre. Chaque crédit libéré est adossé à une preuve GitHub : PR mergée, CI au vert.
          </p>
          <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
            <GithubButton big onClick={toOnboarding} />
            <button className="btn btn-secondary" onClick={() => navigate(ROUTES.cat)} style={{ height: 44, padding: '0 18px', borderRadius: 9, fontSize: 15, gap: 8 }}>
              Parcourir les packs
              <Icon name="arrow-right" />
            </button>
          </div>
          <div style={{ display: 'flex', gap: 18, font: '12px/1.4 var(--mono)', color: 'var(--tx3)', flexWrap: 'wrap' }}>
            <span>Aucun paiement</span><span>·</span><span>OAuth en lecture seule</span><span>·</span><span>Accès révocables</span>
          </div>
        </div>

        <div className="card" style={{ borderRadius: 14, boxShadow: '0 30px 80px -30px rgba(0,0,0,.6)', overflow: 'hidden' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '14px 16px', borderBottom: '1px solid var(--bd)' }}>
            <span className="mono faint" style={{ fontSize: 12, fontWeight: 500, whiteSpace: 'nowrap' }}>EX-0142</span>
            <span style={{ fontWeight: 600, fontSize: 13.5 }}>Pipeline CI/CD GitHub Actions</span>
            <div style={{ flex: 1 }} />
            <span className="pill" style={{ color: 'var(--ok)', background: 'color-mix(in oklab,var(--ok) 13%,transparent)', border: '1px solid color-mix(in oklab,var(--ok) 35%,transparent)' }}>
              <Icon name="circle-check" size={12} />
              CI réussie
            </span>
          </div>
          <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', border: '1px solid var(--bd)', borderRadius: 9, background: 'var(--s2)' }}>
              <Icon name="git-merge" style={{ color: 'var(--ac-tx)' }} />
              <div style={{ display: 'flex', flexDirection: 'column', minWidth: 0, flex: 1 }}>
                <span style={{ fontSize: 13, fontWeight: 500 }}>ci: workflow lint + tests</span>
                <span className="mono faint" style={{ fontSize: 12 }}>acme/shop-api #42 · mergée · a3f9c1e</span>
              </div>
              <Icon name="external-link" size={14} style={{ color: 'var(--tx3)' }} />
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 8 }}>
              <Check name="lint">18s</Check>
              <Check name="test">1m02</Check>
              <Check name="build">44s</Check>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <div className="mono faint" style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
                <span>Séquestre · 120 cr</span>
                <span>40 libérés</span>
              </div>
              <div style={{ display: 'flex', height: 8, borderRadius: 99, overflow: 'hidden', background: 'var(--s3)', gap: 2 }}>
                <div style={{ width: '33.3%', background: 'var(--ac)' }} />
                <div style={{ width: '33.3%', background: 'repeating-linear-gradient(135deg,color-mix(in oklab,var(--ac) 45%,transparent) 0 4px,transparent 4px 8px)' }} />
              </div>
            </div>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '14px 16px', borderTop: '1px solid var(--bd)', background: 'color-mix(in oklab,var(--ac) 9%,var(--s1))' }}>
            <Icon name="lock-open" style={{ color: 'var(--ac-tx)' }} />
            <span style={{ fontSize: 13 }}>Jalon 1 validé</span>
            <div style={{ flex: 1 }} />
            <span style={{ font: '600 14px var(--mono)', color: 'var(--ac-tx)' }}>+40 cr → @leamartin</span>
          </div>
        </div>
      </section>

      <section id="etapes" style={{ borderTop: '1px solid var(--bd)' }}>
        <div style={{ ...wrap, display: 'flex', flexDirection: 'column', gap: 36 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <span style={eyebrow}>// comment ça marche</span>
            <h2 style={h2}>Trois étapes, zéro devis.</h2>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 1, background: 'var(--bd)', border: '1px solid var(--bd)', borderRadius: 14, overflow: 'hidden' }}>
            {STEPS.map(([n, icon, title, text]) => (
              <div key={n} style={{ background: 'var(--bg)', padding: 28, display: 'flex', flexDirection: 'column', gap: 14 }}>
                <span className="mono faint" style={{ fontSize: 13, fontWeight: 500 }}>{n}</span>
                <Icon name={icon} size={22} style={{ color: 'var(--ac-tx)' }} />
                <h3 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>{title}</h3>
                <p className="muted" style={{ margin: 0, textWrap: 'pretty' }}>{text}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="preuves" style={{ borderTop: '1px solid var(--bd)' }}>
        <div style={{ ...wrap, display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)', gap: 64, alignItems: 'center' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <span style={eyebrow}>// vérification github</span>
            <h2 style={h2}>La confiance, ça se prouve dans le dépôt.</h2>
            <p className="muted" style={{ margin: 0, fontSize: 16, textWrap: 'pretty' }}>
              CodeTroc lit l'état de tes PR et de tes checks via l'API GitHub. Un jalon ne peut être validé que si sa preuve est vérifiée. La réputation de chaque membre se calcule uniquement sur des livraisons prouvées.
            </p>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 6 }}>
              {[
                ['git-merge', 'var(--tx2)', 'PR mergée sur la branche cible'],
                ['circle-check', 'var(--ok)', 'Checks CI au vert sur le commit livré'],
                ['scale', 'var(--disp)', 'Arbitrage entre pairs si les deux parties ne sont pas d\'accord'],
              ].map(([icon, color, text]) => (
                <div key={icon} style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}>
                  <Icon name={icon} style={{ marginTop: 3, color }} />
                  <span>{text}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="card" style={{ borderRadius: 14, padding: 20, font: '13px/1.9 var(--mono)', color: 'var(--tx2)' }}>
            <div><span className="faint">$</span> codetroc verify EX-0142 --jalon 1</div>
            <div><span className="faint">→</span> PR acme/shop-api#42 <span style={{ color: 'var(--ok)' }}>merged</span></div>
            <div><span className="faint">→</span> commit a3f9c1e · checks 3/3 <span style={{ color: 'var(--ok)' }}>passed</span></div>
            <div><span className="faint">→</span> critères 2/2 couverts par la CI</div>
            <div><span className="faint">→</span> en attente de validation par <span style={{ color: 'var(--tx)' }}>@samib</span></div>
            <div style={{ color: 'var(--ac-tx)' }}>✓ 40 cr libérables</div>
          </div>
        </div>
      </section>

      <section id="coffre" style={{ borderTop: '1px solid var(--bd)' }}>
        <div style={{ ...wrap, display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)', gap: 64, alignItems: 'center' }}>
          <div className="card" style={{ borderRadius: 14, overflow: 'hidden' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '16px 18px', borderBottom: '1px solid var(--bd)' }}>
              <span style={{ width: 34, height: 34, borderRadius: 8, background: 'var(--s3)', display: 'grid', placeItems: 'center' }}>
                <Icon name="server" />
              </span>
              <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minWidth: 0 }}>
                <span style={{ font: '500 13px var(--mono)' }}>ssh deploy@staging-01</span>
                <span className="faint" style={{ fontSize: 12.5 }}>Partagé avec Léa · utilisateur deploy, pas de sudo</span>
              </div>
              <button className="btn btn-danger" tabIndex={-1} aria-hidden="true" style={{ height: 30, padding: '0 12px', borderRadius: 7, fontSize: 12.5 }}>Révoquer</button>
            </div>
            <div style={{ padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 8 }}>
              <div className="mono faint" style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
                <span>Expire dans 2 j 4 h</span>
                <span>dernier usage il y a 12 min</span>
              </div>
              <div style={{ height: 6, borderRadius: 99, background: 'var(--s3)' }}>
                <div style={{ width: '62%', height: '100%', borderRadius: 99, background: 'var(--ac)' }} />
              </div>
            </div>
            <div className="faint" style={{ padding: '4px 18px 16px', font: '12px/1.9 var(--mono)' }}>
              <div>14:32 · Léa · connexion SSH · 82.64.x.x</div>
              <div>11:02 · Toi · accès créé · 72 h</div>
            </div>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <span style={eyebrow}>// coffre d'accès</span>
            <h2 style={h2}>Partage un accès, garde la main.</h2>
            <p className="muted" style={{ margin: 0, fontSize: 16, textWrap: 'pretty' }}>
              Serveur, dépôt ou secret : chaque accès a une durée, une portée et un journal. Il expire tout seul, et tu le coupes en un clic si besoin.
            </p>
          </div>
        </div>
      </section>

      <section style={{ borderTop: '1px solid var(--bd)' }}>
        <div style={{ maxWidth: 1200, margin: '0 auto', padding: '80px 32px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 20, textAlign: 'center' }}>
          <h2 style={{ margin: 0, fontSize: 40, letterSpacing: '-.035em', fontWeight: 600 }}>Ton premier pack t'attend.</h2>
          <p className="muted" style={{ margin: 0, fontSize: 16 }}>{WELCOME_BONUS} cr offerts à l'inscription pour demander ton premier pack.</p>
          <GithubButton big onClick={toOnboarding} />
        </div>
        <footer className="mono faint" style={{ maxWidth: 1200, margin: '0 auto', padding: '20px 32px', borderTop: '1px solid var(--bd)', display: 'flex', gap: 20, fontSize: 12 }}>
          <span>© 2026 CodeTroc</span>
          <div style={{ flex: 1 }} />
          <span>Charte d'arbitrage</span>
          <span>Sécurité</span>
          <span>Statut</span>
        </footer>
      </section>
    </div>
  );
}
