# Example Buyer Client

This app will demonstrate the buyer flow using `@x402/sdk`.

Required demo flow:

1. call a paid tool
2. receive `402 Payment Required`
3. pay on Solana devnet
4. submit verification
5. retry the tool call with the settled request id
6. show the result and receipt

This app should stay minimal and scriptable.

