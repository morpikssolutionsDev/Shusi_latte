// TR-PRN-05: 32-column layout, Customer vs Internal copy, REPRINT marker; ESC/POS bytes.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { encode, toAscii } from '../src/escpos.ts';
import { WIDTH, receiptLines, row, type ReceiptData } from '../src/layout.ts';

const receipt: ReceiptData = {
  outlet: 'Sushi Latte',
  copy: 'customer',
  reprint: false,
  billNumber: 'B-0005',
  subBill: '1/2',
  tables: ['Meja 8', 'Meja 9'],
  openedAt: Date.UTC(2026, 9, 27, 5, 0),
  closedAt: Date.UTC(2026, 9, 27, 7, 5),
  printedAt: Date.UTC(2026, 9, 27, 7, 6),
  printedBy: 'Maya Anggraini',
  customers: ['Gilang', 'Nabila'],
  lines: [
    { quantity: 1, name: 'Signature Caffè Latte', variant: 'Panas', modifiers: ['Susu segar'], amount: 38000, remake: false },
    { quantity: 2, name: 'Sashimi Tuna & Salmon Premium Platter Besar', modifiers: [], amount: 390000, remake: false },
    { quantity: 1, name: 'Spicy Salmon Roll', modifiers: [], amount: 0, remake: true },
  ],
  totals: { subtotal: 428000, itemDiscount: 3800, billDiscount: 25000, service: 19960, tax: 41916, total: 460076 },
  payments: [{ method: 'cash', status: 'held_by_waiter', amount: 460100, tendered: 500000, change: 39900, rounding: 24 }],
  balance: 0,
};

test('every line fits 32 columns', () => {
  for (const l of receiptLines(receipt)) assert.ok(l.text.length <= WIDTH || l.big, `too wide: "${l.text}"`);
});

test('amounts are right-aligned on the last column', () => {
  assert.deepEqual(row('Subtotal', '428.000'), ['Subtotal' + ' '.repeat(WIDTH - 8 - 7) + '428.000']);
  const wrapped = row('2x Sashimi Tuna & Salmon Premium Platter Besar', '390.000');
  assert.ok(wrapped.length === 2 && wrapped[1]!.endsWith('390.000') && wrapped[1]!.length === WIDTH);
});

test('Customer copy names the Customers; Internal copy does not and is marked', () => {
  const customer = receiptLines(receipt).map((l) => l.text);
  assert.ok(customer.some((t) => t.includes('Gilang, Nabila')));
  assert.ok(!customer.includes('INTERNAL'));
  const internal = receiptLines({ ...receipt, copy: 'internal', customers: [] }).map((l) => l.text);
  assert.ok(internal.includes('INTERNAL'));
  assert.ok(!internal.some((t) => t.includes('Gilang')));
});

test('a reprint says REPRINT under the header (TR-PRN-08)', () => {
  const lines = receiptLines({ ...receipt, reprint: true }).map((l) => l.text);
  assert.equal(lines[2], 'REPRINT');
});

test('cash shows tendered, change and rounding; Remake is marked', () => {
  const text = receiptLines(receipt).map((l) => l.text).join('\n');
  assert.match(text, /Diterima\s+500\.000/);
  assert.match(text, /Kembali\s+39\.900/);
  assert.match(text, /Pembulatan\s+\+24/);
  assert.match(text, /REMAKE/);
  assert.match(text, /TOTAL\s+Rp 460\.076/);
});

test('ESC/POS: init, ASCII-only text, cut', () => {
  assert.equal(toAscii('Caffè – crêpe'), 'Caffe - crepe');
  const bytes = encode(receiptLines(receipt));
  assert.deepEqual([...bytes.slice(0, 2)], [0x1b, 0x40]);
  assert.deepEqual([...bytes.slice(-4)], [0x1d, 0x56, 0x42, 0x00]);
  assert.ok(bytes.every((b) => b < 0x80));
});
