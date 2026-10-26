import { useEffect, useState } from 'react'

// Staff PWA — one app for Admin, Waiter, Kitchen, Cashier and
// HRIS. Server-side role checks still enforce BRD §5.1; the UI
// only hides what the role cannot do, it never grants access.
// Auth is Crew ID + 6-digit PIN, session cookie (HttpOnly).
// Receipts are fetched as bill data and printed with the
// browser print dialog; Web Bluetooth (BLE) printing is a
// hardware-specific step and is not claimed here.

type Any = any
const rp = (n: number) => 'Rp ' + (n || 0).toLocaleString('id-ID')
async function api(path: string, opts: RequestInit = {}) {
  const res = await fetch(path, { ...opts, credentials: 'include', headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) } })
  const text = await res.text()
  let data: Any = null
  try { data = text ? JSON.parse(text) : null } catch { data = text }
  if (!res.ok) { const e: Any = new Error(data?.title || data?.code || `Request failed (${res.status})`); e.data = data; throw e }
  return data
}
function Field({ label, value, onChange, type = 'text', placeholder }: Any) {
  return <div><label>{label}</label><input type={type} value={value} onChange={e => onChange(e.target.value)} placeholder={placeholder} /></div>
}

export default function App() {
  const [me, setMe] = useState<Any>(null)
  const [crewId, setCrewId] = useState('')
  const [pin, setPin] = useState('')
  const [tab, setTab] = useState('waiter')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [lang, setLang] = useState<'id' | 'en'>('id')

  useEffect(() => { api('/auth/me').then(setMe).catch(() => {}) }, [])
  async function login() {
    setError('')
    try { await api('/auth/login', { method: 'POST', body: JSON.stringify({ crew_id: crewId, pin }) }); const m = await api('/auth/me'); setMe(m); setNotice('Signed in as ' + m.full_name) } catch (e: any) { setError(e.message) }
  }
  async function logout() { await api('/auth/logout', { method: 'POST' }); setMe(null) }

  if (!me) {
    return <div className="container"><h1>🍣 Sushi Latte — Staff</h1>
      <div className="card" style={{ maxWidth: 420 }}>
        <h2>Sign in / Masuk</h2><p className="muted">Crew ID + 6-digit PIN. Management sessions expire after 30 minutes of inactivity; others after 12 hours.</p>
        <Field label="Crew ID" value={crewId} onChange={setCrewId} />
        <Field label="PIN" value={pin} onChange={setPin} type="password" />
        {error && <div className="error">{error}</div>}
        <button className="primary" style={{ marginTop: 12 }} onClick={login}>Sign in</button>
      </div></div>
  }
  const tabs = ['waiter', 'kitchen', 'cashier', 'admin', 'hris', 'clock']
  return (
    <div className="container">
      <div className="row"><h1 style={{ flex: 1 }}>🍣 Staff — {me.full_name} ({me.crew_id}, {me.role})</h1>
        <button onClick={() => setLang(lang === 'id' ? 'en' : 'id')}>{lang === 'id' ? 'EN' : 'ID'}</button>
        <button onClick={logout}>Logout</button></div>
      {error && <div className="error">{error}</div>}
      {notice && <div className="success">{notice}</div>}
      <div className="tabs">{tabs.map(t => <button key={t} className={tab === t ? 'active' : ''} onClick={() => setTab(t)}>{t}</button>)}</div>
      {tab === 'waiter' && <Waiter setError={setError} setNotice={setNotice} />}
      {tab === 'kitchen' && <Kitchen setError={setError} setNotice={setNotice} />}
      {tab === 'cashier' && <Cashier setError={setError} setNotice={setNotice} />}
      {tab === 'admin' && <Admin setError={setError} setNotice={setNotice} />}
      {tab === 'hris' && <Hris setError={setError} setNotice={setNotice} />}
      {tab === 'clock' && <ClockScreen setError={setError} setNotice={setNotice} />}
    </div>
  )
}

function useList(path: string, deps: any[] = []) {
  const [data, setData] = useState<Any>(null)
  const [refresh, setRefresh] = useState(0)
  useEffect(() => { api(path).then(setData).catch(() => setData(null)) }, [path, refresh, ...deps])
  return [data, () => setRefresh(r => r + 1)] as const
}

