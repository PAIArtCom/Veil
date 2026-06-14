# Threat Model

**Status:** Accepted

## What we defend against

**Sensitive data (secrets, credentials, PII) leaking to a third-party LLM provider via
the network egress of an AI coding tool.** That is the entire job.

## Trust boundary

- **Trusted:** the user's local machine. Secrets already live there (`.env` files,
  configs, shell history, source). OpenCloak does not expand where secrets exist.
- **Untrusted:** any network egress that crosses to a third party — primarily the LLM
  provider API, and also **remote MCP / remote tools** (a separate egress channel, handled
  later; see [roadmap](../product/roadmap.md)).

The precise boundary is *local vs. any network egress*. In the common case that equals
*local vs. the LLM*; it diverges only for remote MCP/remote tools.

## Out of scope (explicit non-goals)

- A **compromised local machine** or a malicious local process. An attacker who can read
  OpenCloak's process memory can already read the user's `.env`, environment, and files —
  the reverse-map is not a new attack surface relative to that.
- General-purpose network DLP unrelated to LLM traffic.
- Defending against the model *retaining* what it legitimately needs — we ensure it only
  ever receives tokens for sensitive values, not real ones.

## Security invariants (the contract)

These must hold for every change touching detection, masking, restore, or egress:

1. **Two transformation points only.** Mask on egress to the LLM; restore on ingress from
   the LLM. Nothing local is altered. ([Redaction model](../concepts/redaction-model.md).)
2. **Fail-closed.** If the engine errors or is uncertain, **block** the request rather
   than forward plaintext. Never fail open.
3. **Localhost-only binding.** The standalone proxy binds `127.0.0.1` only. A redaction
   proxy reachable off-host with no auth would be an open relay.
4. **Credential pass-through.** The engine never holds or needs provider credentials; the
   transport forwards the tool's own auth header unchanged.
   ([ADR-0004](decisions/0004-auth-pass-through.md).)
5. **The reverse map is local.** Token→value mapping stays in process memory (optionally a
   local cache); it is never transmitted. ([ADR-0005](decisions/0005-global-in-memory-scope.md).)
6. **Tokens are not invertible without the local key.** `id = HMAC(value, local_key)`; an
   observer of tokens alone cannot recover values. (The real protection is that values
   never leave; this is defense in depth.)

## Residual risks (acknowledged)

- **Orphan tokens.** If the model mangles a token (splits, re-encodes), restore may miss
  it and a `CLK_…` literal could land in output. Mitigations: an identifier-safe token
  form that survives in code without breaking syntax, and an egress scan that flags
  residual tokens. ([Token spec](../concepts/token-spec.md).)
- **Detection false negatives.** L1 patterns cannot catch unstructured PII; that is what
  the optional L2 layer is for. Coverage is a tunable, documented trade-off — never a
  silent guarantee.
- **Local key/map at rest.** In-memory by default. If a local cache is enabled, it holds
  the user's own secrets on the user's own disk (within the trust boundary); hardening
  (no-swap, OS keychain) is a later option, not an MVP requirement.
- **Prompt-cache attribution metadata.** Some providers embed a hash of original content
  in attribution headers; deterministic masking keeps caches warm but integrators should
  be aware of provider-side analytics headers.
