// Demo launcher (DEMO-AND-SEED §2): every Table with its QR code, scenario and an
// "Open as Customer" link (each tab is its own Customer), plus the staff screens.
import { api, unwrap } from '@sushi-latte/api-client';
import { Logo, cn } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { ExternalLinkIcon, MonitorPlayIcon, UsersIcon } from 'lucide-react';
import QRCode from 'qrcode';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { LanguageToggle } from './shell';

const staffUrl = import.meta.env.VITE_STAFF_URL ?? '/staff/';
const displayUrl = import.meta.env.VITE_DISPLAY_URL ?? '/display/';

export function DemoLauncher() {
  const { t } = useTranslation();
  const tables = useQuery({ queryKey: ['demo-tables'], queryFn: async () => unwrap(await api.GET('/demo/tables')) });
  return (
    <main className="mx-auto max-w-5xl px-4 py-8">
      <header className="flex flex-wrap items-center gap-4">
        <Logo variant="lockup" className="w-48" />
        <div className="min-w-0 flex-1">
          <h1 className="font-display text-2xl font-bold">{t('demo.title')}</h1>
          <p className="text-sm text-text-secondary">{t('demo.body')}</p>
        </div>
        <LanguageToggle />
      </header>
      <nav className="mt-6 flex flex-wrap gap-2">
        <a href={staffUrl} target="_blank" rel="noreferrer" className="inline-flex h-11 items-center gap-2 rounded-full bg-[var(--sl-color-brand-espresso)] px-4 text-sm font-semibold text-[var(--sl-color-brand-rice)]">
          <UsersIcon className="size-4" aria-hidden />
          {t('demo.staff')}
        </a>
        <a href={displayUrl} target="_blank" rel="noreferrer" className="inline-flex h-11 items-center gap-2 rounded-full bg-sunken px-4 text-sm font-semibold">
          <MonitorPlayIcon className="size-4" aria-hidden />
          {t('demo.display')}
        </a>
      </nav>
      <ul className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {tables.data?.map((tb) => (
          <li key={tb.number} className={cn('flex gap-4 rounded-[20px] bg-card p-4 shadow-[var(--sl-product-card-shadow)]', !tb.active && 'opacity-70')} data-table={tb.number}>
            <Qr value={`${location.origin}/t/${tb.qrToken}`} />
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <h2 className="font-display text-lg font-bold">{tb.label}{!tb.active && <span className="ml-2 text-xs font-semibold text-text-secondary">{t('demo.inactive')}</span>}</h2>
              <p className="text-xs text-text-secondary">{tb.scenario}</p>
              <a
                href={`/t/${tb.qrToken}?fresh=1`}
                target="_blank"
                rel="noreferrer"
                className="mt-auto inline-flex h-10 items-center justify-center gap-1.5 whitespace-nowrap rounded-full bg-primary px-3 text-[13px] font-semibold text-primary-foreground"
              >
                {t('demo.openAsCustomer')}
                <ExternalLinkIcon className="size-4" aria-hidden />
              </a>
            </div>
          </li>
        ))}
      </ul>
    </main>
  );
}

function Qr({ value }: { value: string }) {
  const [src, setSrc] = useState('');
  useEffect(() => {
    void QRCode.toDataURL(value, { margin: 1, width: 240 }).then(setSrc);
  }, [value]);
  return src ? <img src={src} alt="" className="size-28 shrink-0 rounded-lg" /> : <div className="size-28 shrink-0 rounded-lg bg-sunken" />;
}
