import { createContext, FormEvent, ReactNode, useContext, useEffect, useState } from 'react';
import { api, ApiError, AuthConfig, AuthUser, clearAuthenticationState, setAuthenticationFailureListener } from './api';

type AuthContextValue = {
  user: AuthUser | null;
  config: AuthConfig | null;
  loading: boolean;
  message: string;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, displayName: string) => Promise<void>;
  logout: () => Promise<void>;
  changePassword: (currentPassword: string, newPassword: string) => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [config, setConfig] = useState<AuthConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState('');

  useEffect(() => {
    let active = true;
    setAuthenticationFailureListener(() => {
      if (!active) return;
      clearAuthenticationState();
      setUser(null);
      setMessage('Your session expired. Sign in again to continue.');
    });
    Promise.all([
      api.authConfig().then((value) => { if (active) setConfig(value); }),
      api.me().then((value) => { if (active) setUser(value.user); }).catch((error) => {
        if (active && (!(error instanceof ApiError) || error.status !== 401)) setMessage('Authentication is temporarily unavailable.');
      }),
    ]).finally(() => { if (active) setLoading(false); });
    return () => { active = false; setAuthenticationFailureListener(undefined); };
  }, []);

  const login = async (email: string, password: string) => { const result = await api.login(email, password); setUser(result.user); setMessage(''); };
  const register = async (email: string, password: string, displayName: string) => { const result = await api.register(email, password, displayName); setUser(result.user); setMessage(''); };
  const logout = async () => { await api.logout(); setUser(null); setMessage('You have signed out.'); };
  const changePassword = async (currentPassword: string, newPassword: string) => { const result = await api.changePassword(currentPassword, newPassword); setUser(result.user); setMessage('Password changed. Other sessions were signed out.'); };

  return <AuthContext.Provider value={{ user, config, loading, message, login, register, logout, changePassword }}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used inside AuthProvider');
  return value;
}

export function AuthPanel({ compact = false }: { compact?: boolean }) {
  const auth = useAuth();
  const [registering, setRegistering] = useState(false);
  const [changingPassword, setChangingPassword] = useState(false);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => { if (auth.user) { setRegistering(false); setChangingPassword(false); setError(''); } }, [auth.user]);

  if (auth.loading) return <div className="auth-panel" role="status">Checking your session…</div>;
  if (auth.user) return <div className={`auth-panel ${compact ? 'compact' : ''}`} data-testid="authenticated-user"><strong>{auth.user.displayName || auth.user.email}</strong><span>{auth.user.email}</span>{auth.message && <small role="status">{auth.message}</small>}<button className="text-button" onClick={() => setChangingPassword(!changingPassword)}>Change password</button>{changingPassword && <PasswordForm onSubmit={async (currentPassword, newPassword) => { setSubmitting(true); setError(''); try { await auth.changePassword(currentPassword, newPassword); setChangingPassword(false); } catch (reason) { setError(messageFor(reason)); } finally { setSubmitting(false); } }} disabled={submitting} error={error} />}<button className="secondary-button" onClick={() => { setRegistering(false); void auth.logout(); }}>Sign out</button></div>;

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError('');
    const data = new FormData(event.currentTarget);
    try {
      if (registering) { await auth.register(String(data.get('email')), String(data.get('password')), String(data.get('displayName'))); setRegistering(false); }
      else await auth.login(String(data.get('email')), String(data.get('password')));
      event.currentTarget.reset();
    } catch (reason) {
      setError(messageFor(reason));
    } finally {
      setSubmitting(false);
    }
  };

  return <div className={`auth-panel ${compact ? 'compact' : ''}`} data-testid="auth-panel"><h3>{registering ? 'Create local account' : 'Sign in'}</h3><p>{auth.message || 'Private squads and manager data stay inside your account.'}</p><form onSubmit={submit}>{registering && <label>Display name<input name="displayName" autoComplete="name" /></label>}<label>Email<input name="email" type="email" autoComplete="email" required /></label><label>Password<input name="password" type="password" autoComplete={registering ? 'new-password' : 'current-password'} minLength={12} required /></label>{error && <div className="form-error" role="alert">{error}</div>}<button className="primary-button" disabled={submitting}>{submitting ? 'Please wait…' : registering ? 'Create account' : 'Sign in'}</button></form>{auth.config?.registrationEnabled && <button className="text-button" onClick={() => { setRegistering(!registering); setError(''); }}>{registering ? 'Use an existing account' : 'Create a local account'}</button>}{auth.config && !auth.config.registrationEnabled && <small>Registration is disabled. Ask the local operator to bootstrap an account.</small>}<small>No email verification or password-reset email is available in this local deployment.</small></div>;
}

function PasswordForm({ onSubmit, disabled, error }: { onSubmit: (currentPassword: string, newPassword: string) => Promise<void>; disabled: boolean; error: string }) {
  return <form onSubmit={(event) => { event.preventDefault(); const data = new FormData(event.currentTarget); void onSubmit(String(data.get('currentPassword')), String(data.get('newPassword'))); }}><label>Current password<input name="currentPassword" type="password" autoComplete="current-password" required /></label><label>New password<input name="newPassword" type="password" autoComplete="new-password" minLength={12} required /></label>{error && <div className="form-error" role="alert">{error}</div>}<button className="primary-button" disabled={disabled}>Update password</button></form>;
}

function messageFor(reason: unknown) {
  return reason instanceof Error ? reason.message : 'Authentication request failed.';
}
