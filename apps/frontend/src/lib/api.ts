// Frontend API layer.
//
// Strategy:
// - use the live Go/Chi backend when `VITE_API_BASE_URL` is configured
// - fall back to local mocks for dashboard data while the backend routes are
//   still being built out
//
// Current gateway routes in the Go scaffold:
//   POST /mcp/:serverId
//   GET  /v1/challenge/:requestId
//   POST /v1/verify
//
// Planned dashboard routes:
//   GET /v1/servers
//   GET /v1/servers/:serverId/tools
//   GET /v1/requests
//   GET /v1/requests/:requestId
//   GET /v1/dashboard/summary
//   GET /v1/dashboard/receipts

import { receipts, requests, servers, summary, tools } from "./mock/data";
import type { DashboardSummary, PaymentRequest, Receipt, Server, ToolPricing } from "./mock/types";

const delay = (ms = 120) => new Promise((r) => setTimeout(r, ms));
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL?.replace(/\/$/, "");
const USE_MOCK_FALLBACK = import.meta.env.VITE_USE_MOCK_FALLBACK !== "false";

export const backendRoutes = {
  servers: "/v1/servers",
  serverTools: (serverId: string) => `/v1/servers/${serverId}/tools`,
  requests: "/v1/requests",
  request: (requestId: string) => `/v1/requests/${requestId}`,
  invokeMcp: (serverId: string) => `/mcp/${serverId}`,
  challenge: (requestId: string) => `/v1/challenge/${requestId}`,
  verify: "/v1/verify",
  dashboardSummary: "/v1/dashboard/summary",
  dashboardReceipts: "/v1/dashboard/receipts",
} as const;

export type InvokeMcpToolInput = {
  tool: string;
  input?: Record<string, unknown>;
};

export type PaymentChallenge = {
  requestId: string;
  serverId: string;
  toolName: string;
  amountAtomic: number;
  tokenMint: string;
  recipient: string;
  network: string;
  expiresAt: string;
  settled: boolean;
};

export type PaymentRequiredResponse = {
  error: "payment_required";
  message: string;
  challenge: PaymentChallenge;
};

export type VerifyPaymentInput = {
  requestId: string;
  txSignature: string;
  clientWallet: string;
};

class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

const liveApiEnabled = () => Boolean(API_BASE_URL);

const shouldFallbackToMock = (error: unknown) => {
  if (!USE_MOCK_FALLBACK) return false;
  if (!liveApiEnabled()) return true;
  if (error instanceof ApiError) {
    return [404, 405, 501, 502, 503, 504].includes(error.status);
  }
  return error instanceof TypeError;
};

const buildUrl = (path: string, query?: Record<string, string | undefined>) => {
  if (!API_BASE_URL) {
    throw new Error("VITE_API_BASE_URL is not configured");
  }

  const base = API_BASE_URL.endsWith("/") ? API_BASE_URL : `${API_BASE_URL}/`;
  const url = new URL(path, base);

  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value) {
        url.searchParams.set(key, value);
      }
    }
  }

  return url.toString();
};

const fetchJson = async <T>(
  path: string,
  init?: RequestInit,
  query?: Record<string, string | undefined>,
): Promise<T> => {
  const response = await fetch(buildUrl(path, query), {
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    ...init,
  });

  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const body = await response.json();
      message = body.error ?? body.message ?? message;
    } catch {
      // keep default status text
    }
    throw new ApiError(message, response.status);
  }

  return (await response.json()) as T;
};

const withMockFallback = async <T>(
  liveCall: () => Promise<T>,
  mockCall: () => T | Promise<T>,
): Promise<T> => {
  if (!liveApiEnabled()) {
    return await mockCall();
  }

  try {
    return await liveCall();
  } catch (error) {
    if (shouldFallbackToMock(error)) {
      return await mockCall();
    }
    throw error;
  }
};

