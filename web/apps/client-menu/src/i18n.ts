// Client Menu strings, Indonesian first, then English (CONTENT.md). Glossary terms keep the
// translations of CONTENT.md §2. Customer voice: warm, short, "kamu"; no emoji outside
// celebrations and empty states; serious moments are plain.
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

const id = {
  common: {
    close: 'Tutup',
    back: 'Kembali',
    retry: 'Coba lagi',
    loading: 'Memuat…',
    language: 'Bahasa',
    table: 'Meja',
  },
  scan: {
    title: 'Pindai QR di mejamu',
    body: 'Arahkan kamera ke kode QR di meja untuk mulai memesan.',
    unavailableTitle: 'QR ini tidak aktif',
    unavailableBody: 'Minta bantuan pelayan, ya. Mereka akan memberi kode QR yang benar.',
  },
  join: {
    title: 'Halo! Siapa namamu?',
    tagline: 'Sushi & kopi, satu meja.',
    name: 'Nama lengkap',
    namePlaceholder: 'mis. Andi Pratama',
    nameHint: 'Dipakai pelayan untuk mengantar pesananmu.',
    restrictionTitle: 'Ada makanan yang kamu hindari?',
    restrictionHint: 'Opsional. Menu yang mengandungnya akan diberi tanda.',
    noteLabel: 'Catatan diet (opsional)',
    notePlaceholder: 'mis. alergi kacang tanah, tanpa bawang',
    consent: 'Saya setuju data pantangan dan catatan ini disimpan untuk kunjungan ini dan dihapus 2 hari setelah tagihan ditutup.',
    consentNeeded: 'Centang persetujuan untuk menyimpan pantangan, atau kosongkan pilihannya.',
    start: 'Mulai pesan',
  },
  occupied: {
    title: 'Meja ini sudah ditempati oleh {{names}}. Salah satunya kamu?',
    imName: 'Saya {{name}}',
    joining: 'Saya baru bergabung',
    notMine: 'Ini bukan meja saya',
    notMineTitle: 'Minta bantuan pelayan, ya',
    notMineBody: 'Pelayan sudah diberi tahu dan akan segera datang.',
  },
  splash: { skip: 'Ketuk untuk lanjut' },
  menu: {
    hello: 'Hai, {{name}}',
    callWaiter: 'Panggil pelayan',
    waiterCalled: 'Pelayan dipanggil',
    waiterCalledToast: 'Pelayan sudah dipanggil dan segera datang.',
    orders: 'Pesananku',
    priceNote: 'Harga belum termasuk pajak {{tax}} dan service {{service}}.',
    priceNoteInclusive: 'Harga sudah termasuk pajak dan service.',
    soldOut: 'Habis',
    from: 'mulai',
    add: 'Tambah {{name}}',
    empty: 'Belum ada menu di kategori ini.',
    settling: 'Tagihan sedang dibayar — panggil pelayan untuk pesan lagi.',
  },
  product: {
    contains: 'Mengandung',
    variant: 'Pilih varian',
    required: 'Pilih {{n}}',
    optional: 'Opsional, maks {{n}}',
    pickExactly: 'Pilih {{n}}',
    note: 'Catatan untuk dapur',
    notePlaceholder: 'mis. tanpa es, saus terpisah',
    quantity: 'Jumlah',
    addToOrder: 'Tambah ke pesanan',
    chooseFirst: 'Lengkapi pilihan {{group}}',
    warningBanner: 'Mengandung {{tags}} — kamu memilih menghindarinya.',
  },
  warning: {
    title: 'Peringatan diet',
    body: 'Mengandung {{tags}} — kamu memilih menghindari {{tags}}. Tetap pesan?',
    keep: 'Tetap pesan',
    other: 'Lihat menu lain',
    badge: 'Mengandung {{tag}}',
  },
  cart: {
    button: 'Lihat pesanan · {{count}} item',
    title: 'Pesananmu',
    empty: 'Keranjangmu masih kosong. Yuk, pilih sesuatu! 🍣',
    subtotal: 'Subtotal',
    discount: 'Diskon',
    total: 'Total item',
    taxNote: 'Pajak & layanan dihitung saat bayar.',
    send: 'Kirim pesanan',
    sending: 'Mengirim…',
    removed: '{{name}} dihapus',
    undo: 'Batalkan',
    problem: 'Perlu dicek',
    remove: 'Hapus',
  },
  sent: {
    title: 'Pesanan terkirim!',
    body: 'Pelayan akan mengonfirmasi sebentar lagi.',
    view: 'Lihat status',
  },
  orders: {
    title: 'Pesananku',
    empty: 'Belum ada pesanan. Pilih menu dulu, ya.',
    order: 'Pesanan #{{n}}',
    waitingWaiter: 'Menunggu pelayan mengonfirmasi',
    claimed: 'Pelayan sedang mengecek pesananmu',
    preparing: 'Pesananmu lagi disiapkan ☕',
    ready: 'Siap! Sebentar lagi diantar.',
    served: 'Selamat menikmati!',
    estimate: 'Perkiraan waktu ± {{n}} mnt',
    edit: 'Ubah pesanan',
    cancel: 'Batalkan pesanan',
    cancelTitle: 'Batalkan pesanan #{{n}}?',
    cancelBody: 'Semua item di pesanan ini dibatalkan.',
    cancelConfirm: 'Ya, batalkan',
    keep: 'Jangan',
    cancelled: 'Pesanan dibatalkan',
    editTitle: 'Ubah pesanan #{{n}}',
    save: 'Simpan perubahan',
    saved: 'Pesanan diperbarui',
    askWaiter: 'Sudah diterima pelayan — minta pelayan jika ingin mengubah.',
    backToMenu: 'Pesan lagi',
    yourBill: 'Tagihan {{number}}',
  },
  status: {
    pending: 'Menunggu',
    claimed: 'Dicek pelayan',
    confirmed: 'Dikonfirmasi',
    preparing: 'Disiapkan',
    ready: 'Siap',
    served: 'Disajikan',
    cancelled: 'Dibatalkan',
    voided: 'Void',
  },
  ended: {
    elsewhereTitle: 'Kamu melanjutkan di perangkat lain.',
    elsewhereBody: 'Pesananmu aman. Pindai QR meja lagi jika ingin memakai perangkat ini.',
    closedTitle: 'Tagihan ini sudah ditutup',
    closedBody: 'Terima kasih sudah mampir! Pindai QR meja untuk pesan lagi.',
    scanAgain: 'Mulai lagi',
  },
  errors: {
    generic: 'Ada gangguan. Coba lagi, ya.',
    offline: 'Koneksi terputus. Menyambung ulang…',
  },
  demo: {
    banner: 'Demo — data direset setiap malam',
    launcher: 'Peluncur demo',
    title: 'Peluncur demo Sushi Latte',
    body: 'Buka meja sebagai pelanggan (tiap tab = satu pelanggan), atau masuk ke aplikasi staf.',
    openAsCustomer: 'Buka sebagai pelanggan',
    staff: 'Aplikasi staf',
    display: 'Product Display',
    inactive: 'Nonaktif',
  },
};

