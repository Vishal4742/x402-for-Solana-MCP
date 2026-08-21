import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { Keypair } from "@solana/web3.js";

// devnet-state.json lives in apps/example-client/, next to package.json. Never commit it.
export const STATE_PATH = fileURLToPath(new URL("../devnet-state.json", import.meta.url));

export type DevnetState = {
  network: string;
  rpcUrl: string;
  seller: { publicKey: string; secretKeyBase64: string };
  buyer: { publicKey: string; secretKeyBase64: string };
  // Absent until setup-devnet gets past the faucet and finishes minting.
  mint?: string;
  buyerAta?: string;
  sellerAta?: string;
  updatedAt: string;
};

export function loadState(): DevnetState | null {
  if (!existsSync(STATE_PATH)) {
    return null;
  }
  return JSON.parse(readFileSync(STATE_PATH, "utf8")) as DevnetState;
}

export function saveState(state: DevnetState): void {
  writeFileSync(STATE_PATH, JSON.stringify(state, null, 2) + "\n");
}

export function keypairFromBase64(secretKeyBase64: string): Keypair {
  return Keypair.fromSecretKey(Buffer.from(secretKeyBase64, "base64"));
}

export function keypairToBase64(keypair: Keypair): string {
  return Buffer.from(keypair.secretKey).toString("base64");
}
