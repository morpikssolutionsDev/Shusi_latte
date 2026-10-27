import type { Principal } from '@sushi-latte/api-client';
import { Logo, cn } from '@sushi-latte/ui';
import { useQueryClient } from '@tanstack/react-query';
import { Link, Outlet } from '@tanstack/react-router';
import {
  ArmchairIcon,
  ArrowLeftIcon,
  BarChart3Icon,
  CircleSlashIcon,
  MonitorPlayIcon,
  PercentIcon,
  ScrollTextIcon,
  SettingsIcon,
  TagsIcon,
  UsersIcon,
  UtensilsCrossedIcon,
  type LucideIcon,
} from 'lucide-react';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useRealtime } from '../realtime';

type NavItem = { to: string; key: string; icon: LucideIcon; managementOnly?: boolean };

const NAV: NavItem[] = [
  { to: '/admin/products', key: 'products', icon: UtensilsCrossedIcon },
  { to: '/admin/sold-out', key: 'soldOut', icon: CircleSlashIcon },
  { to: '/admin/categories', key: 'categories', icon: TagsIcon, managementOnly: true },
  { to: '/admin/discounts', key: 'discounts', icon: PercentIcon },
  { to: '/admin/display', key: 'display', icon: MonitorPlayIcon },
  { to: '/admin/tables', key: 'tables', icon: ArmchairIcon },
  { to: '/admin/crew', key: 'crew', icon: UsersIcon, managementOnly: true },
  { to: '/admin/settings', key: 'settings', icon: SettingsIcon },
  { to: '/admin/reports', key: 'reports', icon: BarChart3Icon, managementOnly: true },
  { to: '/admin/audit', key: 'audit', icon: ScrollTextIcon },
];

/** Admin Panel shell (LAYOUT.md §3): left rail on desktop, scrolling tab bar on tablets. */
export function AdminLayout({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  // Catalog edits from other devices (and Sold Out from the Kitchen later) refresh this screen.
  useRealtime(['menu'], () => void queryClient.invalidateQueries({ queryKey: ['catalog'] }));

  useEffect(() => {
    const root = document.documentElement;
    root.dataset.surface = 'admin';
    return () => {
      root.dataset.surface = 'staff';
    };
  }, []);

  const items = NAV.filter((n) => !n.managementOnly || me.role === 'management');

  return (
    <div className="flex min-h-[calc(100dvh-28px)] flex-col lg:flex-row">
      <nav aria-label={t('admin.nav.title')} className="border-border-subtle bg-card lg:sticky lg:top-0 lg:h-dvh lg:w-62 lg:shrink-0 lg:border-r">
        <div className="hidden items-center gap-2 px-5 py-5 lg:flex">
          <Logo className="size-10" />
          <span className="font-display text-base font-semibold">{t('admin.nav.title')}</span>
        </div>
        <ul className="flex gap-1 overflow-x-auto border-b border-border-subtle px-2 py-2 lg:flex-col lg:border-b-0 lg:px-3">
          {items.map(({ to, key, icon: Icon }) => (
            <li key={to}>
              <Link
                to={to}
                className="flex items-center gap-3 whitespace-nowrap rounded-control px-3 py-2.5 text-sm font-semibold text-text-secondary hover:bg-accent hover:text-foreground"
                activeProps={{ className: cn('bg-secondary text-foreground') }}
              >
                <Icon aria-hidden className="size-5" />
                {t(`admin.nav.${key}`)}
              </Link>
            </li>
          ))}
          <li className="lg:mt-4 lg:border-t lg:border-border-subtle lg:pt-4">
            <Link to="/" className="flex items-center gap-3 whitespace-nowrap rounded-control px-3 py-2.5 text-sm font-semibold text-muted-foreground hover:bg-accent">
              <ArrowLeftIcon aria-hidden className="size-5" />
              {t('admin.nav.back')}
            </Link>
          </li>
        </ul>
      </nav>
      <main className="mx-auto w-full max-w-[1440px] flex-1 px-4 py-6 lg:px-12">
        <Outlet />
      </main>
    </div>
  );
}

export function PageHeader({ title, actions, children }: { title: string; actions?: React.ReactNode; children?: React.ReactNode }) {
  return (
    <header className="mb-6 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 className="text-2xl font-bold">{title}</h1>
        {children && <p className="mt-1 max-w-2xl text-sm text-muted-foreground">{children}</p>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </header>
  );
}
