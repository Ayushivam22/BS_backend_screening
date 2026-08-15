# Setup, Architecture & Testing Guide
## Cashfree Payment Webhook Consumer in Go

> **Boost Score — Backend Engineering Assignment**  
> Focus: Webhooks, Type Safety, Security, Extensibility, Template-Driven Normalization

---

## Table of Contents
1. [Project Overview](#1-project-overview)
2. [Prerequisites & Zero-Dependency Setup](#2-prerequisites--zero-dependency-setup)
3. [Architecture & Design Approach](#3-architecture--design-approach)
4. [How to Run the Application](#4-how-to-run-the-application)
5. [How to Test the Application](#5-how-to-test-the-application)
   - [A. Automated Live HTTP Test Suite (13 Tests)](#a-automated-live-http-test-suite-13-tests)
   - [B. Unit & Integration Test Suite (22 Tests)](#b-unit--integration-test-suite-22-tests)
   - [C. Signature Generator Tool](#c-signature-generator-tool)
   - [D. Manual `curl` Testing](#d-manual-curl-testing)
6. [Answers to the 4 Operational Questions](#6-answers-to-the-4-operational-questions)
7. [Polymorphic Payment Method Handling](#7-polymorphic-payment-method-handling)
8. [Webhook Security & HMAC-SHA256 Verification](#8-webhook-security--hmac-sha256-verification)
9. [How to Add a New Payment Method (Extensibility)](#9-how-to-add-a-new-payment-method-extensibility)
10. [Automated Testing Checklist (Section 10 Compliance)](#10-automated-testing-checklist-section-10-compliance)
11. [Assumptions and Trade-Offs](#11-assumptions-and-trade-offs)

---

## 1. Project Overview

This project implements a production-minded HTTP service in Go that consumes Cashfree payment webhooks. It satisfies all requirements of the Boost Score Backend Engineering Assignment:
- **Polymorphic Payloads**: Parses varied `payment_method` payloads (`credit_card`, `upi`, `net_banking`) using deferred parsing (`json.RawMessage`) and dynamic registry lookup.
- **Generic Processing Pipeline**: Decouples transport (HTTP) from core business logic using an `EventProcessor` interface.
- **External Template-Driven Normalization**: Normalizes webhook events into a flat, consumer-friendly shape defined entirely by external JSON files in `./templates/` without recompilation.
- **Raw-Body HMAC-SHA256 Security**: Enforces cryptographic signature verification using the unaltered raw request bytes before JSON decoding.
- **Defensive Error Handling**: Rejects malformed JSON, forged signatures, missing headers, and unsupported events with appropriate HTTP status codes (`200`, `400`, `401`, `405`, `422`) without panicking.

---

## 2. Prerequisites & Zero-Dependency Setup

### Prerequisites
- **Go 1.18+** installed ([Download Go](https://golang.org/dl/))
- **Git**

### Zero External Dependencies
This project uses **100% standard library Go** (`net/http`, `crypto/hmac`, `crypto/sha256`, `encoding/json`, `encoding/base64`, `sync`, etc.). No third-party packages or external frameworks are required.

### Clone and Verify
```bash
# Navigate to the workspace
cd BS_backend_screening

# Verify Go module (no external dependencies to download)
go mod tidy
```

---

## 3. Architecture & Design Approach

The service follows a 5-layer decoupled pipeline:

```
+-----------------------------------------------------------------------------------+
|                                 HTTP Request                                      |
|            POST /webhook (Headers: x-webhook-signature, x-webhook-timestamp)     |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| 1. Security Middleware (security.Verifier)                                        |
|    - Reads exact raw request body bytes prior to JSON decoding                    |
|    - Computes HMAC-SHA256(secret, timestamp + rawBody)                            |
|    - Constant-time verification (crypto/hmac.Equal) against timing attacks        |
|    - Injects verified raw body into request context                               |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| 2. Webhook Handler (handlers.WebhookHandler)                                      |
|    - Validates POST method (rejects GET with 405)                                 |
|    - Retrieves raw body from context                                              |
|    - Delegates processing to pipeline.EventProcessor                              |
|    - Maps typed errors to HTTP response codes (200, 400, 401, 405, 422)           |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| 3. Generic Pipeline (pipeline.Pipeline)                                           |
|    - Stage 1: Validate payload non-empty                                          |
|    - Stage 2: Parse envelope & validate event type (SUCCESS / FAILED / DROPPED)   |
|    - Stage 3: Identify payment group (credit_card, upi, net_banking)              |
|    - Stage 4: Polymorphic dispatch via parser.ParserRegistry                      |
|    - Stage 5: Template normalization via engine.TemplateEngine                    |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| 4. Flat Normalized Event (models.NormalizedEvent)                                 |
|    - Emitted in HTTP 200 response and downstream application logs                 |
+-----------------------------------------------------------------------------------+
```

---

## 4. How to Run the Application

### Configuration (Environment Variables)
| Variable | Description | Default (if unset) |
|----------|-------------|--------------------|
| `PORT` | HTTP port the server listens on | `8080` |
| `TEMPLATES_DIR` | Path to external JSON template directory | `./templates` |
| `CASHFREE_WEBHOOK_SECRET` | Secret key used to verify HMAC signatures | `dev_webhook_secret_key_change_in_prod` |

### Start the Server
```bash
# Optional: Set custom environment variables
export PORT="8080"
export CASHFREE_WEBHOOK_SECRET="my_secure_webhook_secret"
export TEMPLATES_DIR="./templates"

# Run the application
go run main.go
```

**Startup Log Output:**
```
[INFO] Loaded templates successfully from './templates'
[INFO] Cashfree Webhook Consumer listening on port 8080 (POST http://localhost:8080/webhook)...
```

> **Operator Safety Feature**: At startup, `engine.NewEngine` validates every JSON file in `./templates/`. If a template is missing required fields or malformed, the server **fails fast immediately** (`log.Fatalf`), preventing misconfigured services from ever taking live traffic.

---

## 5. How to Test the Application

### A. Automated Live HTTP Test Suite (13 Tests)

With the server running (`go run main.go`), open a second terminal and run:

```bash
go run tools/testrunner/main.go
```

This sends real signed HTTP requests over the network to `http://localhost:8080` and verifies all functional and security scenarios:

```
╔═════════════════════════════════════════════════════════════════════════════╗
║     CASHFREE WEBHOOK ENDPOINT TEST SUITE (HMAC-SHA256 VERIFIED)             ║
║     Target Server: http://localhost:8080                                    ║
╚═════════════════════════════════════════════════════════════════════════════╝

✅ [01] Feature 1: Health Check (GET /)                         → HTTP 200 (Passed)
✅ [02] Feature 1: Valid Credit Card Webhook (HMAC verified)    → HTTP 200 (Passed)
✅ [03] Feature 2: Valid UPI Webhook (HMAC verified)            → HTTP 200 (Passed)
✅ [04] Feature 2: Valid Net Banking Webhook (HMAC verified)    → HTTP 200 (Passed)
✅ [05] Security: Missing Signature Header (Expect 401)         → HTTP 401 (Passed)
✅ [06] Security: Missing Timestamp Header (Expect 401)         → HTTP 401 (Passed)
✅ [07] Security: Forged Signature (Expect 401)                 → HTTP 401 (Passed)
✅ [08] Security: Tampered Body (Expect 401)                    → HTTP 401 (Passed)
✅ [09] Pipeline: Unsupported Event Type (Expect 422)           → HTTP 422 (Passed)
✅ [10] Pipeline: Unsupported Payment Group (Expect 422)        → HTTP 422 (Passed)
✅ [11] Pipeline: Empty Body (Expect 400)                       → HTTP 400 (Passed)
✅ [12] Pipeline: Malformed JSON (Expect 400)                   → HTTP 400 (Passed)
✅ [13] Transport: Method Not Allowed GET /webhook (Expect 405) → HTTP 405 (Passed)
───────────────────────────────────────────────────────────────────────────────
  SUMMARY: 13 Passed, 0 Failed (Total 13 Tests)
───────────────────────────────────────────────────────────────────────────────
```

---

### B. Unit & Integration Test Suite (22 Tests)

Run all unit, integration, and security tests across all Go packages:

```bash
go test -v ./...
```

**Packages Tested:**
- `engine/`: Template loading, startup validation, missing field fallback, dot-notation resolver.
- `handlers/`: HTTP status code mappings (`200`, `400`, `401`, `405`, `422`), context extraction.
- `parser/`: Polymorphic dispatch for Card, UPI, Net Banking, dynamic registration.
- `pipeline/`: Linear processing flow, unsupported events, unsupported payment groups, runtime extensibility.
- `security/`: Raw body HMAC computation, timing attack protection (`hmac.Equal`), tampered payload rejection, missing headers.

---

### C. Signature Generator Tool

Use [`tools/signtool/main.go`](file:///c:/Users/AKS/Desktop/WebDev/BS_backend_screening/tools/signtool/main.go) to compute exact HMAC signatures for any sample payload or manual testing:

```bash
# Compute signatures for all 3 sample files (Card, UPI, NetBanking):
go run tools/signtool/main.go -all

# Compute signature for a specific file:
go run tools/signtool/main.go -file samples/payment_success_card.json

# Compute signature with a custom secret key:
go run tools/signtool/main.go -secret "custom_secret" -file samples/payment_failed_upi.json

# Compute signature for an inline JSON string:
go run tools/signtool/main.go '{"type":"PAYMENT_SUCCESS_WEBHOOK",...}'
```

---

### D. Manual `curl` Testing

1. Generate matching headers for a payload:
   ```bash
   go run tools/signtool/main.go -file samples/payment_success_card.json
   ```
2. Copy the timestamp and signature into your `curl` command:
   ```bash
   curl -X POST http://localhost:8080/webhook \
     -H "Content-Type: application/json" \
     -H "x-webhook-timestamp: <GENERATED_TIMESTAMP>" \
     -H "x-webhook-signature: <GENERATED_SIGNATURE>" \
     -d @samples/payment_success_card.json
   ```

**Expected Response (`HTTP 200 OK`):**
```json
{
  "status": "success",
  "message": "Webhook PAYMENT_SUCCESS_WEBHOOK processed successfully",
  "data": {
    "event_type": "PAYMENT_SUCCESS_WEBHOOK",
    "event_time": "2023-08-14T10:00:00+05:30",
    "order_id": "order_card_101",
    "payment_id": "cf_card_999",
    "payment_group": "credit_card",
    "payment_status": "SUCCESS",
    "amount": 2500,
    "currency": "INR",
    "customer_email": "alice@example.com",
    "payment_details": {
      "card": {
        "channel": "link",
        "card_number": "XXXXXXXXXXXX4738",
        "card_network": "visa",
        "card_type": "credit_card",
        "card_bank_name": "HDFC Bank"
      }
    },
    "raw_mapped_fields": {
      "amount": 2500,
      "card_bank_name": "HDFC Bank",
      "card_network": "visa",
      "card_type": "credit_card",
      "channel": "link",
      "currency": "INR",
      "customer_email": "alice@example.com",
      "customer_name": "Alice Johnson",
      "event_time": "2023-08-14T10:00:00+05:30",
      "event_type": "PAYMENT_SUCCESS_WEBHOOK",
      "order_id": "order_card_101",
      "payment_group": "credit_card",
      "payment_id": "cf_card_999",
      "payment_status": "SUCCESS"
    }
  }
}
```

---

## 6. Answers to the 4 Operational Questions

*(As required in Section 5 of the Boost Score assignment specification)*

### 1. How does the service decide, at runtime, which template applies to an incoming webhook?
The template engine implements a 4-tier lookup hierarchy in [`engine/template.go`](file:///c:/Users/AKS/Desktop/WebDev/BS_backend_screening/engine/template.go#L128):
1. **Exact match**: `payment_group:EVENT_TYPE` (e.g. `credit_card:PAYMENT_SUCCESS_WEBHOOK`)
2. **Group wildcard**: `payment_group:*` (matches all events for that payment group)
3. **Event wildcard**: `*:EVENT_TYPE` (matches a specific event across all groups)
4. **Global fallback**: `*:*` (configured in `templates/default.json`)

### 2. What happens when a webhook arrives for a payment group that has no template definition?
The engine automatically routes the payload to the global fallback template (`templates/default.json`), which extracts all standard core fields (`order_id`, `payment_id`, `amount`, `customer_email`, etc.). If no fallback template is present, it returns an explicit error resulting in `HTTP 422 Unprocessable Entity`.

### 3. What happens when a template references a field that is absent from the webhook payload?
The dot-notation resolver [`resolvePath`](file:///c:/Users/AKS/Desktop/WebDev/BS_backend_screening/engine/template.go#L199) safely returns `nil`. The engine omits the key from `RawMappedFields` and assigns zero-values (`""` or `0.0`) to the typed struct without raising errors or panicking.

### 4. How would an operator — not a Go developer — add or change a template safely?
An operator simply adds or edits a `.json` file in `./templates/` and restarts the service. At boot time, [`engine.NewEngine`](file:///c:/Users/AKS/Desktop/WebDev/BS_backend_screening/engine/template.go#L54) validates all JSON files and required fields (`template_id`, `payment_group`, `mappings`). If a template is malformed or invalid, the service **fails fast at startup** (`log.Fatalf`) with an actionable error message, ensuring invalid configurations never process live traffic.

---

## 7. Polymorphic Payment Method Handling

Cashfree webhooks have a polymorphic structure where `data.payment.payment_method` has different fields based on `payment_group`:
- **`credit_card`**: `{ "card": { "card_number", "card_network", "card_type", "card_bank_name", ... } }`
- **`upi`**: `{ "upi": { "channel", "upi_id" } }`
- **`net_banking`**: `{ "netbanking": { "channel", "netbanking_bank_name" } }`

### Implementation Strategy:
1. Envelope struct [`models.WebhookEvent`](file:///c:/Users/AKS/Desktop/WebDev/BS_backend_screening/models/event.go) declares `PaymentMethod` as `json.RawMessage`, deferring parsing until `payment_group` is identified.
2. The [`parser.ParserRegistry`](file:///c:/Users/AKS/Desktop/WebDev/BS_backend_screening/parser/registry.go) looks up the parser function registered for that group.
3. Parses raw bytes directly into the typed struct (`*models.CardPayment`, `*models.UPIPayment`, `*models.NetBankingPayment`).
4. Type safety is preserved without large `switch` statements in application or transport layers.

---

## 8. Webhook Security & HMAC-SHA256 Verification

Cashfree computes signatures as:
$$\text{Signature} = \text{Base64}\Big(\text{HMAC-SHA256}\big(\text{Secret},\; \text{Timestamp} + \text{RawBody}\big)\Big)$$

### Security Enforcement in [`security/security.go`](file:///c:/Users/AKS/Desktop/WebDev/BS_backend_screening/security/security.go):
1. **Raw Body Preservation**: The middleware reads raw bytes from `r.Body` *before* JSON unmarshaling.
2. **Context Passing**: Restores `r.Body` via `io.NopCloser` and injects raw bytes into `r.Context()`.
3. **Timing Attack Protection**: Uses `crypto/hmac.Equal` for constant-time comparison.
4. **Cross-Platform Normalization**: Compares raw bytes, with LF/CRLF and compact JSON fallbacks to handle cross-platform line-ending variations gracefully.

---

## 9. How to Add a New Payment Method (Extensibility)

Adding a new payment group (e.g. `wallet` or `crypto_wallet`) requires **zero modifications** to existing pipeline code:

### Step 1: Define the typed model (`models/wallet.go`)
```go
package models

type WalletPayment struct {
    Wallet struct {
        Channel  string `json:"channel"`
        Provider string `json:"provider"`
        Phone    string `json:"phone"`
    } `json:"wallet"`
}

func (w *WalletPayment) GetPaymentGroup() string { return "wallet" }
```

### Step 2: Register the parser
```go
parser.Register("wallet", func(data []byte) (models.PaymentMethod, error) {
    var w models.WalletPayment
    err := json.Unmarshal(data, &w)
    return &w, err
})
```

### Step 3: Add template file (`templates/wallet.json`)
```json
{
  "template_id": "wallet_template",
  "payment_group": "wallet",
  "event_type": "*",
  "mappings": {
    "event_type": "type",
    "order_id": "data.order.order_id",
    "payment_id": "data.payment.cf_payment_id",
    "payment_group": "data.payment.payment_group",
    "amount": "data.payment.payment_amount",
    "wallet_provider": "data.payment.payment_method.wallet.provider"
  }
}
```

*This extensibility pattern is automated in [`pipeline/extensibility_test.go`](file:///c:/Users/AKS/Desktop/WebDev/BS_backend_screening/pipeline/extensibility_test.go).*

---

## 10. Automated Testing Checklist (Section 10 Compliance)

| Requirement (Section 10 of PDF) | Tested in Test Suite | Status |
|---------------------------------|----------------------|--------|
| `[x]` Card payment webhook | `TestPipeline_PaymentSuccess_Card`, `TestWebhookHandler_E2E_Success_Card` | ✅ PASS |
| `[x]` UPI payment webhook | `TestPipeline_PaymentFailed_UPI`, `TestWebhookHandler_E2E_Success_UPI` | ✅ PASS |
| `[x]` Net Banking payment webhook | `TestPipeline_PaymentUserDropped_NetBanking`, `TestWebhookHandler_E2E_Success_NetBanking` | ✅ PASS |
| `[x]` Payment success event | `TestPipeline_PaymentSuccess_Card` | ✅ PASS |
| `[x]` Payment failed event | `TestPipeline_PaymentFailed_UPI` | ✅ PASS |
| `[x]` User dropped event | `TestPipeline_PaymentUserDropped_NetBanking` | ✅ PASS |
| `[x]` Invalid JSON | `TestWebhookHandler_E2E_InvalidJSON` | ✅ PASS |
| `[x]` Invalid signature | `TestSecurityMiddleware_TamperedBody`, `TestWebhookHandler_E2E_InvalidSignature` | ✅ PASS |
| `[x]` Missing signature | `TestSecurityMiddleware_MissingHeaders` | ✅ PASS |
| `[x]` Unsupported payment group | `TestPipeline_UnsupportedPaymentGroup`, `TestWebhookHandler_E2E_UnsupportedGroup` | ✅ PASS |
| `[x]` Incorrect payment-method payload | `TestTemplateEngine_TransformWithMissingFields` | ✅ PASS |
| `[x]` Normalized event matches template definition | `TestPipeline_PaymentSuccess_Card`, `TestTemplateEngine_TransformWithMissingFields` | ✅ PASS |
| `[x]` Webhook for payment group with no template definition | `TestPipeline_UnsupportedPaymentGroup` | ✅ PASS |
| `[x]` Malformed template definition rejected at startup | `TestNewEngine_MalformedJSON`, `TestNewEngine_InvalidTemplateSchema` | ✅ PASS |
| `[x]` Extension test (new payment method + template) | `TestExtensibility_AddNewPaymentMethodWithoutCoreChanges` | ✅ PASS |

---

## 11. Assumptions and Trade-Offs

1. **In-Memory Sink**: Per assignment specifications, persistent storage (PostgreSQL/MongoDB) is omitted. Normalized events are returned in the HTTP 200 response and logged.
2. **Startup Template Loading**: Templates are read at boot time into an in-memory cache protected by `sync.RWMutex`. This provides sub-millisecond throughput over dynamic disk I/O on each request.
3. **Constant-Time Verification**: `crypto/hmac.Equal` is strictly enforced to prevent timing attacks.
