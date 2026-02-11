import { useEffect, useState } from 'react';
import { api } from '../lib/api';

export function Dashboard() {
  const [stats, setStats] = useState({ apps: 0, endpoints: 0, messages: 0 });

  useEffect(() => {
    api.listApps().then((res) => {
      setStats((s) => ({ ...s, apps: res.data?.length ?? 0 }));
    }).catch(() => {});
  }, []);

  return (
    <div>
      <h1 style={{ fontSize: '1.5rem', fontWeight: 700, marginBottom: '1.5rem' }}>Dashboard</h1>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '1rem', marginBottom: '2rem' }}>
        <StatCard label="Applications" value={stats.apps} />
        <StatCard label="Total Endpoints" value={stats.endpoints} />
        <StatCard label="Messages (24h)" value={stats.messages} />
      </div>

      <div className="card">
        <h2 style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '1rem' }}>Quick Start</h2>
        <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', lineHeight: 1.8 }}>
          <p><strong>1.</strong> Create an <a href="/applications">Application</a> for each of your customers</p>
          <p><strong>2.</strong> Add <strong>Endpoints</strong> (webhook URLs) to each application</p>
          <p><strong>3.</strong> Send messages via the API — Rebuf handles delivery, retries, and signing</p>
        </div>
        <div style={{ marginTop: '1rem', background: 'var(--bg)', padding: '1rem', borderRadius: '0.375rem', fontFamily: 'monospace', fontSize: '0.8rem', overflowX: 'auto' }}>
          <pre style={{ color: 'var(--text-secondary)' }}>
{`curl -X POST http://localhost:8080/api/v1/app/{app_id}/msg/ \\
  -H "Authorization: ApiKey rbf_xxxx_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{"event_type": "invoice.paid", "payload": {"id": "inv_123"}}'`}
          </pre>
        </div>
      </div>
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="card">
      <div style={{ color: 'var(--text-secondary)', fontSize: '0.75rem', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
        {label}
      </div>
      <div style={{ fontSize: '2rem', fontWeight: 700, marginTop: '0.25rem' }}>{value}</div>
    </div>
  );
}
