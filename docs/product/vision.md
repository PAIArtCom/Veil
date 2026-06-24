# Vision

**Status:** Accepted

## Mission

Let people use AI coding agents without leaking secrets or PII to model providers.

Veil is the **de-identification layer for the LLM era**: it removes sensitive data
from traffic on its way to an LLM and restores it on the way back — deterministically,
reversibly, and locally — so the productivity of AI tooling no longer requires accepting
data-exfiltration risk.

## Why now

Agentic coding tools (Claude Code, Codex, Copilot, Cursor, …) have made it normal to
stream code, configuration, and shell context to third-party models. Secrets and personal
data ride along by default. The current options are bad: ban the tools (lose the
productivity) or allow them (accept the leak). Regulation (GDPR, the EU AI Act, HIPAA,
financial rules) is tightening the squeeze.

## What we believe

- **The threat boundary is the network egress to the LLM — not the local machine.** The
  user's own secrets already live on the user's machine. We only need to stop them
  crossing to the provider. This keeps the design small and honest.
- **Privacy must be invisible to be adopted.** No perceptible latency, no broken tool
  calls, no busted prompt caches. If it slows the agent down, developers turn it off.
- **The privacy layer should be a horizontal capability, embedded everywhere — not yet
  another gateway.** Every gateway wants it; none want to build it.
- **Reversible, deterministic masking is the unlock.** Irreversible redaction is safer
  but breaks agents; reversibility (done with a deterministic, type-aware token) is what
  makes agentic tool-use and prompt caching survive de-identification.

## Non-goals

- We are not a model provider, an AI gateway competitor, or a DLP suite for general
  network traffic.
- We do not try to protect against a compromised local machine — that is outside the
  threat model ([details](../architecture/threat-model.md)).

See the [strategy](strategy.md) for how this becomes a product, and the
[roadmap](roadmap.md) for sequencing.
