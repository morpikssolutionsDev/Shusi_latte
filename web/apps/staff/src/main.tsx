import '@sushi-latte/ui/styles.css';
import './i18n';

import type { Principal } from '@sushi-latte/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  Outlet,
  RouterProvider,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router';
import { mountApp } from '@sushi-latte/ui';
import { StrictMode } from 'react';
import { Toaster } from 'sonner';
import { AuditPage } from './admin/audit';
import { ReportsPage } from './admin/reports';
import { CategoriesPage } from './admin/categories';
import { CrewPage } from './admin/crew';
import { DiscountsPage } from './admin/discounts';
import { DisplayPage } from './admin/display';
import { AdminLayout } from './admin/layout';
import { ProductEditorPage } from './admin/product-editor';
import { ProductsPage } from './admin/products';
import { SettingsPage } from './admin/settings';
import { SoldOutPage } from './admin/sold-out';
import { TablesPage } from './admin/tables';
import { DemoBanner } from './components/demo-banner';
import { HomePage } from './routes/home';
import { CashierBillsPage } from './cashier/bills';
import { DrawerPage } from './cashier/drawer';
import { CashierLayout } from './cashier/layout';
import { OutagePage } from './cashier/outage';
import { KitchenDisplay } from './kitchen/display';
import { BillPage } from './waiter/bill';
import { EntryPage } from './waiter/entry';
import { FloorPage, MyOrdersPage } from './waiter/floor';
import { WaiterLayout } from './waiter/layout';
import { NotificationsPage } from './waiter/notifications';
import { useTranslation } from 'react-i18next';
import { LoginPage } from './routes/login';
import { meQuery } from './session';

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: true } } });

const rootRoute = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    <>
      <DemoBanner />
      <Outlet />
      <Toaster position="top-right" richColors closeButton />
    </>
  ),
});

async function requireMe(queryClient: QueryClient, check?: (me: Principal) => boolean) {
  const me = await queryClient.fetchQuery(meQuery);
  if (!me) throw redirect({ to: '/login' });
  if (check && !check(me)) throw redirect({ to: '/' });
  return { me };
}

const isAdmin = (me: Principal) => me.kind === 'crew' && (me.role === 'management' || me.role === 'outlet_admin');
const isManagement = (me: Principal) => me.kind === 'crew' && me.role === 'management';

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  beforeLoad: async ({ context }) => {
    if (await context.queryClient.fetchQuery(meQuery)) throw redirect({ to: '/' });
  },
  component: LoginPage,
});

const homeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: ({ context }) => requireMe(context.queryClient),
  component: function Home() {
    const { me } = homeRoute.useRouteContext();
    return <HomePage me={me} />;
  },
});

// ---------------------------------------------------------------- Admin Panel (Phase 1)

const adminRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin',
  beforeLoad: ({ context }) => requireMe(context.queryClient, isAdmin),
  component: function Admin() {
    const { me } = adminRoute.useRouteContext();
    return <AdminLayout me={me} />;
  },
});

function adminPage<TPath extends string>(path: TPath, render: (me: Principal) => React.ReactNode, managementOnly = false) {
  const route = createRoute({
    getParentRoute: () => adminRoute,
    path,
    beforeLoad: ({ context }) => (managementOnly ? requireMe(context.queryClient, isManagement) : undefined),
    component: function Page() {
      const { me } = route.useRouteContext() as { me: Principal };
      return <>{render(me)}</>;
    },
  });
  return route;
}

const adminIndex = createRoute({
  getParentRoute: () => adminRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/admin/products' });
  },
});

const productEditorRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'products/$productId',
  component: function Editor() {
    const { me } = productEditorRoute.useRouteContext();
    const { productId } = productEditorRoute.useParams();
    return <ProductEditorPage key={productId} me={me} productId={productId} />;
  },
});

// ---------------------------------------------------------------- Waiter (Phase 4)

const isFloorStaff = (me: Principal) =>
  me.kind === 'crew' && (isAdmin(me) || me.stations.includes('waiter') || me.stations.includes('cashier'));

const waiterRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/waiter',
  beforeLoad: ({ context }) => requireMe(context.queryClient, isFloorStaff),
  component: function Waiter() {
    const { me } = waiterRoute.useRouteContext();
    return <WaiterLayout me={me} />;
  },
});
const waiterIndex = createRoute({ getParentRoute: () => waiterRoute, path: '/', component: NotificationsPage });
const waiterTables = createRoute({ getParentRoute: () => waiterRoute, path: 'tables', component: FloorPage });
const waiterOrders = createRoute({ getParentRoute: () => waiterRoute, path: 'orders', component: MyOrdersPage });
const waiterBill = createRoute({
  getParentRoute: () => waiterRoute,
  path: 'bills/$billId',
  component: function Bill() {
    const { me } = waiterBill.useRouteContext() as { me: Principal };
    const { billId } = waiterBill.useParams();
    return <BillPage key={billId} me={me} billId={billId} />;
  },
});
const waiterBillNew = createRoute({
  getParentRoute: () => waiterRoute,
  path: 'bills/$billId/new',
  component: function BillNew() {
    const { billId } = waiterBillNew.useParams();
    const navigate = waiterBillNew.useNavigate();
    const { t } = useTranslation();
    return <EntryPage target={{ billId }} title={t('waiter.floor.newOrder')} back={() => void navigate({ to: '/waiter/bills/$billId', params: { billId } })} />;
  },
});
const waiterTableNew = createRoute({
  getParentRoute: () => waiterRoute,
  path: 'tables/$tableId/new',
  component: function TableNew() {
    const { tableId } = waiterTableNew.useParams();
    const navigate = waiterTableNew.useNavigate();
    const { t } = useTranslation();
    return <EntryPage target={{ tableId }} title={t('waiter.floor.newOrder')} back={() => void navigate({ to: '/waiter/tables' })} />;
  },
});
const waiterOrderAdd = createRoute({
  getParentRoute: () => waiterRoute,
  path: 'orders/$orderId/add',
  validateSearch: (s: Record<string, unknown>) => ({ billId: String(s.billId ?? '') }),
  component: function OrderAdd() {
    const { orderId } = waiterOrderAdd.useParams();
    const { billId } = waiterOrderAdd.useSearch();
    const navigate = waiterOrderAdd.useNavigate();
    const { t } = useTranslation();
    return <EntryPage target={{ orderId }} title={t('waiter.bill.addItems')} back={() => void navigate({ to: '/waiter/bills/$billId', params: { billId } })} />;
  },
});

