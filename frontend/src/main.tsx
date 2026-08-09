import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import { SeasonProvider } from './season-context';
import './styles.css';
import './weights.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <SeasonProvider><App /></SeasonProvider>
  </StrictMode>,
);
