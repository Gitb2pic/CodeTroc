import type { MouseEvent, ReactNode } from 'react';
import { CoinMark, Logo } from './Brand';
import { Icon } from './ui';

/** Défilement explicite : une ancre #id casserait le routage par hash. */
const scrollTo = (id: string) => (e: MouseEvent) => {
  e.preventDefault();
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' });
};

function Check({ name, children }: { name: string; children: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '8px 10px', border: '1px solid var(--bd)', borderRadius: 8, font: '12px var(--mono)', color: 'var(--tx2)' }}>
      <Icon name="check" size={14} style={{ color: 'var(--ok)' }} />
      {name}
      <span style={{ marginLeft: 'auto', color: 'var(--tx3)' }}>{children}</span>
    </div>
  );
}

// Le parcours d'un échange, dans l'ordre réel des opérations du backend.
const STEPS: [string, string, string][] = [
  ['package', 'Choisis un pack', "Un livrable standard, des critères d'acceptation, un prix et des jalons fixés d'avance. Ce qui est écrit est ce qui est livré."],
  ['lock', 'Les crédits passent en séquestre', "À l'acceptation, le prix est bloqué sur un compte propre à l'échange. Rien n'est débité avant, et personne ne peut y toucher seul."],
  ['git-pull-request', 'Livraison par pull request', 'Le prestataire donne l’URL de sa PR. CodeTroc la vérifie auprès de GitHub : ton dépôt, son auteur, mergée, CI au vert.'],
  ['lock-open', 'Validation jalon par jalon', 'Tu valides, les crédits du jalon sont libérés. Sans réponse de ta part sous 72 h, le jalon est validé automatiquement.'],
];

const figures = (bonus: number, note?: string): [string, string][] => [
  [`${bonus} cr`, note ?? "offerts à l'inscription pour demander ton premier pack"],
  ['72 h', 'pour valider un jalon livré, sinon validation tacite'],
  ['0 €', "aucun paiement : les crédits ne s'achètent pas, ils se gagnent"],
  ['1 clé', 'de chiffrement par secret partagé, effacée à la révocation'],
];


const TRUST: [string, string, string][] = [
  ['github', 'Connexion GitHub en lecture seule', 'Ton profil public et ton e-mail vérifié, rien de plus. Aucun accès en écriture à tes dépôts.'],
  ['book-open', 'Un grand livre en partie double', 'Chaque crédit a une origine et une destination. Les écritures sont immuables et aucun solde ne peut passer sous zéro.'],
  ['shield-check', 'Sessions courtes', "Jeton d'accès de 15 minutes gardé en mémoire, renouvelé par un cookie HttpOnly. Un jeton rejoué coupe toute la session."],
  ['user-x', 'Tes données t’appartiennent', 'Supprimer ton compte l’anonymise : pseudo, e-mail et bio effacés, historique des crédits conservé pour les autres.'],
];

const FAQ: [string, string][] = [
  ['Combien ça coûte ?', "Rien. CodeTroc ne manipule pas d'argent : tu reçois 50 crédits à l'inscription, puis tu en gagnes en livrant des packs aux autres membres. Les crédits ne s'achètent pas et ne se revendent pas."],
  ['Et si la PR ne passe pas la vérification ?', "La preuve est rejetée avec la raison exacte (PR d'un autre dépôt, pas encore mergée, check en échec…). Le prestataire corrige et relivre ; tant que le jalon n'est pas validé, les crédits restent en séquestre."],
  ["Et si on n'est pas d'accord sur une livraison ?", "Tu contestes le jalon avec un motif. Ses crédits sont gelés et un arbitre qui n'est partie ni de près ni de loin à l'échange tranche : libérer, rembourser, ou partager au pourcentage, avec une justification visible des deux côtés."],
  ['Et si le prestataire disparaît ?', 'Chaque jalon a une échéance. Passé ce délai et 48 h de grâce, tu peux annuler seul : les jalons déjà validés restent acquis, tout le reste du séquestre te revient.'],
  ["Que deviennent les accès que je partage ?", "Chaque secret (jeton, clé d'API, mot de passe) est chiffré avec sa propre clé, révélé seulement au prestataire et journalisé à chaque consultation. Il expire au bout de 30 jours au plus et il est révoqué automatiquement à la fin de l'échange."],
  ['Qui peut voir mes échanges ?', "Les deux parties, et l'arbitre s'il y a un litige. Pour tous les autres, un échange n'existe tout simplement pas."],
];

