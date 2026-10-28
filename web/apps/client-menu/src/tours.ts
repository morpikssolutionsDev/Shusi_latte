// Guided demo tours (DEMO-AND-SEED §10): numbered steps and the screens to open side by side.
// A presenter runs all six back to back after one reset (Phase 8 done-when).

type Text = { id: string; en: string };
export type Screen = { label: Text; to: string | { table: number } };
export type Tour = { title: Text; flows: Text; screens: Screen[]; steps: Text[] };

const staff = (path: string) => `staff:${path}`;

export const TOURS: Tour[] = [
  {
    title: { id: 'Dari scan sampai ke meja', en: 'From scan to table' },
    flows: { id: 'Scan, pantangan + peringatan, pesan, klaim, dapur, cup-fill, disajikan', en: 'Scan, restriction + warning, order, Claim, Kitchen, cup-fill, Served' },
    screens: [
      { label: { id: 'HP · Meja 1', en: 'Phone · Meja 1' }, to: { table: 1 } },
      { label: { id: 'Pelayan (W001)', en: 'Waiter (W001)' }, to: staff('/waiter') },
      { label: { id: 'Layar Bar (A001 → mode layar)', en: 'Bar screen (A001 → screen mode)' }, to: staff('/kitchen/bar') },
    ],
    steps: [
      { id: 'Buka Meja 1 sebagai tamu, isi nama, centang pantangan Susu dan setujui.', en: 'Open Meja 1 as a Customer, enter a name, tick the Dairy restriction and consent.' },
      { id: 'Tambahkan Signature Caffè Latte: peringatan diet muncul. Konfirmasi lalu kirim pesanan.', en: 'Add a Signature Caffè Latte: the Dietary Warning shows. Acknowledge it and send the Order.' },
      { id: 'Di Pelayan (W001) notifikasi berbunyi: Klaim, periksa peringatan, Konfirmasi.', en: 'On the Waiter (W001) the notification chimes: Claim, review the warning, Confirm.' },
      { id: 'Di layar Bar: Mulai, lalu Siap. Lihat cangkir di HP terisi sampai "Siap!".', en: 'On the Bar screen: Start, then Ready. Watch the cup on the phone fill to "Siap!".' },
      { id: 'Pelayan menandai Disajikan.', en: 'The Waiter marks it Served.' },
    ],
  },
  {
    title: { id: 'Bahan habis', en: 'Running out' },
    flows: { id: 'Batal karena habis → Habis di semua layar → tanda di Meja 5 → diaktifkan lagi', en: 'Out of Stock Cancel → Sold Out everywhere → flags on Meja 5 → re-enabled' },
    screens: [
      { label: { id: 'Layar Dapur', en: 'Kitchen screen' }, to: staff('/kitchen/kitchen') },
      { label: { id: 'Pelayan (W001)', en: 'Waiter (W001)' }, to: staff('/waiter') },
      { label: { id: 'HP · Meja 6', en: 'Phone · Meja 6' }, to: { table: 6 } },
    ],
    steps: [
      { id: 'Di layar Dapur, Batalkan Tonkotsu Ramen Meja 6 dengan alasan Habis.', en: 'On the Kitchen screen, Cancel Meja 6’s Tonkotsu Ramen with reason Out of Stock.' },
      { id: 'Dalam satu detik varian itu abu-abu di semua menu yang terbuka, termasuk HP Meja 6.', en: 'Within a second the Variant greys out on every open menu, including the Meja 6 phone.' },
      { id: 'Pesanan terbuka lain dengan varian itu ditandai untuk pelayan (lihat juga Meja 5: Hojicha habis, Sakura disembunyikan).', en: 'Other open Orders with it are flagged for the Waiter (see Meja 5: Hojicha Sold Out, Sakura hidden).' },
      { id: 'Masuk sebagai A001 → Admin → Habis, aktifkan lagi dan lihat siapa yang mematikannya.', en: 'Sign in as A001 → Admin → Sold Out, re-enable it and see who turned it off and why.' },
    ],
  },
  {
    title: { id: 'Membayar', en: 'Settling up' },
    flows: { id: 'Sedang dibayar, split per item, pembulatan tunai, QRIS, dua struk, tagihan tak terbayar', en: 'Settling lock, split by item, cash rounding, QRIS, two Receipts, Unpaid Bill' },
    screens: [
      { label: { id: 'Kasir (C001)', en: 'Cashier (C001)' }, to: staff('/cashier') },
      { label: { id: 'Pelayan (W002)', en: 'Waiter (W002)' }, to: staff('/waiter/tables') },
      { label: { id: 'HP · Meja 8', en: 'Phone · Meja 8' }, to: { table: 8 } },
    ],
    steps: [
      { id: 'Meja 8 + 9 sedang dibayar: HP Meja 8 menampilkan "Tagihan sedang dibayar" dan tidak bisa memesan.', en: 'Meja 8 + 9 is Settling: the Meja 8 phone shows the Bill is being paid and cannot order.' },
      { id: 'Di Kasir buka Meja 10: Intan sudah lunas; bayar sub-tagihan Fikri tunai — pembulatan Rp 100 dan kembalian.', en: 'In the Cashier open Meja 10: Intan has paid; pay Fikri’s Sub-bill in cash — Rp 100 rounding and change.' },
      { id: 'Struk muncul sebagai pratinjau: salinan tamu dan internal.', en: 'The Receipt shows as a preview: Customer and internal copies.' },
      { id: 'Di Meja 8 bayar sisanya dengan QRIS; split ditolak karena sudah ada pembayaran.', en: 'On Meja 8 pay the rest by QRIS; splitting is refused once a Payment exists.' },
      { id: 'Tutup tagihan lain sebagai Tak terbayar dengan PIN A001.', en: 'Close another Bill as Unpaid with A001’s PIN.' },
    ],
  },
  {
    title: { id: 'Saat ada yang salah', en: 'When things go wrong' },
    flows: { id: 'Void dengan PIN, Remake, Refund dalam / di atas batas, log audit', en: 'Void with PIN, Remake, Refund within / above cap, audit log' },
    screens: [
      { label: { id: 'Pelayan (W001)', en: 'Waiter (W001)' }, to: staff('/waiter/tables') },
      { label: { id: 'Kasir (C001)', en: 'Cashier (C001)' }, to: staff('/cashier') },
      { label: { id: 'Admin (M001)', en: 'Admin (M001)' }, to: staff('/admin/audit') },
    ],
    steps: [
      { id: 'Meja 7: Spicy Salmon sudah di-Remake (A001) — di dapur tiket REMAKE paling depan.', en: 'Meja 7: the Spicy Salmon was Remade (A001) — the REMAKE ticket leads the Kitchen queue.' },
      { id: 'Void satu item yang sudah siap dengan PIN A001; Meja 10 menunjukkan cheesecake yang sudah di-Void.', en: 'Void a Ready item with A001’s PIN; Meja 10 shows the cheesecake already Voided.' },
      { id: 'Di Kasir buka tagihan yang sudah ditutup, Refund Rp 15.000 (A001), lalu di atas Rp 500.000 (perlu M001).', en: 'In the Cashier open a closed Bill, Refund Rp 15,000 (A001), then above Rp 500,000 (needs M001).' },
      { id: 'Admin → Log audit: setiap tindakan dengan penyetuju dan alasannya.', en: 'Admin → Audit log: every action with its approver and reason.' },
    ],
  },
  {
    title: { id: 'Menutup hari', en: 'Closing the day' },
    flows: { id: 'Setoran kas dengan selisih, QRIS tertunda, tutup shift, penghalang absen pulang, koreksi', en: 'Cash Handover with difference, pending QRIS, Shift Closing, clock-out blockers, auto clock-out correction' },
    screens: [
      { label: { id: 'Kasir · Laci kas (C001)', en: 'Cashier · Drawer (C001)' }, to: staff('/cashier/drawer') },
      { label: { id: 'Tablet absensi (A001 → mode layar)', en: 'HRIS tablet (A001 → screen mode)' }, to: staff('/clock') },
      { label: { id: 'HRIS · Kehadiran (A001)', en: 'HRIS · Attendance (A001)' }, to: staff('/hris/attendance') },
    ],
    steps: [
      { id: 'Di tablet absensi, W002 mencoba pulang: "Setor Rp 200.000 ke kasir dulu."', en: 'On the HRIS tablet, W002 tries to clock out: "Hand over Rp 200,000 to the Cashier first."' },
      { id: 'Di Laci kas terima setoran W002; selisih dicatat atas namanya. Lihat juga W003 yang kurang Rp 20.000.', en: 'In the Drawer receive W002’s handover; any difference is recorded against them. See W003 Rp 20,000 short too.' },
      { id: 'Verifikasi atau hapuskan QRIS Rp 94.300, lalu Tutup laci dengan uang dihitung.', en: 'Verify or write off the pending QRIS Rp 94,300, then close the drawer with the counted cash.' },
      { id: 'W002 kini bisa pulang; W001 diminta menyerahkan klaim Meja 5 dan 7.', en: 'W002 can now clock out; W001 is asked to hand over the Meja 5 and 7 Claims.' },
      { id: 'HRIS → Kehadiran, kemarin: koreksi absen pulang otomatis K002 dengan alasan.', en: 'HRIS → Attendance, yesterday: correct K002’s auto clock-out with a reason.' },
    ],
  },
  {
    title: { id: 'Meja sama, HP baru', en: 'Same table, new phone' },
    flows: { id: 'Gabung lagi sebagai Raka → tab lama "dilanjutkan di perangkat lain"; tamu baru', en: 'Rejoin as Raka → old tab shows "continued on another device"; a new Customer joins' },
    screens: [
      { label: { id: 'HP 1 · Meja 3', en: 'Phone 1 · Meja 3' }, to: { table: 3 } },
      { label: { id: 'HP 2 · Meja 3', en: 'Phone 2 · Meja 3' }, to: { table: 3 } },
    ],
    steps: [
      { id: 'Di HP 1 pilih "Saya Raka".', en: 'On phone 1 choose “I’m Raka”.' },
      { id: 'Di HP 2 pilih "Saya Raka" juga: HP 1 menampilkan "dilanjutkan di perangkat lain".', en: 'On phone 2 choose “I’m Raka” too: phone 1 shows “continued on another device”.' },
      { id: 'Di HP 1 gabung sebagai tamu baru: tagihan yang sama, pesanan sendiri.', en: 'On phone 1 join as a new Customer: the same Bill, their own Orders.' },
    ],
  },
];

/** Where a screen link goes: staff paths open the Staff app, Tables a fresh Customer tab. */
export function screenUrl(to: Screen['to'], codeOf: (n: number) => string | undefined, staffBase: string) {
  if (typeof to === 'object') {
    const code = codeOf(to.table);
    return code ? `/${code}?fresh=1` : undefined;
  }
  return `${staffBase.replace(/\/$/, '')}${to.slice('staff:'.length)}`;
}
