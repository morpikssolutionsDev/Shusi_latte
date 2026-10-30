// TR-KT-01: ticket timers keep running between queue fetches instead of freezing and jumping.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { serverTime } from '../src/kitchen/clock.ts';

test('the clock advances with local time while the queue response stays the same', () => {
  const server = 1_000_000;
  const received = 5_000;
  assert.equal(serverTime(server, received, received), server);
  assert.equal(serverTime(server, received, received + 7_000), server + 7_000);
  assert.equal(serverTime(server, received, received + 29_000), server + 29_000);
});

test('a screen whose clock runs behind the server still shows server time', () => {
  assert.equal(serverTime(1_000_000, 940_000, 950_000), 1_010_000);
});
