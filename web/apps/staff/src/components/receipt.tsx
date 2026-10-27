// ReceiptPreview (COMPONENTS.md §3, TRD §10). Both copies are fetched first and held on the
// device, so printing or retrying never needs the network again (TR-PRN-04). Without a paired
// printer (the demo), the preview is the Receipt (DEMO-AND-SEED §2).
import { api, unwrap, type Schemas } from '@sushi-latte/api-client';
import { bluetoothSupported, connectBluetooth, encode, receiptLines, serialSupported, connectSerial, type Printer, type ReceiptData } from '@sushi-latte/printing';
import { Button, Dialog, cn } from '@sushi-latte/ui';
import { BluetoothIcon, PrinterIcon, UsbIcon } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { errorText } from '../admin/api';

type Receipt = Schemas['Receipt'];

let printer: Printer | null = null; // one paired printer per device session

export function ReceiptDialog({ billId, subBillId, onClose }: { billId: string; subBillId?: string; onClose: () => void }) {
  const { t } = useTranslation();
  const [tab, setTab] = useState(0);
  const [, setTick] = useState(0);
  const [busy, setBusy] = useState(false);
  // Fetching a copy records a print, so it must happen exactly once per opening: a query keyed
  // to this opening is deduplicated across re-renders and StrictMode's double effects.
  const [openedAt] = useState(Date.now);
  const receipts = useQuery({
    queryKey: ['receipt', billId, subBillId ?? '', openedAt],
    queryFn: async () => {
      const fetchCopy = async (copy: 'customer' | 'internal') =>
        unwrap(await api.POST('/bills/{id}/receipts', { params: { path: { id: billId } }, body: { copy, subBillId } }));
      return [await fetchCopy('customer'), await fetchCopy('internal')];
    },
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
  });
  const copies = receipts.data ?? null;
  useEffect(() => {
    if (receipts.error) {
      toast.error(errorText(receipts.error));
      onClose();
    }
  }, [receipts.error, onClose]);

  const connect = async (kind: 'bt' | 'serial') => {
    try {
      printer = kind === 'bt' ? await connectBluetooth() : await connectSerial();
      setTick((n) => n + 1);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    }
  };
  const print = async (list: Receipt[]) => {
    if (!printer) return;
    setBusy(true);
    try {
      for (const r of list) await printer.write(encode(receiptLines(r as ReceiptData)));
      toast.success(t('waiter.receipt.printed'));
    } catch {
      toast.error(t('waiter.receipt.failed'));
    } finally {
      setBusy(false);
    }
  };

  const current = copies?.[tab];
  return (
    <Dialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={t('waiter.receipt.title')}
      description={printer ? t('waiter.receipt.connected', { name: printer.name }) : bluetoothSupported() ? undefined : t('waiter.receipt.noBluetooth')}
      footer={
        <>
          {!printer && bluetoothSupported() && (
            <Button variant="secondary" onClick={() => void connect('bt')}><BluetoothIcon aria-hidden />{t('waiter.receipt.connect')}</Button>
          )}
          {!printer && !bluetoothSupported() && serialSupported() && (
            <Button variant="secondary" onClick={() => void connect('serial')}><UsbIcon aria-hidden />{t('waiter.receipt.connect')}</Button>
          )}
          {printer && copies && (
            <Button disabled={busy} onClick={() => void print(copies)}><PrinterIcon aria-hidden />{t('waiter.receipt.printBoth')}</Button>
          )}
          <Button variant={printer ? 'secondary' : 'primary'} onClick={onClose}>{t('waiter.receipt.done')}</Button>
        </>
      }
    >
      <div className="mb-3 flex gap-1.5" role="tablist">
        {[t('waiter.receipt.customer'), t('waiter.receipt.internal')].map((label, i) => (
          <button key={label} type="button" role="tab" aria-selected={tab === i} onClick={() => setTab(i)}
            className={cn('h-10 rounded-control px-4 text-sm font-semibold', tab === i ? 'bg-foreground text-background' : 'bg-sunken')}>
            {label}
          </button>
        ))}
      </div>
      {current ? <ReceiptPaper receipt={current} /> : <div className="h-96 animate-pulse rounded-lg bg-sunken" />}
    </Dialog>
  );
}

/** The 58 mm paper on screen: same 32-column lines the printer gets. */
export function ReceiptPaper({ receipt }: { receipt: Receipt }) {
  const lines = receiptLines(receipt as ReceiptData);
  return (
    <pre
      aria-label="Struk"
      data-testid="receipt"
      className="mx-auto w-[34ch] overflow-hidden rounded-sm bg-white px-[1ch] py-4 font-mono text-[13px] leading-[1.35] text-black shadow-md"
    >
      {lines.map((l, i) => (
        <div key={i} className={cn(l.bold && 'font-bold', l.big && 'text-base', l.align === 'center' && 'text-center')}>{l.text || ' '}</div>
      ))}
    </pre>
  );
}
