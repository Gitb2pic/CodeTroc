import { useNavigate } from 'react-router-dom';
import { Landing } from '../components/Landing';
import { Icon } from '../components/ui';
import { ROUTES } from '../routes';

function GithubButton({ big, onClick }: { big?: boolean; onClick: () => void }) {
  return (
    <button className="btn btn-primary" onClick={onClick} style={big ? { height: 44, padding: '0 18px', borderRadius: 9, fontSize: 15, gap: 8 } : { gap: 8 }}>
      <Icon name="github" size={big ? 18 : undefined} />
      Se connecter avec GitHub
    </button>
  );
}

/** Landing du prototype : la connexion mène à l'onboarding de démonstration. */
export function LandingPage() {
  const navigate = useNavigate();
  return (
    <Landing
      primary={(big) => <GithubButton big={big} onClick={() => navigate(ROUTES.onboarding)} />}
      secondary={{ label: 'Parcourir les packs', onClick: () => navigate(ROUTES.cat) }}
    />
  );
}
