import {
  Connection,
  Keypair,
  LAMPORTS_PER_SOL,
  PublicKey
} from "@solana/web3.js";
import {
  createMint,
  getMint,
  getOrCreateAssociatedTokenAccount,
  mintTo
} from "@solana/spl-token";
import {
  STATE_PATH,
  keypairFromBase64,
  keypairToBase64,
  loadState,
  saveState
} from "./state.js";

// Creates everything the devnet demo needs: seller + buyer keypairs, SOL from the
// faucet, a 6-decimal SPL mint standing in for USDC, ATAs, and 100 tokens for the buyer.

const rpcUrl = process.env.SOLANA_RPC_URL ?? "https://api.devnet.solana.com";
const MINT_DECIMALS = 6;
const BUYER_TOKENS_ATOMIC = 100_000_000n; // 100 tokens at 6 decimals

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function ensureSol(
  connection: Connection,
  pubkey: PublicKey,
  label: string,
  minSol: number,
  airdropSol: number
): Promise<void> {
  if (minSol <= 0) {
    return;
  }
  const balance = await connection.getBalance(pubkey);
  if (balance >= minSol * LAMPORTS_PER_SOL) {
    console.log(`  ${label} already has ${(balance / LAMPORTS_PER_SOL).toFixed(3)} SOL, skipping airdrop`);
    return;
  }

  const attempts = 5;
  for (let attempt = 1; attempt <= attempts; attempt++) {
    try {
      console.log(`  requesting ${airdropSol} SOL airdrop for ${label} (attempt ${attempt}/${attempts})...`);
      const signature = await connection.requestAirdrop(pubkey, airdropSol * LAMPORTS_PER_SOL);
      const latest = await connection.getLatestBlockhash("confirmed");
      await connection.confirmTransaction({ signature, ...latest }, "confirmed");
      console.log(`  airdrop confirmed: ${signature}`);
      return;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      console.log(`  airdrop attempt failed: ${message}`);
      if (attempt < attempts) {
        await sleep(attempt * 3000);
      }
    }
  }
  throw new Error(
    `devnet faucet refused to fund ${label} (${pubkey.toBase58()}) after ${attempts} attempts. ` +
      `The faucet rate-limits aggressively; wait a few minutes or fund it manually at https://faucet.solana.com`
  );
}

async function main(): Promise<void> {
  console.log(`setup-devnet: rpc=${rpcUrl}`);
  const connection = new Connection(rpcUrl, "confirmed");

  const existing = loadState();
  const seller = existing ? keypairFromBase64(existing.seller.secretKeyBase64) : Keypair.generate();
  const buyer = existing ? keypairFromBase64(existing.buyer.secretKeyBase64) : Keypair.generate();
  console.log(existing ? `loaded keypairs from ${STATE_PATH}` : "generated fresh seller + buyer keypairs");
  console.log(`  seller: ${seller.publicKey.toBase58()}`);
  console.log(`  buyer:  ${buyer.publicKey.toBase58()}`);

  if (!existing) {
    // Persist keypairs before the faucet call: if the airdrop is rate-limited, reruns keep
    // the same buyer address and it can be funded manually at https://faucet.solana.com
    saveState({
      network: "devnet",
      rpcUrl,
      seller: { publicKey: seller.publicKey.toBase58(), secretKeyBase64: keypairToBase64(seller) },
      buyer: { publicKey: buyer.publicKey.toBase58(), secretKeyBase64: keypairToBase64(buyer) },
      updatedAt: new Date().toISOString()
    });
    console.log(`  keypairs saved to ${STATE_PATH}`);
  }

  // The buyer pays every fee in the demo (mint + both ATAs + transfers cost well under
  // 0.01 SOL), and the seller never signs, so one small airdrop is all the faucet owes us.
  const airdropSol = Number(process.env.AIRDROP_SOL ?? "1");
  await ensureSol(connection, buyer.publicKey, "buyer", 0.02, airdropSol);

  let mint: PublicKey | null = null;
  if (existing?.mint) {
    try {
      const info = await getMint(connection, new PublicKey(existing.mint));
      if (info.decimals === MINT_DECIMALS && info.mintAuthority?.equals(buyer.publicKey)) {
        mint = info.address;
        console.log(`reusing existing mint ${mint.toBase58()}`);
      }
    } catch {
      // mint from a previous run is gone (devnet reset) — create a new one
    }
  }
  if (!mint) {
    mint = await createMint(connection, buyer, buyer.publicKey, null, MINT_DECIMALS);
    console.log(`created mint ${mint.toBase58()} (${MINT_DECIMALS} decimals)`);
  }

  const buyerAta = await getOrCreateAssociatedTokenAccount(connection, buyer, mint, buyer.publicKey);
  const sellerAta = await getOrCreateAssociatedTokenAccount(connection, buyer, mint, seller.publicKey);
  console.log(`  buyer ATA:  ${buyerAta.address.toBase58()}`);
  console.log(`  seller ATA: ${sellerAta.address.toBase58()}`);

  await mintTo(connection, buyer, mint, buyerAta.address, buyer, BUYER_TOKENS_ATOMIC);
  console.log(`minted 100 tokens (${BUYER_TOKENS_ATOMIC} atomic) to buyer`);

  saveState({
    network: "devnet",
    rpcUrl,
    seller: { publicKey: seller.publicKey.toBase58(), secretKeyBase64: keypairToBase64(seller) },
    buyer: { publicKey: buyer.publicKey.toBase58(), secretKeyBase64: keypairToBase64(buyer) },
    mint: mint.toBase58(),
    buyerAta: buyerAta.address.toBase58(),
    sellerAta: sellerAta.address.toBase58(),
    updatedAt: new Date().toISOString()
  });
  console.log(`state written to ${STATE_PATH}`);

  console.log("\nStart the gateway and verifier with these env vars:");
  console.log(`SELLER_WALLET=${seller.publicKey.toBase58()}`);
  console.log(`SOLANA_USDC_MINT=${mint.toBase58()}`);
  console.log(`SOLANA_VERIFY_MODE=rpc`);
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(`setup-devnet failed: ${error instanceof Error ? error.message : String(error)}`);
    process.exit(1);
  });
