import { useEffect, useRef, useState } from 'react'

// Product Display TV app — Phase 3 API.
// Polls /public/display/version every 10 seconds and refetches
// the feed only when the version changes (BR-PD-02), then loops
// slides by their duration_seconds (BR-PD-01). Sold Out
// featured Products are marked (BR-PD-03).

type Any = any
export default function App() {
  const [slides, setSlides] = useState<Any[]>([])
  const [idx, setIdx] = useState(0)
  const [lang, setLang] = useState<'id' | 'en'>('id')
  const versionRef = useRef(0)

  async function fetchVersion() {
    try {
      const v = await (await fetch('/public/display/version')).json()
      if (v.version !== versionRef.current) {
        versionRef.current = v.version
        const feed = await (await fetch('/public/display/slides')).json()
        setSlides(Array.isArray(feed) ? feed : [])
        setIdx(0)
      }
    } catch {}
  }
  useEffect(() => { fetchVersion(); const t = setInterval(fetchVersion, 10000); return () => clearInterval(t) }, [])

  useEffect(() => {
    if (slides.length === 0) return
    const slide = slides[idx % slides.length]
    const ms = (slide.duration_seconds || 10) * 1000
    const t = setTimeout(() => setIdx(i => (i + 1) % slides.length), ms)
    return () => clearTimeout(t)
  }, [slides, idx])

  if (slides.length === 0) {
    return <div className="display"><h1>🍣 Sushi Latte</h1><h2>Selamat datang / Welcome</h2><p className="muted">Menunggu konten display…</p></div>
  }
  const slide = slides[idx % slides.length]
  const headline = lang === 'id' ? slide.headline_id : slide.headline_en
  return (
    <div className="display" onClick={() => setLang(lang === 'id' ? 'en' : 'id')}>
      <p className="muted">{slide.template} · {idx + 1} / {slides.length} · {lang === 'id' ? 'ketuk untuk EN' : 'tap for ID'}</p>
      <h1>{headline || 'Sushi Latte'}</h1>
      {slide.template === 'featured_product' && slide.sold_out && <h2 style={{ color: '#fca5a5' }}>Sold Out / Habis</h2>}
      {slide.template === 'featured_product' && !slide.sold_out && <h2>Featured / Unggulan</h2>}
      {slide.product_id && <p className="muted">Product: {slide.product_id}</p>}
      {slide.category_id && <p className="muted">Category: {slide.category_id}</p>}
      {slide.discount_id && <p className="muted">Discount: {slide.discount_id}</p>}
    </div>
  )
}
