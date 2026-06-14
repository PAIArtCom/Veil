# Open-Core Boundary

**Status:** Accepted

The single most consequential business decision in an open-core product is *what is open
versus paid*. Get it wrong and you either give away the reason to pay, or starve the
community that drives adoption. This document pins the line.

## The principle

> **Individual value is open. Organizational control is paid.**

If a capability makes a single developer safer on their own machine, it belongs in
**OpenCloak** (open). If a capability lets an *organization* govern, configure, and audit
many developers, it belongs in **Cloakia** (commercial).

## Feature placement

| Capability | OpenCloak (open) | Cloakia (paid) |
|---|---|---|
| Detection engine (L1 patterns; L2 NER when it lands) | ✅ | |
| Deterministic reversible tokenization | ✅ | |
| Mask / restore (incl. streaming) | ✅ | |
| Per-provider adapters (Claude Code, Codex, …) | ✅ | |
| Standalone local proxy | ✅ | |
| Embeddable SDK + HTTP/gRPC service | ✅ | |
| Local, file-based configuration & rule sets | ✅ | |
| Centralized policy authoring & **push to a fleet** | | ✅ |
| Cross-developer **audit & compliance dashboards** | | ✅ |
| SSO / RBAC / team management | | ✅ |
| Enterprise entity dictionaries managed centrally | | ✅ |
| SIEM / log export, reporting | | ✅ |

## Placement rules (for future features)

When a new feature is proposed, ask in order:

1. **Does it protect one developer on one machine?** → Open.
2. **Does it require an account, a server, or visibility across people?** → Paid.
3. **Would withholding it cripple individual usefulness?** → Then it must be open, even if
   tempting to gate.

## The two failure modes we are avoiding

- **Too much in open** → no reason to buy Cloakia. Mitigation: organizational *control and
  visibility* (push, audit, RBAC) is reserved for Cloakia from day one.
- **Too little in open** → no community, no adoption, no validation of product-market fit,
  no moat. Mitigation: the entire *engine* and every *single-developer* workflow is open
  and genuinely complete on its own.

## The Cloakia trust paradox

Selling to a CISO requires audit logs — but audit *metadata* ("developer X sent 47 AWS
keys from the payments repo") is itself sensitive, and over-collecting it re-creates the
leak OpenCloak exists to prevent. Therefore **audit-data minimization is a day-one design
constraint** of Cloakia, not an afterthought. The trust concern scales with how much
leaves the machine; the cleanest commercial posture is the most local one.