function Waiter({ setError, setNotice }: Any) {
  const [orders, reload] = useList('/api/v1/orders')
  const [billId, setBillId] = useState('')
  const [summary, setSummary] = useState<Any>(null)
  const [tableId, setTableId] = useState('')
  const [overrideToken, setOverrideToken] = useState('')
  async function act(fn: () => Promise<any>, msg: string) {
    try { await fn(); setNotice(msg); reload() } catch (e: any) { setError(e.message) }
  }
  return <div>
    <div className="card"><h2>Orders / Pesanan</h2>
      <button onClick={reload}>Refresh</button>
      <table><thead><tr><th>Order</th><th>Status</th><th>Claimed by</th><th>Placed by</th><th>Actions</th></tr></thead>
        <tbody>{(orders || []).map((o: Any) => <tr key={o.id}><td>{o.id.slice(-8)}</td><td>{o.status}</td><td>{o.claimed_by || '-'}</td><td>{o.placed_by}</td>
          <td><div className="row">
            <button onClick={() => act(() => api(`/api/v1/orders/${o.id}/claim`, { method: 'POST' }), 'Claimed')}>Claim</button>
            <button onClick={() => act(() => api(`/api/v1/orders/${o.id}/takeover`, { method: 'POST' }), 'Taken over')}>Takeover</button>
            <button onClick={() => act(() => api(`/api/v1/orders/${o.id}/confirm`, { method: 'POST', body: JSON.stringify({}) }), 'Confirmed')}>Confirm</button>
          </div></td></tr>)}</tbody></table>
      <h3>Order items</h3>
      {(orders || []).map((o: Any) => <div key={o.id}>{o.items?.map((it: Any) => <div key={it.id} className="row"><span style={{ flex: 1 }}>{it.product_name_snapshot} × {it.quantity} ({it.status}) {it.dietary_warnings?.length > 0 && <span className="badge red">⚠ {it.dietary_warnings.join(', ')}</span>} {it.sold_out_flag ? <span className="badge red">Sold Out flag</span> : ''}</span>
        <button onClick={() => act(() => api(`/api/v1/order-items/${it.id}/serve`, { method: 'POST' }), 'Served')}>Serve</button>
        <button onClick={() => act(() => api(`/api/v1/order-items/${it.id}/cancel`, { method: 'POST', body: JSON.stringify({ reason: 'customer_request' }) }), 'Cancelled')}>Cancel</button>
        <button onClick={() => act(() => api(`/api/v1/order-items/${it.id}/void`, { method: 'POST', headers: { 'X-Override-Token': overrideToken }, body: JSON.stringify({ reason: 'wrong order' }) }), 'Voided')}>Void (override)</button>
        <button onClick={() => act(() => api(`/api/v1/order-items/${it.id}/remake`, { method: 'POST', headers: { 'X-Override-Token': overrideToken }, body: JSON.stringify({ reason: 'wrong order' }) }), 'Remade')}>Remake (override)</button>
      </div>)}</div>)}
    </div>
    <div className="card"><h2>Bill / Tagihan</h2>
      <Field label="Bill ID" value={billId} onChange={setBillId} />
      <Field label="PIN Override token (get one below in Cashier/Admin via /auth/override, or paste here)" value={overrideToken} onChange={setOverrideToken} />
      <div className="row"><button onClick={async () => setSummary(await api(`/api/v1/bills/${billId}/summary`))}>Summary</button>
        <button onClick={() => act(() => api(`/api/v1/bills/${billId}/settle`, { method: 'POST' }), 'Settling')}>Settle</button>
        <button onClick={() => act(() => api(`/api/v1/bills/${billId}/unlock`, { method: 'POST' }), 'Unlocked')}>Unlock</button>
        <button onClick={() => act(() => api(`/api/v1/bills/${billId}/receipts/reprint`, { method: 'POST' }), 'REPRINT recorded')}>Reprint receipt</button></div>
      {summary && <div><p>Bill {summary.bill?.number} — {summary.bill?.status} · Tables: {(summary.table_labels || []).join(', ')}</p><p>Subtotal {rp(summary.subtotal)} · Service {rp(summary.service_charge)} · Tax {rp(summary.tax)} · Total {rp(summary.total)}</p>
        <button onClick={() => window.print()}>Print receipt (browser print)</button></div>}
      <h3>Direct order for a Table</h3><p className="muted">Use a Table ID (from Admin → Tables). This creates a Confirmed order straight to the Kitchen.</p>
      <Field label="Table ID" value={tableId} onChange={setTableId} />
      <button onClick={async () => { try { const prod = await api('/api/v1/catalog'); const p = prod.products?.[0]; if (!p) throw new Error('No products yet'); const o = await api('/api/v1/orders', { method: 'POST', body: JSON.stringify({ table_id: tableId, items: [{ product_id: p.id, variant_id: p.variants[0].id, quantity: 1 }] }) }); setNotice('Direct order created: ' + o.id); reload() } catch (e: any) { setError(e.message) } }}>Create direct order (first product, qty 1)</button>
    </div>
    <div className="card"><h2>My claims</h2><MyClaims /></div>
  </div>
}
function MyClaims() {
  const [claims] = useList('/api/v1/waiters/me/claims')
  return <div>{(claims || []).map((o: Any) => <p key={o.id}>{o.id.slice(-8)} — {o.status} — {o.placed_by}</p>)}</div>
}

