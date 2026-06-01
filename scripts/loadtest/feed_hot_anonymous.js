import http from "k6/http";
import { check, sleep } from "k6";
import { Trend, Rate } from "k6/metrics";

export const options = {
  vus: Number(__ENV.VUS || 50),
  duration: __ENV.DURATION || "3m",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500"],
  },
};

const baseURL = __ENV.BASE_URL || "http://127.0.0.1:18080";
const limit = Number(__ENV.LIMIT || 20);
const responseTime = new Trend("feed_hot_anonymous_duration", true);
const successRate = new Rate("feed_hot_anonymous_success");

export default function () {
  const res = http.get(`${baseURL}/api/feed/hot?limit=${limit}`, {
    tags: { endpoint: "feed_hot_anonymous" },
  });

  responseTime.add(res.timings.duration);
  const ok = check(res, {
    "status is 200": (r) => r.status === 200,
    "has feed items": (r) => {
      const body = r.json();
      return Array.isArray(body?.data?.items);
    },
  });
  successRate.add(ok);
  sleep(Number(__ENV.SLEEP || 0.2));
}
