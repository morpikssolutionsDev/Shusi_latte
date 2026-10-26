import { useEffect, useMemo, useState } from 'react'

// Client Menu — Sushi Latte, Phase 2 API.
// Flow: static order URL (/order/{slug}, e.g. /order/table3) or
// QR token (?table=<qr_token>) -> scan -> join (name, restriction,
// consent) -> menu -> order -> bill. Guest token and customer id are
// kept in localStorage so a returning device can rejoin.

type Any = any
const rp = (n: number) => 'Rp ' + n.toLocaleString('id-ID')
async function api(path: string, opts: RequestInit = {}) {
  const res = await fetch(path, { ...opts, headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) } })
  const text = await res.text()
  let data: Any = null
  try { data = text ? JSON.parse(text) : null } catch { data = text }
  if (!res.ok) { const err: Any = new Error(data?.title || data?.code || `Request failed (${res.status})`); err.data = data; throw err }
  return data
}

export default function App() {
  const [lang, setLang] = useState<'id' | 'en'>('id')
  // Static order slug from /order/{slug}, else the old ?table= token.
  const [slug] = useState(() => {
    const m = location.pathname.match(/^\/order\/([^/]+)/)
    return m ? decodeURIComponent(m[1]) : (new URLSearchParams(location.search).get('slug') || '')
  })
  const [token, setToken] = useState(() => new URLSearchParams(location.search).get('table') || localStorage.getItem('sushi_table_token') || '')
  const [scan, setScan] = useState<Any>(null)
  const [guest, setGuest] = useState(() => localStorage.getItem('sushi_guest_token') || '')
  const [customerId, setCustomerId] = useState(() => localStorage.getItem('sushi_customer_id') || '')
  const [name, setName] = useState('')
  const [note, setNote] = useState('')
  const [consent, setConsent] = useState(false)
  const [restriction, setRestriction] = useState<string[]>([])
  const [catalog, setCatalog] = useState<Any>(null)
  const [cart, setCart] = useState<Any[]>([])
  const [bill, setBill] = useState<Any>(null)
  const [orders, setOrders] = useState<Any[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [warnings, setWarnings] = useState<Any>(null)
  const [view, setView] = useState<'scan' | 'menu' | 'bill'>('scan')

  const nameOf = (o: Any) => lang === 'id' ? o.name_id : o.name_en

  async function doScan(t: string) {
    setError('')
    try {
      const s = await api(`/public/tables/${t}`)
      setScan(s); localStorage.setItem('sushi_table_token', t)
      if (s.status === 'occupied' && guest) setView('menu')
    } catch (e: any) { setError(e.message) }
  }
  useEffect(() => {
    if (slug) {
      // Resolve the static slug to the Table's internal token, then
      // run the normal scan flow. Old printed QRs with a token
      // still work via ?table=.
      api(`/public/tables/by-slug/${encodeURIComponent(slug)}`)
        .then((tbl: Any) => { setToken(tbl.qr_token); doScan(tbl.qr_token) })
        .catch((e: Any) => setError(e.message))
    } else if (token) doScan(token)
  }, [])
  useEffect(() => { if (guest) { api('/public/catalog').then(setCatalog).catch(() => {}); loadBill() } }, [guest])

  async function loadBill() {
    try {
      const b = await api('/public/bills/me', { headers: { 'X-Guest-Token': guest } })
      setBill(b); setOrders(b.orders || [])
    } catch {}
  }
  async function join() {
    setError('')
    try {
      const j = await api(`/public/tables/${token}/join`, { method: 'POST', body: JSON.stringify({ name, restriction_tag_ids: restriction, dietary_note: note, consent }) })
      setGuest(j.guest_token); setCustomerId(j.customer?.id || '')
      localStorage.setItem('sushi_guest_token', j.guest_token)
      localStorage.setItem('sushi_customer_id', j.customer?.id || '')
      setView('menu'); setNotice(lang === 'id' ? 'Selamat datang!' : 'Welcome!')
    } catch (e: any) { setError(e.message) }
  }
  async function rejoin() {
    setError('')
    try {
      const j = await api(`/public/tables/${token}/rejoin`, { method: 'POST', body: JSON.stringify({ customer_entry_id: customerId }) })
      setGuest(j.guest_token); localStorage.setItem('sushi_guest_token', j.guest_token); setView('menu')
    } catch (e: any) { setError(e.message) }
  }
  async function placeOrder(acknowledge = false) {
    setError(''); setWarnings(null)
    const items = cart.map(c => ({ product_id: c.product.id, variant_id: c.variant.id, modifier_ids: c.modifiers, quantity: c.qty, note: c.note || undefined }))
    try {
      const o = await api('/public/orders', { method: 'POST', headers: { 'X-Guest-Token': guest }, body: JSON.stringify({ items, acknowledge_warnings: acknowledge }) })
      setCart([]); setNotice(`${lang === 'id' ? 'Pesanan dibuat' : 'Order placed'} — ${o.status}`); loadBill(); setView('bill')
    } catch (e: any) {
      // Dietary warning gate: server returns 409 with code
      // dietary_warning_ack_required and a warnings map.
      if (e.data?.code === 'dietary_warning_ack_required' || e.data?.warnings) {
        setWarnings(e.data.warnings || { message: e.message })
      } else setError(e.message)
    }
  }
  async function waiterCall() {
    try { await api('/public/waiter-calls', { method: 'POST', body: JSON.stringify({ table_qr_token: token, customer_entry_id: customerId || undefined }) }); setNotice(lang === 'id' ? 'Pelayan telah dipanggil.' : 'A waiter has been called.') } catch (e: any) { setError(e.message) }
  }
  async function notMine() {
    try { await api(`/public/tables/${token}/not-mine`, { method: 'POST', body: JSON.stringify({}) }); setNotice(lang === 'id' ? 'Ini bukan meja Anda. Pelayan telah diberitahu.' : "This isn't your table. A waiter has been notified.") } catch (e: any) { setError(e.message) }
  }

  const total = useMemo(() => cart.reduce((sum, c) => sum + (c.variant.price + c.modifiers.reduce((s: number, id: string) => s + (c.product.modifier_groups?.flatMap((g: Any) => g.modifiers).find((m: Any) => m.id === id)?.price || 0), 0)) * c.qty, 0), [cart])

  return (
    <div className="container">
      <div className="row"><h1 style={{ flex: 1 }}>🍣 Sushi Latte</h1>
        <button onClick={() => setLang(lang === 'id' ? 'en' : 'id')}>{lang === 'id' ? 'EN' : 'ID'}</button>
        {guest && <><button onClick={() => setView('menu')}>Menu</button><button onClick={() => { loadBill(); setView('bill') }}>Bill / Tagihan</button></>}
      </div>
      {error && <div className="error">{error}</div>}
      {notice && <div className="success">{notice}</div>}

      {view === 'scan' && (
        <div className="card">
          <h2>{lang === 'id' ? 'Pindai QR meja Anda' : 'Scan your table QR'}</h2>
          <p className="muted">{lang === 'id' ? 'Tempel token QR dari meja, atau buka menu lewat tautan QR.' : 'Paste the QR token from the table, or open the menu from the QR link.'}</p>
          <label>QR token</label><input value={token} onChange={e => setToken(e.target.value)} placeholder="qr_token" />
          <div className="row" style={{ marginTop: 12 }}><button className="primary" onClick={() => doScan(token)}>{lang === 'id' ? 'Lihat meja' : 'View table'}</button></div>
          {scan && (
            <div style={{ marginTop: 16 }}>
              <p><strong>{scan.table_label || scan.label}</strong> — {scan.status === 'free' ? (lang === 'id' ? 'Meja kosong' : 'Table is free') : (lang === 'id' ? 'Meja terisi' : 'Table is occupied')}</p>
              {scan.status === 'occupied' && scan.customers?.length > 0 && (
                <div><p>{lang === 'id' ? 'Meja ini ditempati oleh:' : 'This table is occupied by:'} {scan.customers.map((c: Any) => c.name).join(', ')}</p>
                  <div className="row">
                    {customerId && <button onClick={rejoin}>{lang === 'id' ? 'Saya salah satunya (gabung lagi)' : "I'm one of these (rejoin)"}</button>}
                    <button onClick={notMine}>{lang === 'id' ? 'Ini bukan meja saya' : "This isn't my table"}</button>
                  </div></div>
              )}
              <h3>{lang === 'id' ? 'Gabung ke meja' : 'Join this table'}</h3>
              <label>{lang === 'id' ? 'Nama' : 'Name'}</label><input value={name} onChange={e => setName(e.target.value)} />
              <label>{lang === 'id' ? 'Catatan diet (opsional)' : 'Dietary note (optional)'}</label><input value={note} onChange={e => setNote(e.target.value)} />
              {catalog?.dietary_tags && <><label>{lang === 'id' ? 'Hindari (opsional)' : 'Avoid (optional)'}</label>
                <div className="row">{catalog.dietary_tags.map((t: Any) => (
                  <label key={t.id} style={{ display: 'inline-flex', gap: 4, alignItems: 'center' }}><input type="checkbox" style={{ width: 'auto' }} checked={restriction.includes(t.id)} onChange={e => setRestriction(e.target.checked ? [...restriction, t.id] : restriction.filter(x => x !== t.id))} /> {nameOf(t)}</label>
                ))}</div></>}
              <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}><input type="checkbox" style={{ width: 'auto' }} checked={consent} onChange={e => setConsent(e.target.checked)} /> {lang === 'id' ? 'Saya setuju menyimpan nama dan data diet saya untuk pesanan ini.' : 'I consent to saving my name and dietary data for this order.'}</label>
              <button className="primary" style={{ marginTop: 12 }} onClick={join} disabled={!name || !consent}>{lang === 'id' ? 'Gabung' : 'Join'}</button>
            </div>
          )}
        </div>
      )}

      {view === 'menu' && catalog && (
        <div>
          <div className="row"><button onClick={waiterCall}>🔔 {lang === 'id' ? 'Panggil pelayan' : 'Call waiter'}</button><span className="muted">{lang === 'id' ? 'Harga belum termasuk pajak dan biaya layanan, yang ditambahkan saat pembayaran.' : 'Prices exclude tax and service, which are added at payment.'}</span></div>
          {catalog.categories?.map((cat: Any) => (
            <div key={cat.id}><h2>{nameOf(cat)}</h2><div className="grid">
              {catalog.products?.filter((p: Any) => p.category_id === cat.id).map((p: Any) => (
                <MenuItem key={p.id} product={p} lang={lang} onAdd={(item: Any) => setCart([...cart, item])} />
              ))}
            </div></div>
          ))}
          <div className="card">
            <h3>{lang === 'id' ? 'Keranjang' : 'Cart'} — {rp(total)}</h3>
            {cart.length === 0 && <p className="muted">{lang === 'id' ? 'Keranjang kosong.' : 'Cart is empty.'}</p>}
            {cart.map((c, i) => <div key={i} className="row"><span style={{ flex: 1 }}>{nameOf(c.product)} — {nameOf(c.variant)} × {c.qty}</span><button onClick={() => setCart(cart.filter((_, j) => j !== i))}>×</button></div>)}
            {warnings && <div className="error">{lang === 'id' ? 'Peringatan diet terdeteksi:' : 'Dietary warnings detected:'} {JSON.stringify(warnings)} <button onClick={() => placeOrder(true)}>{lang === 'id' ? 'Saya mengakui, pesan' : 'I acknowledge, order'}</button></div>}
            <button className="primary" style={{ marginTop: 12 }} disabled={cart.length === 0} onClick={() => placeOrder(false)}>{lang === 'id' ? 'Pesan' : 'Place order'}</button>
          </div>
        </div>
      )}

      {view === 'bill' && (
        <div className="card">
          <h2>{lang === 'id' ? 'Tagihan saya' : 'My bill'}</h2>
          {bill ? <>
            <p>{lang === 'id' ? 'Nomor tagihan' : 'Bill number'}: {bill.bill?.number} — {bill.bill?.status}</p>
            <p>Subtotal {rp(bill.subtotal)} · Service {rp(bill.service_charge)} · Tax {rp(bill.tax)} · <strong>Total {rp(bill.total)}</strong></p>
            {bill.estimated_wait_minutes != null && <p>{lang === 'id' ? 'Perkiraan tunggu' : 'Estimated wait'}: {bill.estimated_wait_minutes} {lang === 'id' ? 'menit' : 'minutes'}</p>}
            <h3>{lang === 'id' ? 'Pesanan saya' : 'My orders'}</h3>
            {orders.map((o: Any) => <div key={o.id} className="card"><p>{o.status} — {o.placed_by} {o.estimated_wait_minutes != null && `· ${o.estimated_wait_minutes} min`}</p>
              {o.items?.map((it: Any) => <p key={it.id}>• {it.product_name_snapshot} × {it.quantity} — {rp(it.line_total)} ({it.status}) {it.dietary_warnings?.length > 0 && <span className="badge red">⚠ {it.dietary_warnings.join(', ')}</span>}</p>)}
              {o.status === 'pending' && <button onClick={async () => { await api(`/public/orders/${o.id}`, { method: 'DELETE', headers: { 'X-Guest-Token': guest } }); loadBill() }}>{lang === 'id' ? 'Batalkan pesanan' : 'Cancel order'}</button>}
            </div>)}
          </> : <p className="muted">{lang === 'id' ? 'Memuat tagihan…' : 'Loading bill…'}</p>}
        </div>
      )}
    </div>
  )
}

