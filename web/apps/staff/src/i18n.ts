import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import { adminId } from './admin/i18n-admin';
import { cashierId } from './cashier/i18n-cashier';
import { kitchenId } from './kitchen/i18n-kitchen';
import { waiterId } from './waiter/i18n-waiter';

// Staff surfaces are Indonesian (TR-I18N-02). Terms follow docs/design/CONTENT.md §2.
const id = {
  login: {
    title: 'Masuk',
    crewId: 'Crew ID',
    crewIdPlaceholder: 'mis. W001',
    pin: 'PIN',
    submit: 'Masuk',
    wrongPin: 'PIN salah. Sisa {{count}} percobaan.',
    wrongCredentials: 'Crew ID atau PIN salah.',
    locked: 'Akun terkunci sampai {{time}}.',
    rateLimited: 'Terlalu banyak percobaan. Coba lagi sebentar.',
    network: 'Tidak bisa terhubung ke server.',
    demoTitle: 'Akun demo',
    demoHint: 'Ketuk akun untuk mengisi Crew ID.',
  },
  roles: { management: 'Management', outlet_admin: 'Outlet Admin', crew: 'Crew' },
  stations: { waiter: 'Pelayan', cashier: 'Kasir', kitchen: 'Dapur' },
  screens: { kitchen: 'Layar Dapur', bar: 'Layar Bar', display: 'Product Display', hris: 'Tablet absensi' },
  home: {
    greeting: 'Halo, {{name}}',
    modules: 'Modul',
    comingSoon: 'Segera',
    screenMode: 'Jadikan perangkat ini layar',
    screenModeHint: 'Hanya Outlet Admin dan Management. Berlaku sampai akhir hari.',
    logout: 'Keluar',
  },
  realtime: { open: 'Terhubung', connecting: 'Menghubungkan…', closed: 'Terputus' },
  demo: { banner: 'Demo — data direset setiap malam' },
  modules: {
    admin: 'Admin Panel',
    waiter: 'Pelayan',
    kitchen: 'Dapur',
    cashier: 'Kasir',
    hris: 'HRIS',
  },
};

void i18n.use(initReactI18next).init({
  lng: 'id',
  fallbackLng: 'id',
  resources: { id: { translation: { ...id, admin: adminId, waiter: waiterId, kitchen: kitchenId, cashier: cashierId } } },
  interpolation: { escapeValue: false },
});

export default i18n;
