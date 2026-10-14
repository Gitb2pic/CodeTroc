import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, HashRouter } from 'react-router-dom';
import { App } from './App';
import './styles/global.css';
import './styles/icons.css';

// Le build « artifact » est servi comme une page statique : les routes passent par le hash.
const Router = import.meta.env.MODE === 'artifact' ? HashRouter : BrowserRouter;

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Router>
      <App />
    </Router>
  </StrictMode>,
);
