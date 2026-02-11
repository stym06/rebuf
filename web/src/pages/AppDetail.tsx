import { useEffect, useState, FormEvent } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../lib/api';

export function AppDetail() {
  const { appId } = useParams<{ appId: string }>();
  const [app, setApp] = useState<any>(null);
  const [endpoints, setEndpoints] = useState<any[]>([]);
  const [messages, setMessages] = useState<any[]>([]);
  const [tab, setTab] = useState<'endpoints' | 'messages'>('endpoints');

  // Endpoint form
  const [showEpForm, setShowEpForm] = useState(false);
  const [epUrl, setEpUrl] = useState('');
  const [epDesc, setEpDesc] = useState('');

  // Message form
  const [showMsgForm, setShowMsgForm] = useState(false);
  const [msgEventType, setMsgEventType] = useState('');
  const [msgPayload, setMsgPayload] = useState('{}');

  // Attempts
  const [selectedMsg, setSelectedMsg] = useState<string | null>(null);
  const [attempts, setAttempts] = useState<any[]>([]);

  useEffect(() => {
    if (!appId) return;
    api.getApp(appId).then(setApp).catch(() => {});
    loadEndpoints();
    loadMessages();
  }, [appId]);

  const loadEndpoints = () => {
    if (!appId) return;
    api.listEndpoints(appId).then((res) => setEndpoints(res.data || [])).catch(() => {});
  };

  const loadMessages = () => {
    if (!appId) return;
    api.listMessages(appId).then((res) => setMessages(res.data || [])).catch(() => {});
  };

  const createEndpoint = async (e: FormEvent) => {
    e.preventDefault();
    if (!appId) return;
    await api.createEndpoint(appId, { url: epUrl, description: epDesc });
    setEpUrl('');
    setEpDesc('');
    setShowEpForm(false);
    loadEndpoints();
  };

  const sendMessage = async (e: FormEvent) => {
    e.preventDefault();
    if (!appId) return;
    try {
      const payload = JSON.parse(msgPayload);
      await api.createMessage(appId, { event_type: msgEventType, payload });
      setMsgEventType('');
      setMsgPayload('{}');
      setShowMsgForm(false);
      loadMessages();
    } catch {
      alert('Invalid JSON payload');
    }
  };

  const viewAttempts = async (msgId: string) => {
    if (!appId) return;
    setSelectedMsg(msgId);
    const res = await api.listAttempts(appId, msgId);
    setAttempts(res.data || []);
  };

  if (!app) return <div>Loading...</div>;

  return (
    <div>
      <h1 style={{ fontSize: '1.5rem', fontWeight: 700, marginBottom: '0.25rem' }}>{app.name}</h1>
      <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem', fontFamily: 'monospace', marginBottom: '1.5rem' }}>
        {app.id}
      </p>

      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1.5rem' }}>
        <button className={tab === 'endpoints' ? 'btn-primary' : 'btn-secondary'} onClick={() => setTab('endpoints')}>
          Endpoints ({endpoints.length})
        </button>
        <button className={tab === 'messages' ? 'btn-primary' : 'btn-secondary'} onClick={() => setTab('messages')}>
          Messages ({messages.length})
        </button>
      </div>

      {tab === 'endpoints' && (
        <div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '1rem' }}>
            <button className="btn-primary" onClick={() => setShowEpForm(!showEpForm)}>Add Endpoint</button>
          </div>

          {showEpForm && (
            <div className="card" style={{ marginBottom: '1rem' }}>
              <form onSubmit={createEndpoint} style={{ display: 'flex', gap: '0.75rem', alignItems: 'flex-end' }}>
                <div style={{ flex: 2 }}>
                  <label style={{ fontSize: '0.75rem', display: 'block', marginBottom: '0.25rem' }}>URL</label>
                  <input value={epUrl} onChange={(e) => setEpUrl(e.target.value)} required placeholder="https://example.com/webhook" />
                </div>
                <div style={{ flex: 1 }}>
                  <label style={{ fontSize: '0.75rem', display: 'block', marginBottom: '0.25rem' }}>Description</label>
                  <input value={epDesc} onChange={(e) => setEpDesc(e.target.value)} placeholder="Production" />
                </div>
                <button type="submit" className="btn-primary">Add</button>
              </form>
            </div>
          )}

          <div className="card" style={{ padding: 0 }}>
            <table>
              <thead>
                <tr>
                  <th>URL</th>
                  <th>Description</th>
                  <th>Status</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {endpoints.length === 0 && (
                  <tr><td colSpan={4} style={{ textAlign: 'center', color: 'var(--text-secondary)', padding: '2rem' }}>No endpoints</td></tr>
                )}
                {endpoints.map((ep) => (
                  <tr key={ep.id}>
                    <td style={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>{ep.url}</td>
                    <td style={{ color: 'var(--text-secondary)' }}>{ep.description || '—'}</td>
                    <td>
                      <span className={`badge ${ep.disabled ? 'badge-danger' : 'badge-success'}`}>
                        {ep.disabled ? 'Disabled' : 'Active'}
                      </span>
                    </td>
                    <td style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
                      {new Date(ep.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {tab === 'messages' && (
        <div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '1rem' }}>
            <button className="btn-primary" onClick={() => setShowMsgForm(!showMsgForm)}>Send Message</button>
          </div>

          {showMsgForm && (
            <div className="card" style={{ marginBottom: '1rem' }}>
              <form onSubmit={sendMessage} style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                <div>
                  <label style={{ fontSize: '0.75rem', display: 'block', marginBottom: '0.25rem' }}>Event Type</label>
                  <input value={msgEventType} onChange={(e) => setMsgEventType(e.target.value)} required placeholder="invoice.paid" />
                </div>
                <div>
                  <label style={{ fontSize: '0.75rem', display: 'block', marginBottom: '0.25rem' }}>Payload (JSON)</label>
                  <textarea
                    value={msgPayload}
                    onChange={(e) => setMsgPayload(e.target.value)}
                    rows={4}
                    style={{
                      background: 'var(--bg)',
                      border: '1px solid var(--border)',
                      color: 'var(--text)',
                      padding: '0.5rem 0.75rem',
                      borderRadius: '0.375rem',
                      fontFamily: 'monospace',
                      fontSize: '0.8rem',
                      width: '100%',
                      resize: 'vertical',
                    }}
                  />
                </div>
                <button type="submit" className="btn-primary" style={{ alignSelf: 'flex-start' }}>Send</button>
              </form>
            </div>
          )}

          <div className="card" style={{ padding: 0 }}>
            <table>
              <thead>
                <tr>
                  <th>Event Type</th>
                  <th>Event ID</th>
                  <th>Sent</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {messages.length === 0 && (
                  <tr><td colSpan={4} style={{ textAlign: 'center', color: 'var(--text-secondary)', padding: '2rem' }}>No messages</td></tr>
                )}
                {messages.map((msg) => (
                  <tr key={msg.id}>
                    <td>
                      <span className="badge badge-neutral">{msg.event_type}</span>
                    </td>
                    <td style={{ fontFamily: 'monospace', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                      {msg.event_id || msg.id.slice(0, 8)}
                    </td>
                    <td style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
                      {new Date(msg.created_at).toLocaleString()}
                    </td>
                    <td>
                      <button className="btn-secondary" style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }} onClick={() => viewAttempts(msg.id)}>
                        Attempts
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {selectedMsg && (
            <div className="card" style={{ marginTop: '1rem' }}>
              <h3 style={{ fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.75rem' }}>
                Delivery Attempts
              </h3>
              {attempts.length === 0 ? (
                <p style={{ color: 'var(--text-secondary)', fontSize: '0.875rem' }}>No attempts yet</p>
              ) : (
                <table>
                  <thead>
                    <tr>
                      <th>Endpoint</th>
                      <th>Status</th>
                      <th>Response</th>
                      <th>Attempt #</th>
                      <th>Time</th>
                    </tr>
                  </thead>
                  <tbody>
                    {attempts.map((a) => (
                      <tr key={a.id}>
                        <td style={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>{a.endpoint_id.slice(0, 8)}</td>
                        <td>
                          <span className={`badge ${a.status === 'success' ? 'badge-success' : a.status === 'failed' ? 'badge-danger' : 'badge-warning'}`}>
                            {a.status}
                          </span>
                        </td>
                        <td style={{ fontSize: '0.8rem' }}>{a.response_status_code || '—'}</td>
                        <td>{a.attempt_number}</td>
                        <td style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
                          {new Date(a.updated_at).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
