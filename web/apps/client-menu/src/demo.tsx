// Demo launcher (DEMO-AND-SEED §2): the guided tours (§10), then every Table with its QR code,
// scenario and an "Open as Customer" link (each tab is its own Customer), plus the staff screens.
import { api, unwrap } from '@sushi-latte/api-client';
import { Logo, cn } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { ExternalLinkIcon, MonitorPlayIcon, RouteIcon, UsersIcon } from 'lucide-react';
import QRCode from 'qrcode';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { LanguageToggle } from './shell';
import { TOURS, screenUrl } from './tours';

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
      <Tours qrOf={(n) => tables.data?.find((tb) => tb.number === n)?.qrToken} />
      <h2 className="mt-8 font-display text-xl font-bold">{t('demo.tables')}</h2>
      <ul className="mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {tables.data?.map((tb) => (
          <li key={tb.number} className={cn('flex gap-4 rounded-[20px] bg-card p-4 shadow-[var(--sl-product-card-shadow)]', !tb.active && 'border border-dashed border-border')} data-table={tb.number}>
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

function Tours({ qrOf }: { qrOf: (n: number) => string | undefined }) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language.startsWith('en') ? 'en' : 'id';
  return (
    <section className="mt-8" aria-labelledby="tours-title">
      <h2 id="tours-title" className="font-display text-xl font-bold">{t('demo.tours')}</h2>
      <p className="text-sm text-text-secondary">{t('demo.toursBody')}</p>
      <ol className="mt-3 grid gap-3 lg:grid-cols-2">
        {TOURS.map((tour, i) => (
          <li key={tour.title.en} className="rounded-[20px] bg-card shadow-[var(--sl-product-card-shadow)]" data-tour={i + 1}>
            <details className="group p-4" open={i === 0}>
              <summary className="flex cursor-pointer list-none items-start gap-3">
                <span className="tabular grid size-8 shrink-0 place-items-center rounded-full bg-primary text-sm font-bold text-primary-foreground">{i + 1}</span>
                <span className="min-w-0 flex-1">
                  <span className="block font-display text-lg font-bold">{tour.title[lang]}</span>
                  <span className="block text-xs text-text-secondary">{tour.flows[lang]}</span>
                </span>
                <RouteIcon className="mt-1 size-5 shrink-0 text-text-secondary transition-transform group-open:rotate-90" aria-hidden />
              </summary>
              <div className="mt-3 flex flex-wrap gap-2">
                {tour.screens.map((s) => {
                  const href = screenUrl(s.to, qrOf, staffUrl);
                  return href ? (
                    <a key={s.label.en} href={href} target="_blank" rel="noreferrer"
                      className="inline-flex h-9 items-center gap-1.5 rounded-full bg-sunken px-3 text-xs font-semibold">
                      {s.label[lang]}<ExternalLinkIcon className="size-3.5" aria-hidden />
                    </a>
                  ) : null;
                })}
              </div>
              <ol className="mt-3 flex list-decimal flex-col gap-1.5 pl-5 text-sm">
                {tour.steps.map((step) => <li key={step.en}>{step[lang]}</li>)}
              </ol>
            </details>
          </li>
        ))}
      </ol>
    </section>
  );
}

function Qr({ value }: { value: string }) {
  const [src, setSrc] = useState('');
  useEffect(() => {
    void QRCode.toDataURL(value, { margin: 1, width: 240 }).then(setSrc);
  }, [value]);
  return src ? <img src={src} alt="" className="size-28 shrink-0 rounded-lg" /> : <div className="size-28 shrink-0 rounded-lg bg-sunken" />;
}
