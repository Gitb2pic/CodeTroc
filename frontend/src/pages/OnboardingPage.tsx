import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Icon, Logo } from '../components/ui';
import { ONBOARDING_PACKS, ONBOARDING_SKILLS, WELCOME_BONUS } from '../data/fixtures';
import { ROUTES } from '../routes';
import { useApp } from '../store/AppStore';

const h1 = { margin: 0, fontSize: 30, letterSpacing: '-.03em', fontWeight: 600 } as const;
const lead = { margin: 0, color: 'var(--tx2)', fontSize: 15 } as const;
const col = { display: 'flex', flexDirection: 'column', gap: 20 } as const;

export function OnboardingPage() {
  const navigate = useNavigate();
  const { skills, toggleSkill, showToast } = useApp();
  const [step, setStep] = useState(0);
  const [pack, setPack] = useState(0);

  const next = () => (step >= 3 ? navigate(ROUTES.dash) : setStep(step + 1));
  const back = () => (step === 0 ? navigate(ROUTES.landing) : setStep(step - 1));

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      <div style={{ height: 60, display: 'flex', alignItems: 'center', gap: 9, padding: '0 32px', borderBottom: '1px solid var(--bd)', fontWeight: 600 }}>
        <Logo wordmark size={22} />
        <div style={{ flex: 1 }} />
        <button onClick={() => navigate(ROUTES.landing)} style={{ border: 0, background: 'none', color: 'var(--tx3)', fontSize: 13, cursor: 'pointer' }}>
          Quitter
        </button>
      </div>
      <div style={{ flex: 1, display: 'flex', justifyContent: 'center', padding: '56px 24px' }}>
        <div style={{ width: '100%', maxWidth: 600, display: 'flex', flexDirection: 'column', gap: 28 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <div style={{ display: 'flex', gap: 6 }} aria-hidden="true">
              {[0, 1, 2, 3].map((i) => (
                <div key={i} style={{ flex: 1, height: 4, borderRadius: 99, background: i <= step ? 'var(--ac)' : 'var(--s3)' }} />
              ))}
            </div>
            <span className="mono faint" style={{ fontSize: 12 }}>Étape {step + 1} sur 4</span>
          </div>

          {step === 0 && (
            <div style={col}>
              <h1 style={h1}>Connecte ton compte GitHub</h1>
              <p style={lead}>C'est ton identité sur CodeTroc et la source de tes preuves de livraison.</p>
              <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
                <span className="kicker">Ce que CodeTroc lit</span>
                <div style={{ display: 'flex', gap: 10 }}><Icon name="check" style={{ color: 'var(--ok)', marginTop: 3 }} /><span>Ton profil public et tes dépôts publics</span></div>
                <div style={{ display: 'flex', gap: 10 }}><Icon name="check" style={{ color: 'var(--ok)', marginTop: 3 }} /><span>L'état des PR et des checks CI des dépôts liés à un échange</span></div>
                <div style={{ display: 'flex', gap: 10 }}><Icon name="ban" style={{ color: 'var(--tx3)', marginTop: 3 }} /><span className="muted">Jamais d'écriture sur ton code, jamais de clés privées</span></div>
              </div>
              <button
                className="btn btn-primary"
                onClick={() => {
                  setStep(1);
                  showToast('Compte GitHub connecté', 'ok');
                }}
                style={{ height: 46, borderRadius: 9, fontSize: 15, gap: 8 }}
              >
                <Icon name="github" size={18} />
                Autoriser avec GitHub
              </button>
            </div>
          )}

          {step === 1 && (
            <div style={col}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', border: '1px solid color-mix(in oklab,var(--ok) 35%,transparent)', background: 'color-mix(in oklab,var(--ok) 10%,transparent)', borderRadius: 9, fontSize: 13 }}>
                <Icon name="circle-check" style={{ color: 'var(--ok)' }} />
                Connecté en tant que <span className="mono">@samib</span> · 23 dépôts publics analysés
              </div>
              <h1 style={h1}>Qu'est-ce que tu sais livrer ?</h1>
              <p style={lead}>On a pré-sélectionné ce qu'on voit dans tes dépôts. Ajuste librement.</p>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                {ONBOARDING_SKILLS.map(([name, detected]) => {
                  const on = skills.includes(name);
                  return (
                    <button
                      key={name}
                      onClick={() => toggleSkill(name)}
                      aria-pressed={on}
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: 6,
                        height: 32,
                        padding: '0 12px',
                        borderRadius: 8,
                        font: '500 13px/1 var(--mono)',
                        cursor: 'pointer',
                        background: on ? 'color-mix(in oklab, var(--ac) 14%, transparent)' : 'var(--s1)',
                        color: on ? 'var(--ac-tx)' : 'var(--tx2)',
                        border: `1px solid ${on ? 'color-mix(in oklab, var(--ac) 50%, transparent)' : 'var(--bd)'}`,
                      }}
                    >
                      <Icon name={on ? 'check' : 'plus'} size={14} />
                      {name}
                      {detected && <span style={{ fontSize: 10.5, color: 'var(--tx3)' }}>détecté</span>}
                    </button>
                  );
                })}
              </div>
              <span className="mono faint" style={{ fontSize: 12 }}>{skills.length} compétences sélectionnées</span>
            </div>
          )}

          {step === 2 && (
            <div style={col}>
              <h1 style={h1}>Propose ton premier pack</h1>
              <p style={lead}>Un modèle pour démarrer, basé sur tes compétences. Tu pourras tout modifier.</p>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }} role="radiogroup" aria-label="Modèle de pack">
                {ONBOARDING_PACKS.map((o, i) => {
                  const on = pack === i;
                  const bd = on ? 'var(--ac)' : 'var(--bd)';
                  return (
                    <button
                      key={o.title}
                      role="radio"
                      aria-checked={on}
                      onClick={() => setPack(i)}
                      style={{ display: 'flex', alignItems: 'center', gap: 14, padding: '14px 16px', borderRadius: 10, textAlign: 'left', cursor: 'pointer', background: on ? 'color-mix(in oklab, var(--ac) 8%, var(--s1))' : 'var(--s1)', border: `1px solid ${bd}`, color: 'var(--tx)' }}
                    >
                      <span style={{ width: 16, height: 16, borderRadius: '50%', border: `1.5px solid ${bd}`, display: 'grid', placeItems: 'center', flex: 'none' }}>
                        <span style={{ width: 8, height: 8, borderRadius: '50%', background: on ? 'var(--ac)' : 'transparent' }} />
                      </span>
                      <span style={{ display: 'flex', flexDirection: 'column', flex: 1, gap: 2 }}>
                        <span style={{ fontWeight: 500 }}>{o.title}</span>
                        <span className="mono faint" style={{ fontSize: 12 }}>{o.meta}</span>
                      </span>
                      <span style={{ font: '600 14px var(--mono)', color: 'var(--ac-tx)' }}>{o.price}</span>
                    </button>
                  );
                })}
              </div>
              <button onClick={next} style={{ alignSelf: 'flex-start', border: 0, background: 'none', color: 'var(--tx3)', fontSize: 13, cursor: 'pointer', padding: 0, textDecoration: 'underline' }}>
                Passer, je le ferai plus tard
              </button>
            </div>
          )}

          {step === 3 && (
            <div style={col}>
              <h1 style={h1}>Ton solde de départ</h1>
              <div className="card" style={{ padding: 24, display: 'flex', alignItems: 'baseline', gap: 12 }}>
                <span style={{ font: '600 52px/1 var(--mono)', color: 'var(--ac-tx)', letterSpacing: '-.03em' }}>{WELCOME_BONUS}</span>
                <span style={{ font: '500 18px var(--mono)', color: 'var(--ac-tx)' }}>cr</span>
                <span className="muted" style={{ marginLeft: 'auto', fontSize: 13 }}>offerts à l'inscription</span>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
                {[
                  ['ban', "Les crédits ne s'achètent pas", "Ils se gagnent en livrant. Pas de carte, pas d'euros, jamais."],
                  ['lock', 'Ils passent en séquestre quand tu demandes un pack', "Le prestataire est sûr d'être payé, toi d'avoir une livraison prouvée."],
                  ['lock-open', 'Ils sont libérés jalon par jalon', 'Uniquement quand la PR est mergée, la CI au vert et que tu valides.'],
                ].map(([icon, title, sub]) => (
                  <div key={icon} style={{ display: 'flex', gap: 12 }}>
                    <Icon name={icon} style={{ marginTop: 3, color: 'var(--tx2)' }} />
                    <div>
                      <div style={{ fontWeight: 500 }}>{title}</div>
                      <div className="muted" style={{ fontSize: 13.5 }}>{sub}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div style={{ display: 'flex', gap: 10, justifyContent: 'space-between', paddingTop: 8, borderTop: '1px solid var(--bd)' }}>
            <button className="btn btn-secondary" onClick={back} style={{ height: 38, gap: 6 }}>
              <Icon name="chevron-left" />
              Retour
            </button>
            <button className="btn btn-primary" onClick={next} style={{ height: 38, padding: '0 16px', gap: 6 }}>
              {step === 3 ? 'Aller au tableau de bord' : 'Continuer'}
              <Icon name="arrow-right" />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
