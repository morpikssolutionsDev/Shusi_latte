# Content: Voice, Tone and Formats

## 1. Voice

Sushi Latte sounds like a friendly, competent café crew member: warm with guests, crisp with colleagues.

| | Customer surfaces | Staff surfaces |
|---|---|---|
| Tone | Warm, short, playful | Short, direct, imperative |
| Person | "kamu" / "you" | No pronouns; verbs first |
| Emoji | At most one, only in celebrations and empty states | Never |
| Exclamation marks | Celebrations only | Never |
| Example | "Pesananmu lagi disiapkan ☕" | "Klaim pesanan" · "Meja 5 · 2 item siap" |

## 2. Language

- **Indonesian is the primary language.** Every string is written in ID first, then EN for the Client Menu. Staff surfaces are Indonesian only (EN resource files may be added later).
- Use everyday Indonesian ("pesan", "bayar", "habis"), not formal bureaucratic language ("melakukan pemesanan").
- Glossary terms keep a fixed translation everywhere:

| Glossary | ID (UI) | EN (UI) |
|---|---|---|
| Bill | Tagihan | Bill |
| Order | Pesanan | Order |
| Order Item | Item | Item |
| Claim | Klaim | Claim |
| Pending | Menunggu | Waiting |
| Confirmed | Dikonfirmasi | Confirmed |
| Preparing | Disiapkan | Preparing |
| Ready | Siap | Ready |
| Served | Disajikan | Served |
| Cancel / Cancelled | Batalkan / Dibatalkan | Cancel / Cancelled |
| Void | Void | Void |
| Sold Out | Habis | Sold out |
| Dietary Warning | Peringatan diet | Dietary warning |
| Dietary Restriction | Pantangan makanan | Foods to avoid |
| Settling | Sedang dibayar | Being settled |
| Unpaid Bill | Tagihan tak terbayar | Unpaid bill |
| Remake | Buat ulang | Remake |
| Cash Handover | Setor kas | Cash handover |
| Shift Closing | Tutup shift | Shift closing |
| Estimated Wait | Perkiraan waktu | Estimated wait |
| Takeaway | Bawa pulang | Takeaway |

## 3. Writing rules

1. **Lead with the action or the state.** "Bayar Rp 86.500", not "Klik di sini untuk membayar".
2. **Buttons are verbs** naming the outcome: "Tambah ke pesanan", "Klaim", "Tandai siap", "Tutup tagihan".
3. **Errors say what happened and what to do**, never blame: "Item ini baru saja habis. Pilih menu lain, ya." / "PIN salah. Sisa 2 percobaan."
4. **Destructive confirmations name the object and the consequence**: "Void 1 Salmon Nigiri dari Meja 5? Tindakan ini tercatat di log audit."
5. **Dietary Warnings inform without alarm**: "Mengandung susu — kamu memilih menghindari susu. Tetap pesan?" Buttons: "Tetap pesan" / "Lihat menu lain".
6. **Serious moments are plain**: the Rejoin prompt, Settling notice and "continued on another device" use no emoji or jokes.
7. Keep labels ≤ 3 words on staff buttons; sentences ≤ 15 words on customer screens.
8. Never use ALL CAPS for emphasis; uppercase only for overlines and the REMAKE / REPRINT / INTERNAL markers.

## 4. Key strings

| Moment | ID | EN |
|---|---|---|
| Splash tagline | Sushi & kopi, satu meja. | Sushi & coffee, one table. |
| Name prompt | Halo! Siapa namamu? | Hi! What's your name? |
| Occupied Table | Meja ini sudah ditempati oleh Andi, Budi. Salah satunya kamu? | This table is taken by Andi, Budi. Is one of them you? |
| Order sent | Pesanan terkirim! Pelayan akan mengonfirmasi sebentar lagi. | Order sent! A waiter will confirm it shortly. |
| Waiting | Pesananmu lagi disiapkan ☕ | Your order is being prepared ☕ |
| Ready | Siap! Sebentar lagi diantar. | Ready! Coming to your table. |
| Settling | Tagihan sedang dibayar — panggil pelayan untuk pesan lagi. | Your bill is being settled — ask a waiter to order more. |
| Other device | Kamu melanjutkan di perangkat lain. | You continued on another device. |
| Empty cart | Keranjangmu masih kosong. Yuk, pilih sesuatu! | Your cart is empty. Pick something tasty! |
| Waiter: new Order | Pesanan baru · Meja 5 · 3 item | — |
| Kitchen late | TERLAMBAT | — |
| Clock-out blocked | Setor Rp 250.000 ke kasir dulu. | — |
| Offline | Tidak ada koneksi — gunakan catatan kertas. | — |

## 5. Formats

| Thing | Format | Example |
|---|---|---|
| Currency | `Rp` + space + thousands with `.`, no decimals (id-ID) | Rp 38.000 |
| Negative amount | Minus before Rp | −Rp 5.000 |
| Percent | No space | 10% |
| Time | 24-hour, `.` separator in ID, `:` in EN | 19.45 / 19:45 |
| Timer | `mm:ss`, tabular | 07:42 |
| Date (staff) | `d MMM yyyy` | 26 Okt 2026 |
| Relative time | Short | "2 mnt lalu" / "2 min ago" |
| Bill number | `B-` + 4 digits, daily sequence, mono | B-0427 |
| Table | "Meja" + number | Meja 5 |
| Crew ID | Role letter + 3 digits, mono | W001 |
| Quantity | `×` + number | ×2 |
| Phone-facing names | First name only in headers; full name only in the Rejoin prompt and Receipt | Andi |
