import { useEffect, useState, FormEvent } from 'react';
import { api } from '../lib/api';

export function APIKeys() {
  const [keys, setKeys] = useState<any[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [newKey, setNewKey] = useState<string | null>(null);

  const load = () => {
    api.listAPIKeys().then((res) => setKeys(res.data || [])).catch(() => {});
  };

  useEffect(load, []);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    const res = await api.createAPIKey({ name });
    setNewKey(res.key);
    setName('');
    setShowCreate(false);
    load();
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this API key? Any integrations using it will stop working.')) return;
    await api.deleteAPIKey(id);
    load();
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.5rem', fontWeight: 700 }}>API Keys</h1>
        <button className="btn-primary" onClick={() => { setShowCreate(!showCreate); setNewKey(null); }}>
          Create API Key
        </button>
      </div>

      {newKey && (
        <div className="card" style={{ marginBottom: '1.5rem', borderColor: 'var(--success)' }}>
          <p style={{ fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.5rem' }}>
            API Key Created — copy it now, it won't be shown again:
          </p>
          <div style={{
            background: 'var(--bg)', padding: '0.75rem', borderRadius: '0.375rem',
            fontFamily: 'monospace', fontSize: '0.85rem', wordBreak: 'break-all',
          }}>
            {newKey}
          </div>
        </div>
      )}

      {showCreate && (
        <div className="card" style={{ marginBottom: '1.5rem' }}>
          <form onSubmit={handleCreate} style={{ display: 'flex', gap: '0.75rem', alignItems: 'flex-end' }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: '0.75rem', display: 'block', marginBottom: '0.25rem' }}>Name</label>
              <input value={name} onChange={(e) => setName(e.target.value)} required placeholder="Production key" />
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
              <th>Prefix</th>
              <th>Created</th>
              <th>Expires</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {keys.length === 0 && (
              <tr>
                <td colSpan={5} style={{ textAlign: 'center', color: 'var(--text-secondary)', padding: '2rem' }}>
                  No API keys yet
                </td>
              </tr>
            )}
            {keys.map((k) => (
              <tr key={k.id}>
                <td style={{ fontWeight: 500 }}>{k.name}</td>
                <td style={{ fontFamily: 'monospace', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                  rbf_{k.prefix}_***
                </td>
                <td style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
                  {new Date(k.created_at).toLocaleDateString()}
                </td>
                <td style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
                  {k.expires_at ? new Date(k.expires_at).toLocaleDateString() : 'Never'}
                </td>
                <td>
                  <button className="btn-danger" style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }} onClick={() => handleDelete(k.id)}>
                    Revoke
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
