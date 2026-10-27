// Cashier end-to-end (Phase 6, OP-11..OP-14) on the dev stack + fresh seed, landscape 1280×800.
import { chromium } from '@playwright/test';

const out = process.argv[2];
const origin = 'http://localhost:5174';
const staffBase = `${origin}/staff`;
const browser = await chromium.launch({ channel: 'chrome' });
const errors = [];
const results = [];
const check = (name, ok, extra = '') => results.push(`${ok ? 'PASS' : 'FAIL'} ${name}${extra ? ` — ${extra}` : ''}`);
const shot = (page, name) => page.screenshot({ path: `${out}/c_${name}.png` });
const rupiah = (s) => Number(String(s).replace(/[^\d]/g, ''));

async function login(crewId, pin, viewport = { width: 1280, height: 800 }) {
  const ctx = await browser.newContext({ viewport, deviceScaleFactor: 1, locale: 'id-ID' });
  const page = await ctx.newPage();
  page.on('pageerror', (e) => errors.push(`${crewId} pageerror: ${e.message}`));
  page.on('response', async (r) => {
    if (r.url().includes('/api/') && r.status() >= 500) errors.push(`${crewId} ${r.request().method()} ${new URL(r.url()).pathname} → ${r.status()}`);
  });
  await page.goto(`${staffBase}/login`);
  await page.getByLabel('Crew ID').fill(crewId);
  for (const d of pin) await page.getByRole('button', { name: d, exact: true }).click();
  await page.waitForURL(`${staffBase}/`);
  return { ctx, page };
}
async function approve(page, crewId, pin) {
  await page.getByLabel('Crew ID penyetuju').fill(crewId);
  for (const d of pin) await page.getByRole('button', { name: d, exact: true }).last().click();
}

const c = await login('C001', '444444');
const api = c.ctx.request;
const get = async (path) => (await api.get(`${origin}/api/v1${path}`)).json();
const admin = await login('A001', '222222');
const barHas = async (name) => {
  const q = await (await admin.ctx.request.get(`${origin}/api/v1/prep-areas/bar/queue`)).json();
  return [...q.tickets, ...q.ready].some((k) => k.tables[0] === `Bawa pulang · ${name}`);
};

