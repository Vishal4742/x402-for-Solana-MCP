export type MCPInvokeRequest = {
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
  scheme: string;
  // Per-challenge Solana Pay style reference key (base58). Attach it as a
  // read-only non-signer account on the transfer so the verifier can bind
  // the on-chain payment to this specific challenge.
  reference: string;
  status: string;
  createdAt: string;
  expiresAt: string;
  settled: boolean;
  settledAt?: string;
};

export type PricingMeta = {
  scheme: string;
  amountAtomic: number;
  tokenMint: string;
  recipient: string;
  network: string;
  reference: string;
};

export type RetryMeta = {
  requestId: string;
  requestIdField: string;
  header: string;
};

export type PaymentRequiredResponse = {
  error: "payment_required";
  message: string;
  challenge: PaymentChallenge;
  pricing: PricingMeta;
  retry: RetryMeta;
};

export type VerifyPaymentRequest = {
  requestId: string;
  txSignature: string;
  clientWallet: string;
};

export type VerifyPaymentResponse = {
  requestId: string;
  txSignature: string;
  clientWallet: string;
  status: string;
  settledAt: string;
};

export type ExecutionResponse = {
  serverId: string;
  tool: string;
  status: string;
  requestId?: string;
  result: unknown;
};

export type Receipt = {
  id: string;
  requestId: string;
  serverId: string;
  toolName: string;
  amountUsdc: number;
  payerWallet: string;
  txSignature: string;
  blockTime: string;
  createdAt: string;
  responseHash: string;
};

export type PaymentProof = {
  txSignature: string;
  clientWallet: string;
};

export type PaymentPayer = (challenge: PaymentChallenge) => Promise<PaymentProof>;

export class PaymentRequiredError extends Error {
  challenge: PaymentChallenge;
  response: PaymentRequiredResponse;

  constructor(response: PaymentRequiredResponse) {
    super(response.message);
    this.name = "PaymentRequiredError";
    this.challenge = response.challenge;
    this.response = response;
  }
}

async function errorBodyText(response: Response): Promise<string> {
  try {
    const text = await response.text();
    return text.trim();
  } catch {
    return "";
  }
}

export class X402Client {
  constructor(private readonly baseUrl: string) {}

  async invokeTool(
    serverId: string,
    payload: MCPInvokeRequest,
    requestId?: string
  ): Promise<ExecutionResponse> {
    const response = await fetch(`${this.baseUrl}/mcp/${serverId}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(requestId ? { "X-Payment-Request-Id": requestId } : {})
      },
      body: JSON.stringify(payload)
    });

    if (response.status === 402) {
      const challenge = (await response.json()) as PaymentRequiredResponse;
      throw new PaymentRequiredError(challenge);
    }

    if (!response.ok) {
      const body = await errorBodyText(response);
      throw new Error(
        `invoke failed with status ${response.status}${body ? `: ${body}` : ""}`
      );
    }

    return (await response.json()) as ExecutionResponse;
  }

  async getChallenge(requestId: string): Promise<PaymentChallenge> {
    const response = await fetch(`${this.baseUrl}/v1/challenge/${requestId}`);
    if (!response.ok) {
      const body = await errorBodyText(response);
      throw new Error(
        `challenge lookup failed with status ${response.status}${body ? `: ${body}` : ""}`
      );
    }
    return (await response.json()) as PaymentChallenge;
  }

  async verifyPayment(payload: VerifyPaymentRequest): Promise<VerifyPaymentResponse> {
    const response = await fetch(`${this.baseUrl}/v1/verify`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify(payload)
    });

    if (!response.ok) {
      const body = await errorBodyText(response);
      throw new Error(
        `payment verification failed with status ${response.status}${body ? `: ${body}` : ""}`
      );
    }

    return (await response.json()) as VerifyPaymentResponse;
  }

  async getReceipt(requestId: string): Promise<Receipt> {
    const response = await fetch(`${this.baseUrl}/v1/receipts/${requestId}`);
    if (!response.ok) {
      const body = await errorBodyText(response);
      throw new Error(
        `receipt lookup failed with status ${response.status}${body ? `: ${body}` : ""}`
      );
    }
    return (await response.json()) as Receipt;
  }

  // Full buyer loop: invoke -> 402 -> pay via callback -> verify -> retry invoke.
  async invokeWithPayment(
    serverId: string,
    payload: MCPInvokeRequest,
    payer: PaymentPayer
  ): Promise<ExecutionResponse> {
    let challenge: PaymentChallenge;
    try {
      return await this.invokeTool(serverId, payload);
    } catch (error) {
      if (!(error instanceof PaymentRequiredError)) {
        throw error;
      }
      challenge = error.challenge;
    }

    const proof = await payer(challenge);
    await this.verifyPayment({
      requestId: challenge.requestId,
      txSignature: proof.txSignature,
      clientWallet: proof.clientWallet
    });

    return this.invokeTool(serverId, payload, challenge.requestId);
  }
}
