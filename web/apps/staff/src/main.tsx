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
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Toaster } from 'sonner';
import { AuditPage, ReportsPage } from './admin/audit';
import { CategoriesPage } from './admin/categories';
import { CrewPage } from './admin/crew';
import { DiscountsPage } from './admin/discounts';
import { AdminLayout } from './admin/layout';
import { ProductEditorPage } from './admin/product-editor';
import { ProductsPage } from './admin/products';
import { SettingsPage } from './admin/settings';
import { SoldOutPage } from './admin/sold-out';
import { TablesPage } from './admin/tables';
import { DemoBanner } from './components/demo-banner';
import { HomePage } from './routes/home';
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

const router = createRouter({
  routeTree: rootRoute.addChildren([
    loginRoute,
    homeRoute,
    adminRoute.addChildren([
      adminIndex,
      adminPage('products', (me) => <ProductsPage me={me} />),
      productEditorRoute,
      adminPage('sold-out', () => <SoldOutPage />),
      adminPage('categories', () => <CategoriesPage />, true),
      adminPage('discounts', (me) => <DiscountsPage me={me} />),
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

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