try {
// ---------------------------------------------------------------- module and split view
await c.page.getByRole('link', { name: 'Kasir' }).click();
await c.page.waitForURL(`${staffBase}/cashier`);
await c.page.getByText('Berjalan ·').waitFor();
await c.page.waitForTimeout(400);
await shot(c.page, '01_bills');
const bills = await get('/cashier/bills');
check('Cashier lists open and today\'s closed Bills (LAYOUT.md §3)', bills.some((b) => b.status === 'open') && bills.filter((b) => b.status === 'closed').length >= 15,
  `${bills.length} Bills`);
const pendingBadge = await c.page.getByRole('link', { name: /Laci kas/ }).textContent();
check('Laci kas tab shows the pending QRIS count', pendingBadge.includes('1'));

// TA-3 Joko: held, no Confirm, not in the Bar.
await c.page.locator('[data-bill]').filter({ hasText: 'Bawa pulang · Joko' }).click();
await c.page.getByText('Menunggu pembayaran — belum dikirim ke dapur').waitFor();
check('an unpaid Takeaway is held: no Confirm, no new Order (TR-PAY-04)',
  (await c.page.getByRole('button', { name: 'Konfirmasi' }).count()) === 0 && (await c.page.getByRole('link', { name: 'Pesanan baru' }).count()) === 0 && !(await barHas('Joko')));
await shot(c.page, '02_takeaway_held');

// ---------------------------------------------------------------- new Takeaway, paid cash, released
await c.page.getByRole('link', { name: 'Bawa pulang baru' }).click();
await c.page.getByLabel('Nama untuk dipanggil').fill('Dina');
await c.page.getByLabel('Cari menu').fill('Cold Brew');
await c.page.locator('[data-product="Kyoto Cold Brew"]').click();
await c.page.getByRole('button', { name: 'Buat tagihan' }).click();
await c.page.waitForURL(/\/cashier\?bill=/);
const dinaId = new URL(c.page.url()).searchParams.get('bill');
check('Takeaway created and opened in the split view', !!dinaId && !(await barHas('Dina')));
await c.page.getByRole('button', { name: 'Bayar', exact: true }).click();
await c.page.getByRole('button', { name: 'Uang pas' }).click();
await c.page.getByRole('button', { name: /Catat pembayaran/ }).click();
await c.page.getByText('Lunas. Pesanan sudah dikirim ke dapur.').waitFor({ timeout: 5000 });
await c.page.waitForTimeout(300);
await shot(c.page, '03_takeaway_paid');
const dina = await get(`/bills/${dinaId}`);
check('paid in full: the Takeaway reaches the Bar and closes (BR-CS-02, G-18)', (await barHas('Dina')) && dina.status === 'closed' && dina.billing.payments[0].status === 'settled');
await c.page.getByRole('button', { name: 'Cetak struk' }).click();
await c.page.getByTestId('receipt').waitFor({ timeout: 8000 });
check('receipt follows the Takeaway payment', (await c.page.getByTestId('receipt').textContent()).includes('TOTAL'));
await c.page.keyboard.press('Escape');

// ---------------------------------------------------------------- drawer: handover, QRIS, Shift Closing
await c.page.getByRole('link', { name: /Laci kas/ }).click();
await c.page.getByTestId('shift-closing').waitFor();
await c.page.waitForTimeout(300);
await shot(c.page, '04_drawer');
const closeBtn = c.page.getByRole('button', { name: 'Tutup laci' });
check('Shift Closing refused while a QRIS is pending (BR-CS-07, TR-PAY-03)', await closeBtn.isDisabled() && (await c.page.getByText('Selesaikan 1 QRIS dulu.').count()) === 1);

await c.page.locator('[data-held="W002"]').getByRole('button', { name: 'Terima setoran' }).click();
await c.page.getByLabel('Diterima').fill('190000');
const diffText = await c.page.getByRole('dialog').getByRole('status').textContent();
check('CashHandoverForm shows the difference', diffText.includes('10.000') && diffText.includes('−'), diffText);
await c.page.getByRole('dialog').getByRole('button', { name: 'Terima setoran' }).click();
await c.page.getByText(/Selisih −Rp\s?10\.000 dicatat atas nama Dimas Saputra/).waitFor({ timeout: 5000 });
let drawer = await get('/drawer');
const w002 = drawer.handovers.find((h) => h.waiter === 'Dimas Saputra');
check('Cash Handover records the difference against the Waiter (BR-CS-05, BR-R-36)', w002?.expected === 200000 && w002.difference === -10000 && drawer.held.length === 0);

await c.page.getByTestId('pending-qris').getByRole('button', { name: 'Sudah masuk' }).click();
await c.page.getByText('Tidak ada QRIS yang menunggu.').waitFor({ timeout: 5000 });
drawer = await get('/drawer');
const e = drawer.expected;
check('expected cash = Float + handovers + counter cash − change − cash Refunds (TR-PAY-01)',
  e.total === e.openingFloat + e.handovers + e.counterTendered - e.change - e.refunds && rupiah(await c.page.getByTestId('expected-total').textContent()) === e.total,
  `total ${e.total}`);

// ---------------------------------------------------------------- Refund within and above the cap
await c.page.getByRole('link', { name: 'Tagihan' }).click();
const closedTable = bills.find((b) => b.status === 'closed' && b.tables[0]?.label === 'Meja 4');
await c.page.locator(`[data-bill="${closedTable.number}"]`).click();
await c.page.getByRole('button', { name: 'Refund' }).click();
await c.page.getByLabel('Jumlah').fill('15000');
await c.page.getByLabel('Alasan').fill('Minuman tumpah');
await c.page.getByRole('button', { name: /Refund · Rp/ }).click();
await approve(c.page, 'A001', '222222');
await c.page.getByText('Refund dicatat').first().waitFor({ timeout: 5000 });
const refunded = await get(`/bills/${closedTable.id}`);
check('Refund within the cap with Outlet Admin PIN (BR-R-20)', refunded.refunded === 15000);
await shot(c.page, '05_refund');

await c.page.getByRole('button', { name: 'Refund' }).click();
const capHint = c.page.getByText('Di atas batas — perlu PIN Management.');
await c.page.getByLabel('Jumlah').fill(String(refunded.billing.paid - 15000));
const aboveCap = refunded.billing.paid - 15000 > 500000 ? await capHint.count() === 1 : await capHint.count() === 0;
check('the Refund dialog names the approver the cap requires', aboveCap);
await c.page.keyboard.press('Escape');

// ---------------------------------------------------------------- Post-outage Entry
await c.page.getByRole('link', { name: 'Pasca-gangguan' }).click();
await c.page.getByLabel('Meja', { exact: true }).selectOption({ label: 'Meja 1' });
await c.page.getByLabel('Dilayani oleh').fill('w001');
await c.page.getByLabel('Tambah item').selectOption({ label: 'Kyoto Cold Brew' });
await c.page.getByLabel('Jumlah').first().fill('1000');
await c.page.getByRole('button', { name: 'Simpan tagihan' }).click();
await c.page.getByText('Total menurut sistem').waitFor({ timeout: 5000 });
const total = rupiah(await c.page.getByRole('status').filter({ hasText: 'Total menurut sistem' }).textContent());
await c.page.getByLabel('Jumlah').first().fill(String(total));
await shot(c.page, '06_outage');
await c.page.getByRole('button', { name: 'Simpan tagihan' }).click();
await c.page.waitForURL(/\/cashier\?bill=/);
const outage = await get(`/bills/${new URL(c.page.url()).searchParams.get('bill')}`);
check('Post-outage Entry: closed, marked, never in the Kitchen (BR-CS-08, G-21)',
  outage.source === 'post_outage' && outage.status === 'closed' && outage.orders[0].items.every((i) => i.status === 'served') && total > 0, `total ${total}`);

// ---------------------------------------------------------------- Shift Closing
await c.page.getByRole('link', { name: /Laci kas/ }).click();
const expected = rupiah(await c.page.getByTestId('expected-total').textContent());
await c.page.getByLabel('Uang dihitung').fill(String(expected - 1000));
await c.page.getByRole('button', { name: 'Tutup laci' }).click();
await c.page.getByRole('status').filter({ hasText: 'Shift ditutup' }).waitFor({ timeout: 5000 });
await c.page.waitForTimeout(300);
await shot(c.page, '07_closed');
check('Shift Closing records counted cash and the difference (BR-CS-06)', (await c.page.getByText(/selisih −Rp\s?1\.000/).count()) === 1 && (await get('/drawer')).session === undefined);
check('the header warns the drawer is closed', (await c.page.getByText('Laci belum dibuka').count()) >= 1);
await c.page.getByLabel('Opening Float').fill('300000');
await c.page.getByRole('button', { name: 'Buka laci' }).click();
await c.page.getByTestId('shift-closing').waitFor({ timeout: 5000 });
check('a new drawer session opens with its own Opening Float (BR-CS-04)', (await get('/drawer')).session?.openingFloat === 300000);

// ---------------------------------------------------------------- Operations report
const m = await login('M001', '111111', { width: 1366, height: 900 });
await m.page.goto(`${staffBase}/admin/reports`);
await m.page.getByTestId('operations').waitFor({ timeout: 8000 });
await m.page.getByTestId('operations').scrollIntoViewIfNeeded();
await m.page.waitForTimeout(400);
await shot(m.page, '08_operations');
const date = await m.page.locator('#report-date').inputValue();
const ops = await (await m.ctx.request.get(`${origin}/api/v1/reports/operations?date=${date}`)).json();
check('Operations report: Shift Closings, Unpaid, Remakes, Refunds (BR-ADM-11)',
  ops.shiftClosings.length === 2 && ops.unpaid.length === 1 && ops.refunds.length === 3 && ops.remakes.length >= 1,
  `shifts ${ops.shiftClosings.length} unpaid ${ops.unpaid.length} refunds ${ops.refunds.length} remakes ${ops.remakes.length} outage ${ops.postOutage.length}`);

} catch (e) {
  await shot(c.page, 'fail');
  results.push(`FAIL (aborted) ${e.message.split('\n')[0]}`);
}
await browser.close();
console.log(results.join('\n'));
if (errors.length) console.log([...new Set(errors)].join('\n'));
