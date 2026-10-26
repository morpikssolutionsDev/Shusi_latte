# Receipts print from the browser over Web Bluetooth, so staff devices must run Chrome

Receipts print directly from the Staff PWA to a 58 mm Bluetooth thermal printer paired with each Waiter tablet and Cashier PC, using the Web Bluetooth API to send ESC/POS commands. This makes printing work without internet and without any local print server. Web Bluetooth exists only in Chromium browsers, so staff devices are Android tablets and Windows PCs running Chrome; iPads and Safari are not supported.

## Considered Options

- **Browser print dialog**: rejected; slow, inconsistent paper sizing, and needs an OS-level printer driver on every device.
- **Local print agent (a small Go program on each device)**: rejected for now; an extra install per device, and impossible on Android tablets without a native app.
- **Native wrapper (Capacitor) for full Bluetooth access**: kept as the fallback if iPads are ever required.

## Consequences

- Web Bluetooth reaches only **Bluetooth Low Energy** printers. Many cheap 58 mm printers are Bluetooth Classic (SPP) only and will not work. Printer models must be tested before purchase.
- Windows PCs can use the Web Serial API with the printer paired as a COM port, for Classic-only printers.
- Pairing requires a user tap; the app must handle reconnection after a tablet sleeps.
