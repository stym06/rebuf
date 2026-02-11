import { Link, Outlet, useNavigate } from 'react-router-dom';

export function Layout() {
  const navigate = useNavigate();

  const logout = () => {
    localStorage.removeItem('token');
    navigate('/login');
  };

  return (
    <div style={{ display: 'flex', minHeight: '100vh' }}>
      <nav
        style={{
          width: 240,
          background: 'var(--bg-secondary)',
          borderRight: '1px solid var(--border)',
          padding: '1.5rem 1rem',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <div style={{ fontSize: '1.25rem', fontWeight: 700, marginBottom: '2rem', padding: '0 0.5rem' }}>
          rebuf
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', flex: 1 }}>
          <NavLink to="/">Dashboard</NavLink>
          <NavLink to="/applications">Applications</NavLink>
          <NavLink to="/api-keys">API Keys</NavLink>
        </div>
        <button onClick={logout} className="btn-secondary" style={{ width: '100%' }}>
          Log out
        </button>
      </nav>
      <main style={{ flex: 1, padding: '2rem' }}>
        <Outlet />
      </main>
    </div>
  );
}

function NavLink({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <Link
      to={to}
      style={{
        padding: '0.5rem 0.75rem',
        borderRadius: '0.375rem',
        color: 'var(--text-secondary)',
        fontSize: '0.875rem',
      }}
    >
      {children}
    </Link>
  );
}
