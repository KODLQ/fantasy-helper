import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import { SeasonProvider } from './season-context';
import { AuthProvider } from './auth-context';
import './styles.css';
import './weights.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AuthProvider><SeasonProvider><App /></SeasonProvider></AuthProvider>
  </StrictMode>,
);