type Strings = typeof id;

const en: Strings = {
  common: { close: 'Close', back: 'Back', retry: 'Try again', loading: 'Loading…', language: 'Language', table: 'Table' },
  scan: {
    title: 'Scan the QR on your table',
    body: 'Point your camera at the QR code on the table to start ordering.',
    unavailableTitle: 'This QR is not active',
    unavailableBody: 'Please ask a waiter — they will give you the right code.',
  },
  join: {
    title: "Hi! What's your name?",
    tagline: 'Sushi & coffee, one table.',
    name: 'Full name',
    namePlaceholder: 'e.g. Andi Pratama',
    nameHint: 'Waiters use it to bring your order.',
    restrictionTitle: 'Anything you avoid?',
    restrictionHint: 'Optional. Dishes that contain it get a badge.',
    noteLabel: 'Dietary note (optional)',
    notePlaceholder: 'e.g. peanut allergy, no onion',
    consent: 'I agree to this dietary information being stored for this visit and deleted 2 days after the bill closes.',
    consentNeeded: 'Tick the consent to save these, or clear your choices.',
    start: 'Start ordering',
  },
  occupied: {
    title: 'This table is taken by {{names}}. Is one of them you?',
    imName: "I'm {{name}}",
    joining: "I'm joining this table",
    notMine: "This isn't my table",
    notMineTitle: 'Please ask a waiter',
    notMineBody: "We've let the waiters know; someone will be right with you.",
  },
  splash: { skip: 'Tap to continue' },
  menu: {
    hello: 'Hi, {{name}}',
    callWaiter: 'Call waiter',
    waiterCalled: 'Waiter called',
    waiterCalledToast: 'A waiter is on the way.',
    orders: 'My orders',
    priceNote: 'Prices exclude {{tax}} tax and {{service}} service.',
    priceNoteInclusive: 'Prices include tax and service.',
    soldOut: 'Sold out',
    from: 'from',
    add: 'Add {{name}}',
    empty: 'Nothing in this category yet.',
    settling: 'Your bill is being settled — ask a waiter to order more.',
  },
  product: {
    contains: 'Contains',
    variant: 'Choose a variant',
    required: 'Choose {{n}}',
    optional: 'Optional, up to {{n}}',
    pickExactly: 'Choose {{n}}',
    note: 'Note for the kitchen',
    notePlaceholder: 'e.g. no ice, sauce on the side',
    quantity: 'Quantity',
    addToOrder: 'Add to order',
    chooseFirst: 'Complete {{group}}',
    warningBanner: 'Contains {{tags}} — you chose to avoid it.',
  },
  warning: {
    title: 'Dietary warning',
    body: 'Contains {{tags}} — you chose to avoid {{tags}}. Order anyway?',
    keep: 'Order anyway',
    other: 'See other dishes',
    badge: 'Contains {{tag}}',
  },
  cart: {
    button: 'View order · {{count}} items',
    title: 'Your order',
    empty: 'Your cart is empty. Pick something tasty! 🍣',
    subtotal: 'Subtotal',
    discount: 'Discount',
    total: 'Items total',
    taxNote: 'Tax & service are added when you pay.',
    send: 'Send order',
    sending: 'Sending…',
    removed: '{{name}} removed',
    undo: 'Undo',
    problem: 'Needs a look',
    remove: 'Remove',
  },
  sent: { title: 'Order sent!', body: 'A waiter will confirm it shortly.', view: 'See status' },
  orders: {
    title: 'My orders',
    empty: 'No orders yet. Pick something from the menu.',
    order: 'Order #{{n}}',
    waitingWaiter: 'Waiting for a waiter to confirm',
    claimed: 'A waiter is checking your order',
    preparing: 'Your order is being prepared ☕',
    ready: 'Ready! Coming to your table.',
    served: 'Enjoy!',
    estimate: 'Estimated wait ± {{n}} min',
    edit: 'Edit order',
    cancel: 'Cancel order',
    cancelTitle: 'Cancel order #{{n}}?',
    cancelBody: 'Every item in this order will be cancelled.',
    cancelConfirm: 'Yes, cancel',
    keep: 'Keep it',
    cancelled: 'Order cancelled',
    editTitle: 'Edit order #{{n}}',
    save: 'Save changes',
    saved: 'Order updated',
    askWaiter: 'A waiter has it now — ask them for changes.',
    backToMenu: 'Order more',
    yourBill: 'Bill {{number}}',
  },
  status: {
    pending: 'Waiting',
    claimed: 'Being checked',
    confirmed: 'Confirmed',
    preparing: 'Preparing',
    ready: 'Ready',
    served: 'Served',
    cancelled: 'Cancelled',
    voided: 'Voided',
  },
  ended: {
    elsewhereTitle: 'You continued on another device.',
    elsewhereBody: 'Your orders are safe. Scan the table QR again to use this device.',
    closedTitle: 'This bill is closed',
    closedBody: 'Thanks for coming! Scan the table QR to order again.',
    scanAgain: 'Start again',
  },
  errors: { generic: 'Something went wrong. Please try again.', offline: 'Connection lost. Reconnecting…' },
  demo: {
    banner: 'Demo — resets nightly',
    launcher: 'Demo launcher',
    title: 'Sushi Latte demo launcher',
    body: 'Open a table as a customer (each tab is one customer), or sign in to the staff app.',
    openAsCustomer: 'Open as customer',
    staff: 'Staff app',
    display: 'Product Display',
    inactive: 'Inactive',
  },
};

export type Lang = 'id' | 'en';
const LANG_KEY = 'sl.lang';

/** Phone language with a manual toggle, defaulting to Indonesian (TR-I18N-01). */
function initialLang(): Lang {
  try {
    const saved = localStorage.getItem(LANG_KEY);
    if (saved === 'id' || saved === 'en') return saved;
  } catch {
    // storage blocked: fall through to the phone language
  }
  return navigator.language.toLowerCase().startsWith('en') ? 'en' : 'id';
}

export function setLang(lang: Lang) {
  try {
    localStorage.setItem(LANG_KEY, lang);
  } catch {
    // not persisted; the toggle still applies to this page
  }
  document.documentElement.lang = lang;
  void i18n.changeLanguage(lang);
}

void i18n.use(initReactI18next).init({
  resources: { id: { translation: id }, en: { translation: en } },
  lng: initialLang(),
  fallbackLng: 'id',
  interpolation: { escapeValue: false },
});
document.documentElement.lang = i18n.language;

export default i18n;
