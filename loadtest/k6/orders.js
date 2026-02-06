/**
 * k6 load test: POST /orders against gateway
 * Env: K6_DURATION (default 60), K6_VUS (default 5)
 */
import http from 'k6/http';
import { check, sleep } from 'k6';

const base = __ENV.GATEWAY_URL || 'http://localhost:8080';
const duration = __ENV.K6_DURATION || '60';
const vus = __ENV.K6_VUS || 5;

export const options = {
  vus: parseInt(vus, 10),
  duration: duration + 's',
  thresholds: {
    http_req_failed: ['rate<0.1'],
    http_req_duration: ['p(95)<2000'],
  },
};

function randomId() {
  return 'k6-' + Date.now() + '-' + Math.random().toString(36).slice(2, 10);
}

export default function () {
  const idem = randomId();
  const payload = JSON.stringify({
    user_id: 'u-' + idem,
    amount_cents: 1999,
    currency: 'USD',
    idempotency_key: idem,
  });
  const res = http.post(base + '/orders', payload, {
    headers: { 'Content-Type': 'application/json' },
  });
  check(res, { 'status 200 or 201': (r) => r.status === 200 || r.status === 201 });
  sleep(0.5);
}
