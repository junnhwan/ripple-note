import http from "k6/http";
import { check, sleep } from "k6";
import { Trend, Rate } from "k6/metrics";

export const options = {
  vus: Number(__ENV.VUS || 50),
  duration: __ENV.DURATION || "3m",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<800"],
  },
};

const baseURL = __ENV.BASE_URL || "http://127.0.0.1:18080";
const limit = Number(__ENV.LIMIT || 20);
const loginEmail = __ENV.LOGIN_EMAIL || "loadtest@ripple.dev";
const loginPassword = __ENV.LOGIN_PASSWORD || "loadtest123";
const responseTime = new Trend("feed_latest_auth_duration", true);
const successRate = new Rate("feed_latest_auth_success");

export function setup() {
  const res = http.post(
    `${baseURL}/api/sessions`,
    JSON.stringify({ email: loginEmail, password: loginPassword }),
    { headers: { "Content-Type": "application/json" } },
  );

  check(res, {
    "login status is 200": (r) => r.status === 200,
    "login returns token": (r) => Boolean(r.json()?.data?.token),
  });

  return { token: res.json()?.data?.token || "" };
}

export default function (data) {
  const res = http.get(`${baseURL}/api/feed/latest?limit=${limit}`, {
    headers: { Authorization: `Bearer ${data.token}` },
    tags: { endpoint: "feed_latest_auth" },
  });

  responseTime.add(res.timings.duration);
  const ok = check(res, {
    "status is 200": (r) => r.status === 200,
    "has viewer flags": (r) => {
      const first = r.json()?.data?.items?.[0];
      return first && "viewer_liked" in first && "viewer_favorited" in first && "viewer_following" in first;
    },
  });
  successRate.add(ok);
  sleep(Number(__ENV.SLEEP || 0.2));
}