export const api = {
  async getServers(): Promise<Server[]> {
    return withMockFallback(
      () => fetchJson<Server[]>(backendRoutes.servers),
      async () => {
        await delay();
        return servers;
      },
    );
  },
  async getServer(id: string): Promise<Server | undefined> {
    return withMockFallback(
      async () => {
        const allServers = await fetchJson<Server[]>(backendRoutes.servers);
        return allServers.find((server) => server.id === id);
      },
      async () => {
        await delay();
        return servers.find((server) => server.id === id);
      },
    );
  },
  async getTools(serverId?: string): Promise<ToolPricing[]> {
    return withMockFallback(
      async () => {
        if (serverId) {
          return fetchJson<ToolPricing[]>(backendRoutes.serverTools(serverId));
        }

        const allServers = await fetchJson<Server[]>(backendRoutes.servers);
        const results = await Promise.all(
          allServers.map((server) =>
            fetchJson<ToolPricing[]>(backendRoutes.serverTools(server.id)),
          ),
        );
        return results.flat();
      },
      async () => {
        await delay();
        return serverId ? tools.filter((tool) => tool.serverId === serverId) : tools;
      },
    );
  },
  async getReceipts(serverId?: string): Promise<Receipt[]> {
    return withMockFallback(
      () =>
        fetchJson<Receipt[]>(
          backendRoutes.dashboardReceipts,
          undefined,
          serverId ? { serverId } : undefined,
        ),
      async () => {
        await delay();
        return serverId ? receipts.filter((receipt) => receipt.serverId === serverId) : receipts;
      },
    );
  },
  async getRequests(opts?: {
    serverId?: string;
    status?: PaymentRequest["status"];
  }): Promise<PaymentRequest[]> {
    return withMockFallback(
      () =>
        fetchJson<PaymentRequest[]>(
          backendRoutes.requests,
          undefined,
          opts
            ? {
                serverId: opts.serverId,
                status: opts.status,
              }
            : undefined,
        ),
      async () => {
        await delay();
        let filteredRequests = requests;
        if (opts?.serverId) {
          filteredRequests = filteredRequests.filter(
            (request) => request.serverId === opts.serverId,
          );
        }
        if (opts?.status) {
          filteredRequests = filteredRequests.filter((request) => request.status === opts.status);
        }
        return filteredRequests;
      },
    );
  },
  async getRequest(id: string): Promise<PaymentRequest | undefined> {
    return withMockFallback(
      () => fetchJson<PaymentRequest>(backendRoutes.request(id)),
      async () => {
        await delay();
        return requests.find((request) => request.id === id);
      },
    );
  },
  async getDashboardSummary(): Promise<DashboardSummary> {
    return withMockFallback(
      () => fetchJson<DashboardSummary>(backendRoutes.dashboardSummary),
      async () => {
        await delay();
        return summary;
      },
    );
  },
  async invokeTool(serverId: string, payload: InvokeMcpToolInput) {
    return fetchJson<Record<string, unknown> | PaymentRequiredResponse>(
      backendRoutes.invokeMcp(serverId),
      {
        method: "POST",
        body: JSON.stringify(payload),
      },
    );
  },
  async getChallenge(requestId: string) {
    return fetchJson<PaymentChallenge>(backendRoutes.challenge(requestId));
  },
  async verifyPayment(payload: VerifyPaymentInput) {
    return fetchJson<{ requestId: string; txSignature: string; status: string }>(
      backendRoutes.verify,
      {
        method: "POST",
        body: JSON.stringify(payload),
      },
    );
  },
};

export const fmtUsdc = (n: number) => `${n.toFixed(n < 0.01 ? 4 : n < 1 ? 3 : 2)} USDC`;

export const truncate = (s: string, head = 4, tail = 4) =>
  s.length <= head + tail + 1 ? s : `${s.slice(0, head)}…${s.slice(-tail)}`;

export const fmtRelative = (iso: string): string => {
  const diff = Date.now() - new Date(iso).getTime();
  const m = Math.floor(diff / 60_000);
  if (m < 1) return "just now";
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
};
