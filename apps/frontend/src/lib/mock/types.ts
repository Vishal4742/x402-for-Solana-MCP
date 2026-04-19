// Type definitions aligned with the Go/Chi backend entities.
// Frontend-only mock layer — wire up to real endpoints later via src/lib/api.ts.

export type RequestStatus = "challenged" | "paid" | "verified" | "executed" | "failed";

export type FailureReason =
  | "insufficient_funds"
  | "signature_invalid"
  | "timeout"
  | "tool_error"
  | "verification_failed";

export interface Server {
  id: string;
  name: string;
  endpoint: string;
  payoutWallet: string; // base58 Solana address
  webhookUrl: string;
  network: "devnet" | "mainnet-beta";
  createdAt: string;
  apiKey: string;
}

export interface ToolPricing {
  id: string;
  serverId: string;
  toolName: string;
  description: string;
  priceUsdc: number; // 0 = free
  enabled: boolean;
  callsLast24h: number;
}

export interface TimelineEvent {
  status: RequestStatus;
  timestamp: string;
  payload?: Record<string, unknown>;
  signatureHash?: string;
  durationMs?: number;
}

export interface PaymentRequest {
  id: string;
  serverId: string;
  toolName: string;
  payerWallet: string;
  amountUsdc: number;
  status: RequestStatus;
  failureReason?: FailureReason;
  txSignature?: string;
  createdAt: string;
  settledAt?: string;
  timeline: TimelineEvent[];
  rawRequest: Record<string, unknown>;
  rawResponse?: Record<string, unknown>;
}

export interface Receipt {
  id: string;
  requestId: string;
  serverId: string;
  toolName: string;
  amountUsdc: number;
  payerWallet: string;
  txSignature: string;
  blockTime: string;
  createdAt: string;
}

export interface DashboardSummary {
  totalRevenueUsdc: number;
  paidRequests: number;
  failedVerifications: number;
  avgSettlementMs: number;
  paidToolCount: number;
  freeToolCount: number;
  revenueDelta7d: number; // pct
  paidDelta7d: number;
}