function Kitchen({ setError, setNotice }: Any) {
  const [prepArea, setPrepArea] = useState('prep-kitchen')
  const [queue, reload] = useList(`/api/v1/prep-areas/${prepArea}/queue`)
  return <div className="card"><h2>Kitchen queue / Antrian dapur</h2>
    <Field label="Prep Area ID" value={prepArea} onChange={setPrepArea} />
    <button onClick={reload}>Refresh</button>
    <h3>Confirmed / Preparing</h3>
    {(queue?.items || []).map((it: Any) => <div key={it.id} className="row"><span style={{ flex: 1 }}>{it.is_remake && <span className="badge red">REMAKE</span>} {it.product_name_snapshot} × {it.quantity} — {it.status} — {it.placed_by} — {it.table_labels?.join(', ')} — elapsed {it.elapsed_seconds}s / prep {it.prep_time_seconds}s {it.late_status && <span className="badge red">{it.late_status}</span>} {it.dietary_warnings?.length > 0 && <span className="badge red">⚠ {it.dietary_warnings.join(', ')}</span>} {it.note && `· Note: ${it.note}`}</span>
      <button onClick={async () => { try { await api(`/api/v1/order-items/${it.id}/transition`, { method: 'POST', body: JSON.stringify({ to_status: it.status === 'confirmed' ? 'preparing' : 'ready' }) }); setNotice('Transitioned'); reload() } catch (e: any) { setError(e.message) } }}>{it.status === 'confirmed' ? 'Start preparing' : 'Mark ready'}</button>
      <button onClick={async () => { try { await api(`/api/v1/order-items/${it.id}/kitchen-cancel`, { method: 'POST', body: JSON.stringify({ reason: 'out_of_stock' }) }); setNotice('Cancelled'); reload() } catch (e: any) { setError(e.message) } }}>Cancel (out of stock)</button></div>)}
    <h3>Ready, waiting pickup</h3>
    {(queue?.ready_waiting_pickup || []).map((it: Any) => <p key={it.id}>{it.product_name_snapshot} — {it.placed_by} {it.waiting_pickup && <span className="badge red">waiting pickup</span>}</p>)}
  </div>
}

