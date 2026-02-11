import { useEffect, useState, FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../lib/api';

export function Applications() {
  const [apps, setApps] = useState<any[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [uid, setUid] = useState('');

  const load = () => {
    api.listApps().then((res) => setApps(res.data || [])).catch(() => {});
  };

  useEffect(load, []);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    await api.createApp({ name, uid: uid || undefined });
    setName('');
    setUid('');
    setShowCreate(false);
    load();
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this application? All endpoints and messages will be removed.')) return;
    await api.deleteApp(id);
    load();
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.5rem', fontWeight: 700 }}>Applications</h1>
        <button className="btn-primary" onClick={() => setShowCreate(!showCreate)}>
          Create Application
        </button>
      </div>

      {showCreate && (
        <div className="card" style={{ marginBottom: '1.5rem' }}>
          <form onSubmit={handleCreate} style={{ display: 'flex', gap: '0.75rem', alignItems: 'flex-end' }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: '0.75rem', display: 'block', marginBottom: '0.25rem' }}>Name</label>
              <input value={name} onChange={(e) => setName(e.target.value)} required placeholder="My Customer" />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: '0.75rem', display: 'block', marginBottom: '0.25rem' }}>UID (optional)</label>
              <input value={uid} onChange={(e) => setUid(e.target.value)} placeholder="user_123" />
            </div>
            <button type="submit" className="btn-primary">Create</button>
          </form>
        </div>
      )}

      <div className="card" style={{ padding: 0 }}>
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>UID</th>
              <th>Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {apps.length === 0 && (
              <tr>
                <td colSpan={4} style={{ textAlign: 'center', color: 'var(--text-secondary)', padding: '2rem' }}>
                  No applications yet
                </td>
              </tr>
            )}
            {apps.map((app) => (
              <tr key={app.id}>
                <td>
                  <Link to={`/applications/${app.id}`} style={{ fontWeight: 500 }}>{app.name}</Link>
                </td>
                <td style={{ color: 'var(--text-secondary)', fontFamily: 'monospace', fontSize: '0.8rem' }}>
                  {app.uid || '—'}
                </td>
                <td style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
                  {new Date(app.created_at).toLocaleDateString()}
                </td>
                <td>
                  <button className="btn-danger" style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }} onClick={() => handleDelete(app.id)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