function MenuItem({ product, lang, onAdd }: Any) {
  const [variantId, setVariantId] = useState(product.variants?.[0]?.id || '')
  const [mods, setMods] = useState<string[]>([])
  const [qty, setQty] = useState(1)
  const variant = product.variants?.find((v: Any) => v.id === variantId) || product.variants?.[0]
  const nameOf = (o: Any) => lang === 'id' ? o.name_id : o.name_en
  if (!variant) return null
  return (
    <div className="card">
      <strong>{nameOf(product)}</strong>
      <p className="muted">{lang === 'id' ? product.description_id : product.description_en}</p>
      {product.dietary_tag_ids?.map((t: string) => <span key={t} className="badge">{t.replace('tag-', '')}</span>)}
      {product.variants?.length > 1 && <><label>Variant</label><select value={variantId} onChange={e => setVariantId(e.target.value)}>{product.variants.map((v: Any) => <option key={v.id} value={v.id}>{nameOf(v)} — Rp {v.price.toLocaleString('id-ID')} {v.sold_out ? '(Sold Out)' : ''}</option>)}</select></>}
      {product.modifier_groups?.map((g: Any) => (
        <div key={g.id}><label>{nameOf(g)} {g.max_select ? `(max ${g.max_select})` : ''}</label>
          {g.modifiers.map((m: Any) => (
            <label key={m.id} style={{ display: 'flex', gap: 6, alignItems: 'center' }}><input type="checkbox" style={{ width: 'auto' }} checked={mods.includes(m.id)} onChange={e => {
              if (e.target.checked) { if (g.max_select && mods.filter(x => g.modifiers.some((mm: Any) => mm.id === x)).length >= g.max_select) return; setMods([...mods, m.id]) } else setMods(mods.filter(x => x !== m.id))
            }} /> {nameOf(m)} {m.price > 0 ? `+ Rp ${m.price.toLocaleString('id-ID')}` : ''}</label>
          ))}
        </div>
      ))}
      <div className="row" style={{ marginTop: 12 }}>
        <span className="price">Rp {variant.price.toLocaleString('id-ID')}</span>
        <input type="number" min={1} value={qty} onChange={e => setQty(Math.max(1, parseInt(e.target.value) || 1))} style={{ width: 70 }} />
        <button className="primary" disabled={variant.sold_out} onClick={() => onAdd({ product, variant, modifiers: mods, qty })}>{variant.sold_out ? 'Sold Out' : (lang === 'id' ? 'Tambah' : 'Add')}</button>
      </div>
    </div>
  )
}