function Cashier({ setError, setNotice }: Any) {
  const [billId, setBillId] = useState('')
  const [summary, setSummary] = useState<Any>(null)
  const [method, setMethod] = useState('cash')
  const [amount, setAmount] = useState('')
  const [drawerId, setDrawerId] = useState('')
  const [drawerFloat, setDrawerFloat] = useState('')
  const [countedCash, setCountedCash] = useState('')
  const [overrideCrew, setOverrideCrew] = useState('')
  const [overridePin, setOverridePin] = useState('')
  const [overrideAction, setOverrideAction] = useState('void')
  const [overrideToken, setOverrideToken] = useState('')
  async function load() { try { setSummary(await api(`/api/v1/bills/${billId}/payments`)) } catch (e: any) { setError(e.message) } }
  return <div>
    <div className="card"><h2>Payments / Pembayaran</h2>
      <Field label="Bill ID" value={billId} onChange={setBillId} />
      <div className="row"><button onClick={load}>Load bill payments</button></div>
      {summary && <div><p>Bill {summary.bill?.number} — {summary.bill?.status} · Total {rp(summary.total)} · Paid {rp(summary.total_paid)} · Balance {rp(summary.balance)}</p>
        {(summary.payments || []).map((p: Any) => <div key={p.id} className="row"><span style={{ flex: 1 }}>{p.method} {rp(p.amount)} — {p.status}</span>{p.status === 'pending' && <><button onClick={async () => { await api(`/api/v1/payments/${p.id}/verify`, { method: 'POST' }); setNotice('QRIS verified'); load() }}>Verify QRIS</button><button onClick={async () => { await api(`/api/v1/payments/${p.id}/write-off`, { method: 'POST', body: JSON.stringify({ reason: 'not received' }) }); setNotice('QRIS written off'); load() }}>Write off</button></>}</div>)}
      </div>}
      <label>Method</label><select value={method} onChange={e => setMethod(e.target.value)}><option value="cash">Cash</option><option value="qris">QRIS</option><option value="card">Card</option></select>
      <Field label="Amount (Rp)" value={amount} onChange={setAmount} type="number" />
      <div className="row"><button className="primary" onClick={async () => { try { await api(`/api/v1/bills/${billId}/payments`, { method: 'POST', body: JSON.stringify({ method, amount: parseInt(amount), pay_now_serve_later: true }) }); setNotice('Payment recorded'); load() } catch (e: any) { setError(e.message) } }}>Record payment</button>
        <button className="green" onClick={async () => { try { const b = await api(`/api/v1/bills/${billId}/close`, { method: 'POST' }); setNotice('Bill closed: ' + b.status); load() } catch (e: any) { setError(e.message) } }}>Close bill</button></div>
      <h3>PIN Override</h3><p className="muted">Get a single-use override token (60 seconds) for void, remake, unpaid_bill, refund_le_cap, refund_gt_cap, manual_discount, unverified_qris.</p>
      <Field label="Authoriser Crew ID" value={overrideCrew} onChange={setOverrideCrew} />
      <Field label="Authoriser PIN" value={overridePin} onChange={setOverridePin} type="password" />
      <Field label="Action" value={overrideAction} onChange={setOverrideAction} />
      <button onClick={async () => { try { const t = await api('/auth/override', { method: 'POST', body: JSON.stringify({ crew_id: overrideCrew, pin: overridePin, action: overrideAction }) }); setOverrideToken(t.override_token); setNotice('Override token ready (paste it in Waiter/Cashier actions)') } catch (e: any) { setError(e.message) } }}>Get override token</button>
      {overrideToken && <p className="muted" style={{ wordBreak: 'break-all' }}>Token: {overrideToken}</p>}
    </div>
    <div className="card"><h2>Takeaway / Bawa pulang</h2><Takeaway setError={setError} setNotice={setNotice} /></div>
    <div className="card"><h2>Drawer / Laci kas</h2>
      <Field label="Opening float (Rp)" value={drawerFloat} onChange={setDrawerFloat} />
      <div className="row"><button onClick={async () => { try { const d = await api('/api/v1/drawer-sessions', { method: 'POST', body: JSON.stringify({ opening_float: parseInt(drawerFloat) || 0 }) }); setNotice('Drawer opened: ' + d.id); setDrawerId(d.id) } catch (e: any) { setError(e.message) } }}>Open drawer session</button></div>
      <Field label="Drawer session ID (to close)" value={drawerId} onChange={setDrawerId} />
      <Field label="Counted cash (Rp)" value={countedCash} onChange={setCountedCash} type="number" />
      <p className="muted">Expected cash = float + confirmed handovers + cash taken − refunds.</p>
      <button onClick={async () => { try { const d = await api(`/api/v1/drawer-sessions/${drawerId}/close`, { method: 'POST', body: JSON.stringify({ counted_cash: parseInt(countedCash) || 0 }) }); setNotice(`Drawer closed: expected ${rp(d.expected_cash)}, difference ${rp(d.difference)}`) } catch (e: any) { setError(e.message) } }}>Close drawer session</button>
    </div>
    <div className="card"><h2>Post-outage entry / Entri pasca gangguan</h2>
      <button onClick={async () => { try { const b = await api('/api/v1/post-outage/bills', { method: 'POST', body: JSON.stringify({ bill_number: 'OUTAGE-001', items: [], payments: [] }) }); setNotice('Post-outage bill created: ' + b.id) } catch (e: any) { setError(e.message) } }}>Create empty post-outage bill (demo; add items via API)</button>
    </div>
  </div>
}
function Takeaway({ setError, setNotice }: Any) {
  const [name, setName] = useState('')
  const [billId, setBillId] = useState('')
  return <div>
    <Field label="Customer name" value={name} onChange={setName} />
    <button onClick={async () => { try { const b = await api('/api/v1/bills/takeaway', { method: 'POST', body: JSON.stringify({ customer_name: name }) }); setBillId(b.id); setNotice('Takeaway bill: ' + b.number) } catch (e: any) { setError(e.message) } }}>Open takeaway bill</button>
    {billId && <p className="muted">Bill ID: {billId} — pay it in Payments above (use this Bill ID), then confirm its orders:</p>}
    <button onClick={async () => { try { const os = await api(`/api/v1/bills/takeaway/${billId}/confirm-orders`, { method: 'POST' }); setNotice(`Confirmed ${os.length} takeaway order(s) to Kitchen`) } catch (e: any) { setError(e.message) } }}>Confirm takeaway orders after payment</button>
  </div>
}

