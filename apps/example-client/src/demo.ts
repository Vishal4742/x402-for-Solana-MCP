import { randomBytes } from "node:crypto";
import {
  Connection,
  PublicKey,
  Transaction,
  sendAndConfirmTransaction
} from "@solana/web3.js";
import { createTransferInstruction, getAssociatedTokenAddressSync } from "@solana/spl-token";
import {
  PaymentRequiredError,
  X402Client,
  type PaymentChallenge,
  type PaymentProof
} from "@x402/sdk";
import { keypairFromBase64, loadState, type DevnetState } from "./state.js";

// Phase 3 acceptance demo: free call, paid call with a real (or mock) payment,
// receipt lookup, and a replay-rejection negative test.

const gatewayUrl = process.env.GATEWAY_URL ?? "http://localhost:8080";
const serverId = process.env.SERVER_ID ?? "srv_01HX3K";
const payMode = process.env.PAY_MODE ?? "devnet";

function step(n: number, title: string): void {
  console.log(`\n=== Step ${n}: ${title} ===`);
}

function print(value: unknown): void {
  console.log(JSON.stringify(value, null, 2));
}

function requireDevnetState(): DevnetState {
  const state = loadState();
  if (!state) {
    throw new Error(
      "devnet-state.json not found — run `pnpm example:setup` (setup-devnet) first, " +
        "then start the gateway/verifier with the env vars it prints"
    );
  }
  return state;
}

async function payOnDevnet(challenge: PaymentChallenge): Promise<PaymentProof> {
  const state = requireDevnetState();
  if (!state.mint) {
    throw new Error(
      "devnet-state.json has keypairs but no mint — setup-devnet did not finish (faucet?). Re-run `pnpm example:setup`"
    );
  }
  if (challenge.tokenMint !== state.mint) {
    throw new Error(
      `challenge asks for mint ${challenge.tokenMint} but devnet-state.json has ${state.mint}. ` +
        `Restart the gateway with SOLANA_USDC_MINT=${state.mint} and SELLER_WALLET=${state.seller.publicKey}`
    );
  }

  const connection = new Connection(process.env.SOLANA_RPC_URL ?? state.rpcUrl, "confirmed");
  const buyer = keypairFromBase64(state.buyer.secretKeyBase64);
  const mint = new PublicKey(challenge.tokenMint);
  const source = getAssociatedTokenAddressSync(mint, buyer.publicKey);
  const destination = getAssociatedTokenAddressSync(mint, new PublicKey(challenge.recipient));

  console.log(
    `  paying ${challenge.amountAtomic} atomic units of ${challenge.tokenMint} to ${challenge.recipient}...`
  );
  const ix = createTransferInstruction(
    source,
    destination,
    buyer.publicKey,
    BigInt(challenge.amountAtomic)
  );
  // Bind the transfer to this challenge: the reference rides along as a
  // read-only non-signer key so it shows up in the tx accountKeys.
  ix.keys.push({ pubkey: new PublicKey(challenge.reference), isSigner: false, isWritable: false });
  const tx = new Transaction().add(ix);
  const signature = await sendAndConfirmTransaction(connection, tx, [buyer], {
    commitment: "confirmed"
  });
  console.log(`  transfer confirmed: ${signature}`);
  return { txSignature: signature, clientWallet: buyer.publicKey.toBase58() };
}

async function payMock(challenge: PaymentChallenge): Promise<PaymentProof> {
  const state = loadState();
  const wallet = state?.buyer.publicKey ?? "MockBuyerWallet";
  const signature = `mock-sig-${randomBytes(16).toString("hex")}`;
  console.log(`  mock payment for ${challenge.amountAtomic} atomic units: ${signature}`);
  return { txSignature: signature, clientWallet: wallet };
}

async function main(): Promise<void> {
  console.log(`x402 demo: gateway=${gatewayUrl} server=${serverId} payMode=${payMode}`);
  if (payMode !== "devnet" && payMode !== "mock") {
    throw new Error(`PAY_MODE must be "devnet" or "mock", got "${payMode}"`);
  }

  const client = new X402Client(gatewayUrl);

  step(1, "call the free tool (ping)");
  const pingResult = await client.invokeTool(serverId, { tool: "ping", input: { hello: "x402" } });
  print(pingResult);

  step(2, "call premium.search — pay the 402 challenge, verify, retry");
  let paidChallenge: PaymentChallenge | undefined;
  let paidProof: PaymentProof | undefined;
  const payer = async (challenge: PaymentChallenge): Promise<PaymentProof> => {
    paidChallenge = challenge;
    console.log(`  received challenge ${challenge.requestId} for ${challenge.toolName}`);
    paidProof = payMode === "devnet" ? await payOnDevnet(challenge) : await payMock(challenge);
    return paidProof;
  };
  const searchResult = await client.invokeWithPayment(
    serverId,
    { tool: "premium.search", input: { query: "solana mcp" } },
    payer
  );

  step(3, "execution result from the gateway");
  print(searchResult);

  if (!paidChallenge || !paidProof) {
    throw new Error("payer callback never ran — was premium.search misconfigured as free?");
  }

  step(4, "fetch the receipt");
  const receipt = await client.getReceipt(paidChallenge.requestId);
  print(receipt);

  step(5, "negative test: reuse the same tx signature against a new challenge");
  // Same tool (premium.search) so the fresh challenge has the same price as the
  // paid tx — the verifier gets past the amount check and reaches a reuse guard
  // instead of rejecting on underpayment, which would fake a passing test.
  // Two guards can catch the reuse, both correct:
  //   - rpc mode: the old transfer carries the old challenge's reference, so it
  //     fails reference binding ("not bound to this payment request").
  //   - mock mode: no reference is checked, so it falls through to the replay
  //     guard on the already-spent signature ("transaction already used").
  let freshChallenge: PaymentChallenge;
  try {
    await client.invokeTool(serverId, { tool: "premium.search", input: { query: "replay me" } });
    throw new Error("premium.search executed without payment — expected a 402 challenge");
  } catch (error) {
    if (!(error instanceof PaymentRequiredError)) {
      throw error;
    }
    freshChallenge = error.challenge;
    console.log(`  got fresh challenge ${freshChallenge.requestId} for premium.search`);
  }
  try {
    await client.verifyPayment({
      requestId: freshChallenge.requestId,
      txSignature: paidProof.txSignature,
      clientWallet: paidProof.clientWallet
    });
    console.error("FAIL: reused signature was accepted — reuse protection is broken");
    process.exit(1);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (!/already used|not bound to this payment request/i.test(message)) {
      // Anything else (underpayment, rpc_error, network, fetch failure) is a
      // real failure — do not pass it off as a successful negative test.
      throw new Error(`reuse was rejected for the wrong reason: ${message}`);
    }
    console.log(`  signature reuse correctly rejected: ${message}`);
  }

  console.log("\ndemo complete: free call, paid call, receipt, and reuse rejection all worked");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(`\ndemo failed: ${error instanceof Error ? error.message : String(error)}`);
    process.exit(1);
  });
