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
  expiresAt: string;
  settled: boolean;
};

export type PaymentRequiredResponse = {
  error: "payment_required";
  message: string;
  challenge: PaymentChallenge;
};

export type VerifyPaymentRequest = {
  requestId: string;
  txSignature: string;
  clientWallet: string;
};

export class PaymentRequiredError extends Error {
  challenge: PaymentChallenge;

  constructor(response: PaymentRequiredResponse) {
    super(response.message);
    this.name = "PaymentRequiredError";
    this.challenge = response.challenge;
  }
}

export class X402Client {
  constructor(private readonly baseUrl: string) {}

  async invokeTool(serverId: string, payload: MCPInvokeRequest, requestId?: string) {
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
      throw new Error(`invoke failed with status ${response.status}`);
    }

    return response.json();
  }

  async getChallenge(requestId: string) {
    const response = await fetch(`${this.baseUrl}/v1/challenge/${requestId}`);
    if (!response.ok) {
      throw new Error(`challenge lookup failed with status ${response.status}`);
    }
    return (await response.json()) as PaymentChallenge;
  }

  async verifyPayment(payload: VerifyPaymentRequest) {
    const response = await fetch(`${this.baseUrl}/v1/verify`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify(payload)
    });

    if (!response.ok) {
      throw new Error(`payment verification failed with status ${response.status}`);
    }

    return response.json();
  }
}