function Admin({ setError, setNotice }: Any) {
  const [catalog] = useList('/api/v1/catalog')
  const [tables] = useList('/api/v1/tables')
  const [slides] = useList('/api/v1/display/slides')
  const [reports, setReports] = useState<Any>(null)
  const [audit] = useList('/api/v1/audit-log?limit=20')
  const [newCat, setNewCat] = useState('')
  const [newTable, setNewTable] = useState('')
  return <div>
    <div className="card"><h2>Catalog / Katalog</h2>
      <p className="muted">Management only for mutations. Products are created via the API; this screen lists them and creates categories.</p>
      <table><thead><tr><th>Product</th><th>Category</th><th>Status</th><th>Variants</th></tr></thead><tbody>
        {(catalog?.products || []).map((p: Any) => <tr key={p.id}><td>{p.name_id} / {p.name_en}</td><td>{p.category_id}</td><td>{p.status}</td><td>{(p.variants || []).map((v: Any) => `${v.name_id} ${rp(v.price)}${v.sold_out ? ' (Sold Out)' : ''}`).join(', ')}</td></tr>)}
      </tbody></table>
      <Field label="New category name (ID)" value={newCat} onChange={setNewCat} />
      <button onClick={async () => { try { await api('/api/v1/categories', { method: 'POST', body: JSON.stringify({ name_id: newCat, name_en: newCat, sort: 99, prep_area_id: 'prep-kitchen' }) }); setNotice('Category created'); location.reload() } catch (e: any) { setError(e.message) } }}>Create category</button>
    </div>
    <div className="card"><h2>Tables & QR / Meja & QR</h2>
      <table><tbody>{(tables || []).map((t: Any) => <tr key={t.id}><td>{t.label}</td><td>{t.active ? 'active' : 'inactive'}</td><td><a href={`/api/v1/tables/${t.id}/qr.png`} target="_blank">QR PNG</a></td><td><button onClick={async () => { await api(`/api/v1/tables/${t.id}/regenerate-qr`, { method: 'POST' }); setNotice('QR regenerated — old printed QR no longer works'); location.reload() }}>Regenerate QR</button></td></tr>)}</tbody></table>
      <Field label="New table label" value={newTable} onChange={setNewTable} />
      <button onClick={async () => { try { await api('/api/v1/tables', { method: 'POST', body: JSON.stringify({ label: newTable }) }); setNotice('Table created'); location.reload() } catch (e: any) { setError(e.message) } }}>Create table</button>
    </div>
    <div className="card"><h2>Display slides / Slide display</h2>
      {(slides || []).map((s: Any) => <p key={s.id}>{s.template} — {s.headline_id} — sort {s.sort} — {s.active ? 'active' : 'inactive'}</p>)}
      <button onClick={async () => { try { await api('/api/v1/display/slides', { method: 'POST', body: JSON.stringify({ template: 'hero_promo', headline_id: 'Promo Hari Ini', headline_en: 'Today Promo', sort: 1, duration_seconds: 10, active: true }) }); setNotice('Slide created'); location.reload() } catch (e: any) { setError(e.message) } }}>Create hero promo slide</button>
    </div>
    <div className="card"><h2>Reports / Laporan</h2>
      <button onClick={async () => setReports(await api('/api/v1/reports/daily-sales'))}>Daily sales</button>
      <button onClick={async () => setReports(await api('/api/v1/reports/best-sellers'))}>Best sellers</button>
      <button onClick={async () => setReports(await api('/api/v1/reports/unpaid-bills'))}>Unpaid bills</button>
      {reports && <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>{JSON.stringify(reports, null, 2)}</pre>}
    </div>
    <div className="card"><h2>Audit log</h2>
      {(audit?.data || []).map((a: Any) => <p key={a.id} className="muted">{new Date(a.at).toLocaleString()} — {a.actor_name || a.actor_crew_id} — {a.action} {a.entity} {a.entity_id} {a.reason || ''}</p>)}
    </div>
  </div>
}

