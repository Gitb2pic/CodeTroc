import { act, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { App } from './App';
import { CI_DURATION_MS } from './store/AppStore';

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  );
}

const balanceButton = () => screen.getByRole('button', { name: /^Solde :/ });

beforeEach(() => {
  localStorage.clear();
});

describe('parcours de démonstration', () => {
  it('va de la demande du pack à la libération du jalon 1', () => {
    vi.useFakeTimers();
    try {
      renderAt('/catalogue/p1');
      expect(balanceButton()).toHaveAccessibleName('Solde : 240 crédits disponibles, 190 en séquestre');

      fireEvent.click(screen.getByRole('button', { name: /Demander ce pack/ }));
      expect(screen.getByRole('heading', { level: 1, name: 'Pipeline CI/CD GitHub Actions pour app Node' })).toBeInTheDocument();
      expect(screen.getByText('Léa doit accepter ta demande')).toBeInTheDocument();
      expect(balanceButton()).toHaveAccessibleName('Solde : 240 crédits disponibles, 190 en séquestre');

      fireEvent.click(screen.getByRole('button', { name: 'Léa accepte →' }));
      fireEvent.click(screen.getByRole('button', { name: 'Léa ouvre la PR #42 →' }));
      expect(screen.getByText('La CI vérifie la PR #42')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Valider le jalon 1/ })).not.toBeInTheDocument();

      act(() => vi.advanceTimersByTime(CI_DURATION_MS));
      expect(screen.getByText('À toi : vérifie et valide le jalon 1')).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: 'Valider le jalon 1 · libérer 40 cr' }));
      expect(screen.getByText('40 cr libérés à Léa')).toBeInTheDocument();
      expect(balanceButton()).toHaveAccessibleName('Solde : 240 crédits disponibles, 150 en séquestre');
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('catalogue', () => {
  it('filtre les packs et propose de réinitialiser quand rien ne correspond', () => {
    renderAt('/catalogue');
    expect(screen.getByText('9 packs')).toBeInTheDocument();
    fireEvent.change(screen.getByRole('textbox', { name: 'Rechercher un pack' }), { target: { value: 'kubernetes rust' } });
    expect(screen.getByText('Aucun pack ne correspond')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Réinitialiser les filtres' }));
    expect(screen.getByText('9 packs')).toBeInTheDocument();
  });

  it("affiche l'état d'erreur demandé par ?etat=erreur", () => {
    renderAt('/catalogue?etat=erreur');
    expect(screen.getByRole('alert')).toHaveTextContent("Le catalogue n'a pas pu se charger.");
  });
});

describe("coffre d'accès", () => {
  it('demande confirmation avant de révoquer et journalise la révocation', () => {
    renderAt('/coffre');
    fireEvent.click(screen.getByRole('button', { name: 'Révoquer GHCR_PUSH_TOKEN' }));
    expect(screen.getByRole('alertdialog', { name: 'Confirmer la révocation' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Révoquer maintenant' }));
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    expect(screen.getByText("révoqué à l'instant")).toBeInTheDocument();
    expect(screen.getByText('Accès révoqué')).toBeInTheDocument();
  });
});

describe('création de pack', () => {
  it('empêche de publier si les jalons ne totalisent pas le prix', () => {
    renderAt('/packs/nouveau');
    fireEvent.click(screen.getByRole('button', { name: /Prix$/ }));
    fireEvent.change(screen.getByRole('spinbutton', { name: /^Prix/ }), { target: { value: '150' } });
    expect(screen.getByRole('button', { name: /Publier le pack/ })).toBeDisabled();
    fireEvent.change(screen.getByRole('spinbutton', { name: /^Prix/ }), { target: { value: '120' } });
    expect(screen.getByRole('button', { name: /Publier le pack/ })).toBeEnabled();
  });
});
