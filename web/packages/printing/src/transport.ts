// Printer transports (ADR-0004): Web Bluetooth to a BLE printer's write characteristic, with a
// Web Serial fallback for Bluetooth-Classic printers paired as a COM port on Windows.
// Writes are chunked to the characteristic size and retried; a failure never loses the Bill
// state because the Receipt data is already on the device (TR-PRN-04, TR-PRN-06).

/* Minimal typings for the parts of Web Bluetooth / Web Serial used here (not in lib.dom yet). */
interface BleCharacteristic {
  properties: { write: boolean; writeWithoutResponse: boolean };
  writeValueWithoutResponse(data: BufferSource): Promise<void>;
  writeValueWithResponse(data: BufferSource): Promise<void>;
}
interface BleService {
  getCharacteristics(): Promise<BleCharacteristic[]>;
}
interface BleDevice {
  name?: string;
  gatt?: { connected: boolean; connect(): Promise<{ getPrimaryServices(): Promise<BleService[]> }> };
}
interface NavigatorBluetooth {
  requestDevice(options: { acceptAllDevices: boolean; optionalServices: string[] }): Promise<BleDevice>;
}
interface SerialPortLike {
  open(o: { baudRate: number }): Promise<void>;
  writable: WritableStream<Uint8Array> | null;
  close(): Promise<void>;
}
interface NavigatorSerial {
  requestPort(): Promise<SerialPortLike>;
}

const nav = () => navigator as Navigator & { bluetooth?: NavigatorBluetooth; serial?: NavigatorSerial };

/** Web Bluetooth needs Chrome (Android/Windows) on HTTPS (TR-PRN-07). */
export const bluetoothSupported = () => typeof navigator !== 'undefined' && !!nav().bluetooth && isSecureContext;
export const serialSupported = () => typeof navigator !== 'undefined' && !!nav().serial && isSecureContext;

// GATT services exposed by common 58 mm BLE printers (validated in the hardware spike, TR-TST-04).
const PRINTER_SERVICES = [
  '000018f0-0000-1000-8000-00805f9b34fb',
  'e7810a71-73ae-499d-8c15-faa9aef0c3f2',
  '49535343-fe7d-4ae5-8fa9-9fafd205e455',
];

export interface Printer {
  readonly name: string;
  write(data: Uint8Array): Promise<void>;
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export async function connectBluetooth(): Promise<Printer> {
  const bt = nav().bluetooth;
  if (!bt) throw new Error('Web Bluetooth tidak didukung di perangkat ini.');
  const device = await bt.requestDevice({ acceptAllDevices: true, optionalServices: PRINTER_SERVICES });
  const server = await device.gatt!.connect();
  let target: BleCharacteristic | undefined;
  for (const svc of await server.getPrimaryServices()) {
    for (const c of await svc.getCharacteristics()) {
      if (c.properties.writeWithoutResponse || c.properties.write) {
        target = c;
        break;
      }
    }
    if (target) break;
  }
  if (!target) throw new Error('Printer tidak punya karakteristik tulis.');
  const ch = target;
  return {
    name: device.name ?? 'Printer Bluetooth',
    async write(data) {
      const chunk = 180; // under the common 182-byte MTU payload
      for (let i = 0; i < data.length; i += chunk) {
        const part = data.slice(i, i + chunk);
        for (let attempt = 0; ; attempt++) {
          try {
            if (ch.properties.writeWithoutResponse) await ch.writeValueWithoutResponse(part);
            else await ch.writeValueWithResponse(part);
            break;
          } catch (e) {
            if (attempt >= 2) throw e;
            await sleep(100 * (attempt + 1));
          }
        }
        await sleep(15); // flow control for printers without write-with-response
      }
    },
  };
}

export async function connectSerial(): Promise<Printer> {
  const serial = nav().serial;
  if (!serial) throw new Error('Web Serial tidak didukung di perangkat ini.');
  const port = await serial.requestPort();
  await port.open({ baudRate: 9600 });
  return {
    name: 'Printer COM',
    async write(data) {
      const writer = port.writable!.getWriter();
      try {
        await writer.write(data);
      } finally {
        writer.releaseLock();
      }
    },
  };
}
