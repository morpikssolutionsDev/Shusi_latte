// ESC/POS encoding for 58 mm thermal printers (TR-PRN-01, TR-PRN-05).
import type { Line } from './layout';

const ESC = 0x1b;
const GS = 0x1d;

/** Printers speak a single-byte code page; strip accents (Caffè → Caffe) and drop the rest. */
export function toAscii(text: string): string {
  return text
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[–—]/g, '-')
    .replace(/[^\x20-\x7e]/g, '?');
}

export function encode(lines: Line[], { cut = true }: { cut?: boolean } = {}): Uint8Array {
  const bytes: number[] = [ESC, 0x40]; // initialise
  for (const l of lines) {
    bytes.push(ESC, 0x61, l.align === 'center' ? 1 : 0); // alignment
    bytes.push(ESC, 0x45, l.bold ? 1 : 0); // emphasis
    bytes.push(GS, 0x21, l.big ? 0x11 : 0x00); // double width + height
    for (const ch of toAscii(l.text)) bytes.push(ch.charCodeAt(0));
    bytes.push(0x0a);
  }
  bytes.push(ESC, 0x61, 0, ESC, 0x45, 0, GS, 0x21, 0);
  bytes.push(0x0a, 0x0a, 0x0a);
  if (cut) bytes.push(GS, 0x56, 0x42, 0x00); // partial cut with feed
  return new Uint8Array(bytes);
}
