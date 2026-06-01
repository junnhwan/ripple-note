import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: Number(__ENV.VUS || 100),
  duration: __ENV.DURATION || "5m",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<800"],
  },
};

const baseURL = __ENV.BASE_URL || "http://127.0.0.1:18080";
const loginEmail = __ENV.LOGIN_EMAIL || "loadtest@ripple.dev";
const loginPassword = __ENV.LOGIN_PASSWORD || "loadtest123";

export function setup() {
  const res = http.post(
    `${baseURL}/api/sessions`,
    JSON.stringify({ email: loginEmail, password: loginPassword }),
    { headers: { "Content-Type": "application/json" } },
  );
  return { token: res.json()?.data?.token || "" };
}

export default function (data) {
  const rand = Math.random();
  let path = "/api/feed/latest?limit=20";
  const params = { tags: { endpoint: "feed_latest_anonymous" } };

  if (rand >= 0.55 && rand < 0.8) {
    path = "/api/feed/latest?limit=20";
    params.headers = { Authorization: `Bearer ${data.token}` };
    params.tags = { endpoint: "feed_latest_auth" };
  } else if (rand >= 0.8 && rand < 0.95) {
    path = "/api/feed/hot?limit=20";
    params.tags = { endpoint: "feed_hot_anonymous" };
  } else if (rand >= 0.95) {
    path = "/api/feed/following?limit=20";
    params.headers = { Authorization: `Bearer ${data.token}` };
    params.tags = { endpoint: "feed_following_auth" };
  }

  const res = http.get(`${baseURL}${path}`, params);
  check(res, {
    "status is 200": (r) => r.status === 200,
    "response has data": (r) => Boolean(r.json()?.data),
  });
  sleep(Number(__ENV.SLEEP || 0.2));
}
