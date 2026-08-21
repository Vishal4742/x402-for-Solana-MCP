package main

import (
	"crypto/rand"
	"math/big"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// base58Encode encodes data with the Bitcoin/Solana alphabet, preserving
// leading zero bytes as leading '1' characters.
func base58Encode(data []byte) string {
	value := new(big.Int).SetBytes(data)
	radix := big.NewInt(58)
	mod := new(big.Int)

	var encoded []byte
	for value.Sign() > 0 {
		value.DivMod(value, radix, mod)
		encoded = append(encoded, base58Alphabet[mod.Int64()])
	}
	for _, b := range data {
		if b != 0 {
			break
		}
		encoded = append(encoded, base58Alphabet[0])
	}
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}
	return string(encoded)
}

// newPaymentReference returns 32 random bytes base58-encoded, so the value is a
// valid Solana pubkey string that binds an on-chain transfer to one challenge.
func newPaymentReference() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base58Encode(buf[:]), nil
}
