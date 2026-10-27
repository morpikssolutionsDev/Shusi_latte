// Cashier module shell: 10–11″ landscape tablet or PC (LAYOUT.md §3). Tabs across the top; the
// drawer state is always visible because no cash can be taken while it is closed (BR-CS-04).
import type { Principal } from '@sushi-latte/api-client';
import { Chip } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { Link, Outlet, useRouterState } from '@tanstack/react-router';
import { ArrowLeftIcon, CloudOffIcon, LockIcon, ReceiptTextIcon, WalletIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { drawerQuery, useCashierRealtime } from './api';

export function CashierLayout({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const status = useCashierRealtime();
  const drawer = useQuery(drawerQuery);
  const path = useRouterState({ select: (s) => s.location.pathname });
  const pending = drawer.data?.pending.length ?? 0;
  const tabs = [
    { to: '/cashier', label: t('cashier.tabs.bills'), icon: ReceiptTextIcon, badge: 0 },
    { to: '/cashier/drawer', label: t('cashier.tabs.drawer'), icon: WalletIcon, badge: pending },
    { to: '/cashier/outage', label: t('cashier.tabs.outage'), icon: CloudOffIcon, badge: 0 },
  ] as const;
  // Takeaway and Order entry belong to the Bills tab.
  const onEntry = /\/cashier\/(takeaway|bills|orders)\b/.test(path);
  return (
    <div className="flex h-[calc(100dvh-28px)] flex-col">
      <header className="flex items-center gap-3 border-b border-border-subtle bg-background px-4 py-2">
        <Link to="/" className="grid size-10 place-items-center rounded-control hover:bg-accent" aria-label={t('admin.nav.back')}>
          <ArrowLeftIcon className="size-5" aria-hidden />
        </Link>
        <div className="min-w-0">
          <p className="font-bold">{t('modules.cashier')}</p>
          <p className="text-xs text-muted-foreground"><span className="font-mono">{me.crewId}</span> · {me.name} · {t(`realtime.${status}`)}</p>
        </div>
        <nav className="ml-4 flex gap-1" aria-label={t('modules.cashier')}>
          {tabs.map(({ to, label, icon: Icon, badge }) => (
            // Bills stay active on the entry screens under /cashier; the other tabs match their own path.
            <Link key={to} to={to} activeOptions={{ exact: to !== '/cashier' ? false : !onEntry, includeSearch: false }}
              className="inline-flex h-11 items-center gap-2 rounded-control px-4 text-sm font-semibold text-muted-foreground hover:bg-accent data-[status=active]:bg-foreground data-[status=active]:text-background">
              <Icon className="size-4" aria-hidden />
              {label}
              {badge > 0 && (
                <span className="tabular grid h-5 min-w-5 place-items-center rounded-full bg-[var(--sl-color-status-pending-solid)] px-1 text-[11px] font-bold text-white">{badge}</span>
              )}
            </Link>
          ))}
        </nav>
        <span className="flex-1" />
        {drawer.data && !drawer.data.session && (
          <Link to="/cashier/drawer">
            <Chip feedback="warning" icon={<LockIcon aria-hidden />}>{t('cashier.drawer.closedTitle')}</Chip>
          </Link>
        )}
      </header>
      <main className="min-h-0 flex-1">
        <Outlet />
      </main>
    </div>
  );
}
