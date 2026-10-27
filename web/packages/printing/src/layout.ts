// 58 mm Receipt layout: 32 characters per line, Font A (TR-PRN-05). The same lines feed the
// ESC/POS encoder and the on-screen ReceiptPreview, so the preview is exactly what prints.

export const WIDTH = 32;

export interface ReceiptData {
  outlet: string;
  copy: 'customer' | 'internal';
  reprint: boolean;
  billNumber: string;
  subBill?: string;
  tables: string[];
  openedAt: number;
  closedAt?: number;
  printedAt: number;
  printedBy: string;
  customers: string[];
  lines: { quantity: number; name: string; variant?: string; modifiers: string[]; amount: number; remake: boolean }[];
  totals: { subtotal: number; itemDiscount: number; billDiscount: number; service: number; tax: number; total: number };
  manualDiscount?: string;
  payments: { method: 'cash' | 'qris' | 'card'; status: string; amount: number; tendered?: number; change: number; rounding: number }[];
  balance: number;
}

/** A printed line: text plus how the printer should style it. */
export interface Line {
  text: string;
  align?: 'left' | 'center';
  bold?: boolean;
  big?: boolean;
}

const rupiah = (n: number) => (n < 0 ? '-' : '') + new Intl.NumberFormat('id-ID').format(Math.abs(n));

const dateTime = (ms: number) => {
  const d = new Date(ms);
  const date = d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Asia/Jakarta' });
  const time = d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta' });
  return `${date} ${time}`;
};

/** Left text and right amount on one 32-column line; long text wraps above the amount. */
export function row(left: string, right: string, width = WIDTH): string[] {
  const room = width - right.length - 1;
  if (left.length <= room) return [left + ' '.repeat(width - left.length - right.length) + right];
  const out = wrap(left, width);
  const last = out.pop()!;
  return [...out, ...row(last, right, width)];
}

export function wrap(text: string, width = WIDTH, indent = ''): string[] {
  const words = text.split(/\s+/).filter(Boolean);
  const out: string[] = [];
  let cur = '';
  for (const w of words) {
    const next = cur ? `${cur} ${w}` : w;
    if (next.length > width && cur) {
      out.push(cur);
      cur = indent + w;
    } else cur = next;
  }
  if (cur) out.push(cur);
  return out.length ? out : [''];
}

const METHOD: Record<string, string> = { cash: 'Tunai', qris: 'QRIS', card: 'Kartu' };
const rule = (): Line => ({ text: '-'.repeat(WIDTH) });

export function receiptLines(r: ReceiptData): Line[] {
  const out: Line[] = [];
  const push = (text: string | string[], style: Omit<Line, 'text'> = {}) => (Array.isArray(text) ? text : [text]).forEach((t) => out.push({ text: t, ...style }));
  push(r.outlet.toUpperCase(), { align: 'center', bold: true, big: true });
  push('Sushi & kopi, satu meja.', { align: 'center' });
  if (r.reprint) push('REPRINT', { align: 'center', bold: true });
  if (r.copy === 'internal') push('INTERNAL', { align: 'center', bold: true });
  out.push(rule());
  push(row(`Tagihan ${r.billNumber}`, r.subBill ? `Bagian ${r.subBill}` : ''));
  push(wrap(r.tables.join(' + ')));
  push(dateTime(r.closedAt ?? r.printedAt));
  if (r.copy === 'customer' && r.customers.length) push(wrap(`Tamu: ${r.customers.join(', ')}`));
  out.push(rule());
  for (const l of r.lines) {
    push(row(`${l.quantity}x ${l.name}`, rupiah(l.amount)));
    const detail = [l.variant, ...l.modifiers].filter(Boolean).join(', ');
    if (detail) push(wrap(detail, WIDTH - 3).map((x) => `   ${x}`));
    if (l.remake) push('   REMAKE', { bold: true });
  }
  out.push(rule());
  const t = r.totals;
  push(row('Subtotal', rupiah(t.subtotal)));
  if (t.itemDiscount) push(row('Diskon item', rupiah(-t.itemDiscount)));
  if (t.billDiscount) push(row(r.manualDiscount ? `Diskon (${r.manualDiscount})` : 'Diskon tagihan', rupiah(-t.billDiscount)));
  push(row('Service', rupiah(t.service)));
  push(row('Pajak PB1', rupiah(t.tax)));
  push(row('TOTAL', `Rp ${rupiah(t.total)}`), { bold: true });
  if (r.payments.length) {
    out.push(rule());
    for (const p of r.payments) {
      push(row(METHOD[p.method] + (p.status === 'pending_verification' ? ' (menunggu verifikasi)' : ''), rupiah(p.amount)));
      if (p.method === 'cash' && p.tendered) {
        push(row('  Diterima', rupiah(p.tendered)));
        push(row('  Kembali', rupiah(p.change)));
      }
      if (p.rounding) push(row('  Pembulatan', (p.rounding > 0 ? '+' : '') + rupiah(p.rounding)));
    }
    if (r.balance > 0) push(row('Sisa', rupiah(r.balance)), { bold: true });
  }
  out.push(rule());
  push(`Dicetak oleh ${r.printedBy}`);
  push(dateTime(r.printedAt));
  push('Terima kasih!', { align: 'center' });
  return out;
}