function Hris({ setError, setNotice }: Any) {
  const [employees] = useList('/api/v1/employees')
  const [attendance] = useList('/api/v1/attendance')
  const [leave] = useList('/api/v1/leave-requests')
  const [crewId, setCrewId] = useState('')
  return <div>
    <div className="card"><h2>Employees / Karyawan</h2>
      {(employees || []).map((e: Any) => <p key={e.id}>{e.full_name} ({e.crew_id}) — {e.employment_type || ''} — hired {e.hire_date || '-'}</p>)}
      <Field label="Crew ID for new employee record" value={crewId} onChange={setCrewId} />
      <button onClick={async () => { try { await api('/api/v1/employees', { method: 'POST', body: JSON.stringify({ crew_id: crewId }) }); setNotice('Employee record created'); location.reload() } catch (e: any) { setError(e.message) } }}>Create employee record (Management)</button>
    </div>
    <div className="card"><h2>Attendance / Kehadiran</h2>
      {(attendance || []).map((a: Any) => <p key={a.id}>{a.crew_id} — in {new Date(a.clock_in_at).toLocaleString()} — out {a.clock_out_at ? new Date(a.clock_out_at).toLocaleString() : '—'} {a.auto_clocked_out && <span className="badge red">auto</span>} {a.correction_reason && `· corrected: ${a.correction_reason}`}</p>)}
      <div className="row"><button onClick={async () => { const r = await api('/api/v1/attendance/auto-clock-out', { method: 'POST' }); setNotice(`Auto clocked out ${r.length} row(s)`) }}>Run auto clock-out now</button>
        <a href="/api/v1/attendance/export" target="_blank"><button>Export CSV for payroll</button></a></div>
    </div>
    <div className="card"><h2>Leave / Cuti</h2>
      {(leave || []).map((l: Any) => <div key={l.id} className="row"><span style={{ flex: 1 }}>{l.crew_id} — {l.from_date} to {l.to_date} — {l.type} — {l.status}</span>
        {l.status === 'pending' && <><button onClick={async () => { await api(`/api/v1/leave-requests/${l.id}/decide`, { method: 'POST', body: JSON.stringify({ decision: 'approved' }) }); setNotice('Leave approved'); location.reload() }}>Approve</button><button onClick={async () => { await api(`/api/v1/leave-requests/${l.id}/decide`, { method: 'POST', body: JSON.stringify({ decision: 'rejected' }) }); setNotice('Leave rejected'); location.reload() }}>Reject</button></>}</div>)}
    </div>
  </div>
}

function ClockScreen({ setError, setNotice }: Any) {
  const [screenCrew, setScreenCrew] = useState('')
  const [screenPin, setScreenPin] = useState('')
  const [screenToken, setScreenToken] = useState('')
  const [crewId, setCrewId] = useState('')
  const [pin, setPin] = useState('')
  return <div className="card"><h2>HRIS clock-in screen / Layar absensi</h2>
    <p className="muted">Step 1: an Outlet Admin signs the screen in once per day. Step 2: crew clock in/out with Crew ID + PIN on this screen only.</p>
    <Field label="Outlet Admin Crew ID (screen sign-in)" value={screenCrew} onChange={setScreenCrew} />
    <Field label="Outlet Admin PIN" value={screenPin} onChange={setScreenPin} type="password" />
    <button onClick={async () => { try { const s = await api('/auth/screen-login', { method: 'POST', body: JSON.stringify({ crew_id: screenCrew, pin: screenPin, screen: 'hris' }) }); setScreenToken(s.screen_token); setNotice('HRIS screen signed in') } catch (e: any) { setError(e.message) } }}>Sign in HRIS screen</button>
    {screenToken && <><Field label="Crew ID (clock)" value={crewId} onChange={setCrewId} />
      <Field label="PIN (clock)" value={pin} onChange={setPin} type="password" />
      <div className="row"><button className="green" onClick={async () => { try { const a = await api('/api/v1/attendance/clock', { method: 'POST', headers: { 'X-Screen-Token': screenToken }, body: JSON.stringify({ crew_id: crewId, pin, action: 'in' }) }); setNotice('Clocked in: ' + a.id) } catch (e: any) { setError(e.message) } }}>Clock in</button>
        <button onClick={async () => { try { const a = await api('/api/v1/attendance/clock', { method: 'POST', headers: { 'X-Screen-Token': screenToken }, body: JSON.stringify({ crew_id: crewId, pin, action: 'out' }) }); setNotice('Clocked out: ' + a.id) } catch (e: any) { setError(e.message) } }}>Clock out</button></div></>}
  </div>
}