export interface LandingProps {
  /** Bouton d'action principal (connexion GitHub, choix d'un persona…). */
  primary: (big?: boolean) => ReactNode;
  /** Action secondaire du héros, par exemple « Parcourir les packs ». */
  secondary?: { label: string; onClick: () => void };
  /** Contenu inséré sous le héros (erreur de connexion, choix des personas). */
  children?: ReactNode;
  /** Bonus de bienvenue affiché (50 cr en production) et sa légende éventuelle. */
  bonus?: { amount: number; note?: string };
}

export function Landing({ primary, secondary, children, bonus = { amount: 50 } }: LandingProps) {
  return (
    <div>
      <header style={{ position: 'sticky', top: 'env(safe-area-inset-top, 0px)', zIndex: 10, background: 'color-mix(in oklab,var(--bg) 82%,transparent)', backdropFilter: 'blur(12px)', borderBottom: '1px solid var(--bd)' }}>
        <div className="lp-wrap lp-header">
          <Logo wordmark size={22} />
          <nav className="lp-nav" aria-label="Sections">
            <a href="#etapes" onClick={scrollTo('etapes')}>Comment ça marche</a>
            <a href="#preuves" onClick={scrollTo('preuves')}>Preuves GitHub</a>
            <a href="#arbitrage" onClick={scrollTo('arbitrage')}>Litiges</a>
            <a href="#coffre" onClick={scrollTo('coffre')}>Coffre d'accès</a>
            <a href="#faq" onClick={scrollTo('faq')}>Questions</a>
          </nav>
          <div style={{ flex: 1 }} />
          <div className="lp-header-cta">{primary(false)}</div>
        </div>
      </header>

      <section className="lp-wrap lp-hero">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24, minWidth: 0 }}>
          <div style={{ display: 'inline-flex', alignSelf: 'flex-start', alignItems: 'center', gap: 8, minHeight: 26, padding: '4px 10px', border: '1px solid var(--bd2)', borderRadius: 999, font: '500 12px/1.3 var(--mono)', color: 'var(--tx2)' }}>
            <span style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--brand-b)', flex: 'none' }} />
            Entre devs. Sans argent. Sur preuve.
          </div>
          <h1 className="lp-h1">Échange ton code contre du code.</h1>
          <p style={{ margin: 0, fontSize: 18, lineHeight: 1.55, color: 'var(--tx2)', maxWidth: 540, textWrap: 'pretty' }}>
            Tu livres un pack, tu gagnes des crédits, tu les dépenses chez n'importe quel membre. Chaque crédit libéré est adossé à une preuve GitHub : pull request mergée, CI au vert.
          </p>
          <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
            {primary(true)}
            {secondary && (
              <button className="btn btn-secondary" onClick={secondary.onClick} style={{ height: 44, padding: '0 18px', borderRadius: 9, fontSize: 15, gap: 8 }}>
                {secondary.label}
                <Icon name="arrow-right" />
              </button>
            )}
          </div>
          <div style={{ display: 'flex', gap: '6px 18px', font: '12px/1.4 var(--mono)', color: 'var(--tx3)', flexWrap: 'wrap' }}>
            <span>Aucun paiement</span><span aria-hidden="true">·</span><span>OAuth en lecture seule</span><span aria-hidden="true">·</span><span>Accès révocables</span>
          </div>
        </div>

        <div className="card" style={{ borderRadius: 14, boxShadow: '0 30px 80px -30px rgba(0,0,0,.45)', overflow: 'hidden', minWidth: 0 }} aria-label="Exemple d'échange">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '14px 16px', borderBottom: '1px solid var(--bd)', flexWrap: 'wrap' }}>
            <span className="mono faint" style={{ fontSize: 12, fontWeight: 500 }}>EX-0142</span>
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
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '14px 16px', borderTop: '1px solid var(--bd)', background: 'color-mix(in oklab,var(--ac) 9%,var(--s1))', flexWrap: 'wrap' }}>
            <Icon name="lock-open" style={{ color: 'var(--ac-tx)' }} />
            <span style={{ fontSize: 13 }}>Jalon 1 validé</span>
            <div style={{ flex: 1 }} />
            <span style={{ font: '600 14px var(--mono)', color: 'var(--ac-tx)' }}>+40 cr → @lea</span>
          </div>
        </div>
      </section>

      {children && <section id="essayer" className="lp-wrap" style={{ paddingBlock: '0 56px' }}>{children}</section>}

      <section aria-label="En chiffres" style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap" style={{ paddingBlock: 40 }}>
          <div className="lp-cells lp-cols-4">
            {figures(bonus.amount, bonus.note).map(([n, text]) => (
              <div key={n}>
                <span style={{ font: '600 30px/1 var(--mono)', letterSpacing: '-.03em', color: 'var(--ac-tx)' }}>{n}</span>
                <span className="muted" style={{ fontSize: 13.5, textWrap: 'pretty' }}>{text}</span>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="etapes" style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap" style={{ display: 'flex', flexDirection: 'column', gap: 36 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <span className="lp-eyebrow">// comment ça marche</span>
            <h2 className="lp-h2">Quatre étapes, zéro devis.</h2>
          </div>
          <ol className="lp-cells lp-cols-4" style={{ margin: 0, padding: 0, listStyle: 'none' }}>
            {STEPS.map(([icon, title, text], i) => (
              <li key={title}>
                <span className="mono faint" style={{ fontSize: 13, fontWeight: 500 }}>{String(i + 1).padStart(2, '0')}</span>
                <Icon name={icon} size={22} style={{ color: 'var(--ac-tx)' }} />
                <h3 style={{ margin: 0, fontSize: 17, fontWeight: 600 }}>{title}</h3>
                <p className="muted" style={{ margin: 0, textWrap: 'pretty' }}>{text}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      <section id="preuves" style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap lp-split">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16, minWidth: 0 }}>
            <span className="lp-eyebrow">// vérification github</span>
            <h2 className="lp-h2">La confiance, ça se prouve dans le dépôt.</h2>
            <p className="muted" style={{ margin: 0, fontSize: 16, textWrap: 'pretty' }}>
              CodeTroc lit l'état des PR et des checks via une GitHub App en lecture seule. Un jalon ne passe « livré » que si sa preuve est vérifiée, et la réputation de chaque membre ne compte que des livraisons prouvées.
            </p>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 6 }}>
              {[
                ['folder-git-2', 'var(--tx2)', "PR sur le dépôt de l'échange, ouverte par le prestataire après l'acceptation"],
                ['git-merge', 'var(--tx2)', 'PR mergée sur la branche cible'],
                ['circle-check', 'var(--ok)', 'Tous les checks CI au vert sur le commit livré'],
                ['webhook', 'var(--ac-tx)', 'Webhooks signés : la livraison est détectée sans rien rafraîchir'],
              ].map(([icon, color, text]) => (
                <div key={text} style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}>
                  <Icon name={icon} style={{ marginTop: 3, color, flex: 'none' }} />
                  <span>{text}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="card" style={{ borderRadius: 14, padding: 20, font: '13px/1.9 var(--mono)', color: 'var(--tx2)', minWidth: 0, overflowX: 'auto' }}>
            <div><span className="faint">$</span> codetroc verify EX-0142 --jalon 1</div>
            <div><span className="faint">→</span> PR acme/shop-api#42 <span style={{ color: 'var(--ok)' }}>merged</span></div>
            <div><span className="faint">→</span> auteur @lea = prestataire <span style={{ color: 'var(--ok)' }}>ok</span></div>
            <div><span className="faint">→</span> commit a3f9c1e · checks 3/3 <span style={{ color: 'var(--ok)' }}>passed</span></div>
            <div><span className="faint">→</span> en attente de validation par <span style={{ color: 'var(--tx)' }}>@sami</span> (72 h)</div>
            <div style={{ color: 'var(--ac-tx)' }}>✓ 40 cr libérables</div>
          </div>
        </div>
      </section>

      <section id="arbitrage" style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap" style={{ display: 'flex', flexDirection: 'column', gap: 32 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10, maxWidth: 720 }}>
            <span className="lp-eyebrow">// litiges et imprévus</span>
            <h2 className="lp-h2">Quand ça coince, les règles sont écrites d'avance.</h2>
          </div>
          <div className="lp-cells lp-cols-3">
            <div>
              <Icon name="flag" size={20} style={{ color: 'var(--disp)' }} />
              <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>Contester un jalon</h3>
              <p className="muted" style={{ margin: 0 }}>Tu indiques quel critère n'est pas rempli. Les crédits du jalon sont gelés et l'autre partie peut répondre une fois.</p>
            </div>
            <div>
              <Icon name="scale" size={20} style={{ color: 'var(--disp)' }} />
              <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>Un arbitre indépendant</h3>
              <p className="muted" style={{ margin: 0 }}>Il n'est jamais partie à l'échange. Il libère, rembourse ou partage au pourcentage, et sa justification est visible des deux côtés.</p>
            </div>
            <div>
              <Icon name="undo-2" size={20} style={{ color: 'var(--disp)' }} />
              <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>Annuler sans perdre</h3>
              <p className="muted" style={{ margin: 0 }}>D'un commun accord, ou seul si le prestataire dépasse l'échéance de 48 h : ce qui est validé reste acquis, le reste te revient.</p>
            </div>
          </div>
        </div>
      </section>

      <section id="coffre" style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap lp-split">
          <div className="card" style={{ borderRadius: 14, overflow: 'hidden', minWidth: 0 }} aria-label="Exemple d'accès partagé">
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '16px 18px', borderBottom: '1px solid var(--bd)' }}>
              <span style={{ width: 34, height: 34, borderRadius: 8, background: 'var(--s3)', display: 'grid', placeItems: 'center', flex: 'none' }}>
                <Icon name="key-round" />
              </span>
              <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minWidth: 0 }}>
                <span style={{ font: '500 13px var(--mono)' }}>GHCR_PUSH_TOKEN</span>
                <span className="faint" style={{ fontSize: 12.5 }}>Partagé avec Léa · packages:write</span>
              </div>
              <span className="pill" style={{ color: 'var(--ok)', background: 'color-mix(in oklab,var(--ok) 13%,transparent)', border: '1px solid color-mix(in oklab,var(--ok) 35%,transparent)' }}>Actif</span>
            </div>
            <div style={{ padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 8 }}>
              <div className="mono faint" style={{ display: 'flex', justifyContent: 'space-between', gap: 12, fontSize: 12, flexWrap: 'wrap' }}>
                <span>Expire dans 2 j 4 h</span>
                <span>révoqué à la fin de l'échange</span>
              </div>
              <div style={{ height: 6, borderRadius: 99, background: 'var(--s3)' }}>
                <div style={{ width: '62%', height: '100%', borderRadius: 99, background: 'var(--ac)' }} />
              </div>
            </div>
            <div className="faint" style={{ padding: '4px 18px 16px', font: '12px/1.9 var(--mono)' }}>
              <div>14:32 · Léa · révélation · 82.64.x.x</div>
              <div>11:02 · Toi · création · 72 h</div>
            </div>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16, minWidth: 0 }}>
            <span className="lp-eyebrow">// coffre d'accès</span>
            <h2 className="lp-h2">Partage un accès, garde la main.</h2>
            <p className="muted" style={{ margin: 0, fontSize: 16, textWrap: 'pretty' }}>
              Jeton, clé d'API ou mot de passe : chaque secret est chiffré avec sa propre clé (AES-256-GCM) et lié à son échange. Seul le prestataire peut le révéler, chaque consultation est journalisée, et il disparaît à l'expiration ou à la clôture.
            </p>
          </div>
        </div>
      </section>

      <section aria-labelledby="pour-qui" style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap" style={{ display: 'flex', flexDirection: 'column', gap: 32 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <span className="lp-eyebrow">// pour qui</span>
            <h2 className="lp-h2" id="pour-qui">Des deux côtés de la pièce.</h2>
          </div>
          <div className="lp-cells lp-cols-2">
            <div>
              <Icon name="arrow-up-right" size={20} style={{ color: 'var(--brand-a)' }} />
              <h3 style={{ margin: 0, fontSize: 17, fontWeight: 600 }}>Tu as besoin d'un coup de main</h3>
              <p className="muted" style={{ margin: 0 }}>Une CI qui ne tourne pas, une app à dockeriser, un formulaire à rendre accessible : tu demandes un pack, tu paies en crédits, tu ne valides que ce qui est prouvé.</p>
            </div>
            <div>
              <Icon name="arrow-down-left" size={20} style={{ color: 'var(--brand-b)' }} />
              <h3 style={{ margin: 0, fontSize: 17, fontWeight: 600 }}>Tu veux valoriser ton savoir-faire</h3>
              <p className="muted" style={{ margin: 0 }}>Tu publies tes offres, tu livres sur de vrais dépôts et chaque livraison vérifiée nourrit ta réputation. Les crédits gagnés t'ouvrent les compétences des autres.</p>
            </div>
          </div>
        </div>
      </section>

      <section aria-labelledby="marque" style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap lp-split">
          <div style={{ display: 'grid', placeItems: 'center', padding: 24 }}>
            <CoinMark size={200} title="La pièce fendue, symbole de codetroc" />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16, minWidth: 0 }}>
            <span className="lp-eyebrow">// la pièce fendue</span>
            <h2 className="lp-h2" id="marque">Une pièce, coupée par le « / » du code.</h2>
            <p className="muted" style={{ margin: 0, fontSize: 16, textWrap: 'pretty' }}>
              Les deux moitiés glissent l'une contre l'autre : deux devs qui échangent. Ici, un crédit gagné est toujours un crédit dépensé par quelqu'un d'autre ; la pièce ne vaut que parce qu'elle circule.
            </p>
          </div>
        </div>
      </section>

      <section aria-labelledby="confiance" style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap" style={{ display: 'flex', flexDirection: 'column', gap: 32 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <span className="lp-eyebrow">// sécurité</span>
            <h2 className="lp-h2" id="confiance">Conçu pour qu'on n'ait pas à te faire confiance.</h2>
          </div>
          <div className="lp-cells lp-cols-4">
            {TRUST.map(([icon, title, text]) => (
              <div key={title}>
                <Icon name={icon} size={20} style={{ color: 'var(--ac-tx)' }} />
                <h3 style={{ margin: 0, fontSize: 15.5, fontWeight: 600 }}>{title}</h3>
                <p className="muted" style={{ margin: 0, fontSize: 13.5, textWrap: 'pretty' }}>{text}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="faq" style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap lp-faq" style={{ display: 'flex', flexDirection: 'column', gap: 18, maxWidth: 860 }}>
          <span className="lp-eyebrow">// questions fréquentes</span>
          <h2 className="lp-h2" style={{ marginBottom: 8 }}>Tu te demandes sûrement…</h2>
          <div>
            {FAQ.map(([q, a]) => (
              <details key={q}>
                <summary>{q}</summary>
                <p>{a}</p>
              </details>
            ))}
          </div>
        </div>
      </section>

      <section style={{ borderTop: '1px solid var(--bd)' }}>
        <div className="lp-wrap" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 20, textAlign: 'center' }}>
          <CoinMark size={44} />
          <h2 className="lp-h2">Ton premier pack t'attend.</h2>
          <p className="muted" style={{ margin: 0, fontSize: 16 }}>{bonus.amount} crédits offerts à l'inscription. Connexion GitHub en lecture seule.</p>
          {primary(true)}
        </div>
        <footer style={{ borderTop: '1px solid var(--bd)' }}>
          <div className="lp-wrap" style={{ paddingBlock: 22, display: 'flex', gap: '10px 22px', alignItems: 'center', flexWrap: 'wrap', font: '12px var(--mono)', color: 'var(--tx3)' }}>
            <Logo wordmark size={18} />
            <span>© 2026</span>
            <div style={{ flex: 1 }} />
            <span>
              Conçu et développé par <strong style={{ color: 'var(--tx2)', fontWeight: 600 }}>LabTools</strong>
            </span>
          </div>
        </footer>
      </section>
    </div>
  );
}