// ---------------------------------------------------------------- Cashier (Phase 6)

const isCashier = (me: Principal) => me.kind === 'crew' && (isAdmin(me) || me.stations.includes('cashier'));

const cashierRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/cashier',
  beforeLoad: ({ context }) => requireMe(context.queryClient, isCashier),
  component: function Cashier() {
    const { me } = cashierRoute.useRouteContext();
    return <CashierLayout me={me} />;
  },
});
const cashierIndex = createRoute({
  getParentRoute: () => cashierRoute,
  path: '/',
  validateSearch: (s: Record<string, unknown>): { bill?: string } => (typeof s.bill === 'string' && s.bill ? { bill: s.bill } : {}),
  component: function CashierBills() {
    const { me } = cashierIndex.useRouteContext() as { me: Principal };
    const { bill } = cashierIndex.useSearch();
    return <CashierBillsPage me={me} billId={bill} />;
  },
});
const cashierDrawer = createRoute({ getParentRoute: () => cashierRoute, path: 'drawer', component: DrawerPage });
const cashierOutage = createRoute({ getParentRoute: () => cashierRoute, path: 'outage', component: OutagePage });
const cashierTakeaway = createRoute({
  getParentRoute: () => cashierRoute,
  path: 'takeaway',
  component: function Takeaway() {
    const navigate = cashierTakeaway.useNavigate();
    const { t } = useTranslation();
    return (
      <CashierEntry>
        <EntryPage target={{ takeaway: true }} title={t('cashier.takeaway.title')} back={() => void navigate({ to: '/cashier' })}
          onCreated={(bill) => void navigate({ to: '/cashier', search: { bill } })} />
      </CashierEntry>
    );
  },
});
const cashierBillNew = createRoute({
  getParentRoute: () => cashierRoute,
  path: 'bills/$billId/new',
  component: function CashierBillNew() {
    const { billId } = cashierBillNew.useParams();
    const navigate = cashierBillNew.useNavigate();
    const { t } = useTranslation();
    return (
      <CashierEntry>
        <EntryPage target={{ billId }} title={t('waiter.floor.newOrder')} back={() => void navigate({ to: '/cashier', search: { bill: billId } })} />
      </CashierEntry>
    );
  },
});
const cashierOrderAdd = createRoute({
  getParentRoute: () => cashierRoute,
  path: 'orders/$orderId/add',
  validateSearch: (s: Record<string, unknown>) => ({ billId: String(s.billId ?? '') }),
  component: function CashierOrderAdd() {
    const { orderId } = cashierOrderAdd.useParams();
    const { billId } = cashierOrderAdd.useSearch();
    const navigate = cashierOrderAdd.useNavigate();
    const { t } = useTranslation();
    return (
      <CashierEntry>
        <EntryPage target={{ orderId }} title={t('waiter.bill.addItems')} back={() => void navigate({ to: '/cashier', search: { bill: billId } })} />
      </CashierEntry>
    );
  },
});

/** Entry screens scroll inside the Cashier's fixed-height shell. */
function CashierEntry({ children }: { children: React.ReactNode }) {
  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-5xl p-6">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------- Kitchen (Phase 5)

const isKitchen = (me: Principal) =>
  me.kind === 'screen' ? me.screen === 'kitchen' || me.screen === 'bar' : isAdmin(me) || me.stations.includes('kitchen');

const kitchenRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/kitchen/$area',
  beforeLoad: ({ context }) => requireMe(context.queryClient, isKitchen),
  component: function Kitchen() {
    const { area } = kitchenRoute.useParams();
    return <KitchenDisplay key={area} area={area} />;
  },
});

const router = createRouter({
  routeTree: rootRoute.addChildren([
    loginRoute,
    homeRoute,
    kitchenRoute,
    waiterRoute.addChildren([waiterIndex, waiterTables, waiterOrders, waiterBill, waiterBillNew, waiterTableNew, waiterOrderAdd]),
    cashierRoute.addChildren([cashierIndex, cashierDrawer, cashierOutage, cashierTakeaway, cashierBillNew, cashierOrderAdd]),
    adminRoute.addChildren([
      adminIndex,
      adminPage('products', (me) => <ProductsPage me={me} />),
      productEditorRoute,
      adminPage('sold-out', () => <SoldOutPage />),
      adminPage('categories', () => <CategoriesPage />, true),
      adminPage('discounts', (me) => <DiscountsPage me={me} />),
      adminPage('display', (me) => <DisplayPage me={me} />),
      adminPage('tables', () => <TablesPage />),
      adminPage('crew', () => <CrewPage />, true),
      adminPage('settings', (me) => <SettingsPage me={me} />),
      adminPage('reports', () => <ReportsPage />, true),
      adminPage('audit', (me) => <AuditPage canListCrew={me.role === 'management'} />),
    ]),
  ]),
  basepath: '/staff',
  context: { queryClient },
  defaultPreload: 'intent',
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

mountApp(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
