# Agent Payments Protocol (AP2) & Pluggable Gateways

Hyperrr includes full support for the **Agent Payments Protocol (AP2)**—an open industry standard (maintained by the FIDO Alliance) that enables autonomous AI shopping agents to execute secure, pre-authorized transactions on behalf of users without real-time human intervention (no interactive OTPs or 3DS verification).

This guide explains how AP2 works in Hyperrr and details the execution flow using a concrete example of a customer buying **premium Alphonso mangoes** using the **Razorpay** provider.

---

## 1. How AP2 Works in Hyperrr

Hyperrr decouples the cryptographic verification of authorization rules (Mandates) from the actual payment gateways (Stripe, Razorpay, Mock). 

AP2 introduces two core digital contracts:
1. **The Mandate (SD-JWT-VC)**: Signed by the **User** (via device passkeys/biometrics) when deploying the agent. It defines spending limits, permitted merchants, and the agent's public key.
2. **The Agent Assertion (JWT)**: Signed by the **Agent** at checkout. It asserts specific transaction details (`order_id`, `amount`, `currency`) and matches them against the mandate.

```
       +---------------------------------------------+
       |               AI Agent (LLM)                |
       +---------------------------------------------+
             |                                 |
    1. Sign Assertion (ES256)        2. Submit verifyPaymentAp2()
             v                                 v
       +---------------------------------------------+
       |             Hyperrr Core API                |
       +---------------------------------------------+
             |                                 |
    3. Verify Mandate                4. Verify Constraints
    (User Public Key)                (Limit, Currency, Merchant)
             |                                 |
             +---------------+-----------------+
                             |
                             v
       +---------------------------------------------+
       |      Payment gateway (Stripe/Razorpay)      |
       +---------------------------------------------+
                             |
                    5. Capture & Charge
```

---

## 2. Walkthrough: Buying Alphonso Mangoes via Razorpay

### The Scenario
* **User**: Viraj
* **AI Agent**: ShoppingAssistant-9000
* **Store**: `mangofarms.in` (running Hyperrr + Razorpay)
* **Goal**: Buy "5kg of Alphonso Mangoes for under ₹2,500 INR"

### Step 1: User Issues & Signs the Mandate (SD-JWT-VC)
When Viraj deploys the AI agent, his browser/wallet generates a cryptographically signed **Agent Payment Mandate** (W3C Verifiable Credential using Selective Disclosure JWT). This mandate binds the agent's public key to the user's authorization and sets strict spending limits.

#### The Mandate Payload (Unsigned JWT)
```json
{
  "iss": "did:example:viraj123",            // The customer's identity
  "sub": "did:example:agent_sa9000",        // The agent's identity
  "iat": 1781418200,                        // Issued time
  "exp": 1784010200,                        // Expiration time (e.g. 2 weeks)
  "credentialSubject": {
    "agentKey": {                           // The Agent's public key (P-256)
      "kty": "EC",
      "crv": "P-256",
      "x": "W-z1vX5sQW...",
      "y": "b83OJ3D2..."
    },
    "constraints": {
      "max_amount": 2500.00,                // Strict limit of ₹2500
      "currency": "INR",                    // Restrict to Indian Rupees
      "merchant_allowlist": ["mangofarms.in"] // Only buy from this store
    }
  }
}
```
*Viraj signs this payload using his private key (stored securely in his passkey/device keychain). This outputs the signed **`mandate_jwt`**.*

### Step 2: Agent Selects Mangoes & Creates the Assertion
The AI agent crawls `mangofarms.in`, finds 5kg of premium Alphonso mangoes for **₹2,000 INR**, adds them to the cart, and obtains an order ID: `order_mango_99`.

To initiate checkout, the agent must prove it holds the mandate. It creates a signed **Agent Assertion** asserting the details of this specific transaction.

#### The Agent Assertion Payload (Unsigned JWT)
```json
{
  "order_id": "order_mango_99",
  "amount": 2000.00,
  "currency": "INR",
  "nonce": "secure_random_nonce_abc123",
  "iat": 1781418300
}
```
*The agent signs this payload using its private key (matching the `agentKey` declared in the mandate). This outputs the signed **`agent_assertion`**.*

### Step 3: AP2 Handshake at Shop's API
The agent submits the order along with the cryptographic proofs to the store's GraphQL API:

```graphql
mutation {
  verifyPaymentAp2(
    input: {
      orderId: "order_mango_99",
      provider: "razorpay",
      mandateJwt: "eyJhbGciOiJFUzI1NiI...", // Signed Mandate from Step 1
      agentAssertion: "eyJhbGciOiJFUzI1Ni...", // Signed Assertion from Step 2
      payload: {
        razorpay_payment_id: "pay_xyz789"  // Verified payment token
      }
    }
  ) {
    orderId
    status
    message
  }
}
```

### Step 4: Verification on Hyperrr Server
The Hyperrr backend handles the mutation via the `VerifyAP2` workflow step:

1. **Retrieve User Public Key**: The server queries the GORM database for Customer `viraj123`'s profile metadata and extracts his `ap2_public_key` (PEM).
2. **Verify User Mandate Signature**: The server verifies that the `mandate_jwt` signature was indeed created by Viraj's public key using Go's `crypto/ecdsa` standard library.
3. **Verify Constraints**: The server parses the constraints inside the verified mandate:
   * Is ₹2,000 <= Max ₹2,500? **Yes.**
   * Is currency `INR`? **Yes.**
   * Is merchant allowed? `mangofarms.in` matches the allowlist. **Yes.**
4. **Verify Agent Assertion**: The server extracts the `agentKey` from the verified mandate and checks the signature of the `agent_assertion` to prove that the agent itself authorized this specific ₹2,000 charge.

### Step 5: Capture Payment via Razorpay
Once the AP2 checks pass, Hyperrr moves to execute the payment via the registered **Razorpay Provider**:

1. **Create Razorpay Order**: Hyperrr calls Razorpay's `CreateIntent` API to register the purchase:
   ```go
   client.Order.Create(map[string]any{
       "amount":   200000, // Amount in paise (₹2,000.00)
       "currency": "INR",
       "receipt":  "order_mango_99",
   })
   ```
   *Razorpay returns a native order ID: `order_rzp_mango_123`.*
2. **Charge & Verify Payment**: Using the payment method token sent by the agent in the `payload`, the server calls:
   ```go
   verified, err := prov.VerifyPayment(ctx, payload)
   ```
   Razorpay performs HMAC-SHA256 signature verification to confirm the payment was successfully captured.
3. **Fulfillment**: 
   * The transaction in GORM changes status to `SUCCEEDED`.
   * The order status changes to `PAID`.
   * A `commerce.order.paid` event is published to the internal Event Fabric, notifying the inventory and shipping modules to dispatch the mangoes to Viraj's home.

---

## 3. Local Test Suite & Intercepted Mocking

To verify that these integrations behave correctly without calling live APIs or requiring live credentials, Hyperrr utilizes custom HTTP transports to intercept requests during unit tests:

1. **Stripe Mock Backend**: Set up inside `payments_test.go` using a custom `stripe.BackendImplementation` and `httptest.NewServer`.
2. **Razorpay Mock Backend**: Mocks the Razorpay SDK calls by redirecting base URL paths to a local `httptest.NewServer` instance.

To run the payment and AP2 verification test suite, run:
```bash
cd commerce
go test -v ./payments
```
