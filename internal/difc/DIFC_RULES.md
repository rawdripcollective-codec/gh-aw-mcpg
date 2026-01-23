# Decentralized Information Flow Control (DIFC) Rules

This document explains the DIFC labeling system used in this package. All implementations and tests MUST follow these rules.

## Overview

DIFC uses two types of labels to control information flow:

- **Secrecy Labels**: Control who can read confidential information
- **Integrity Labels**: Control who can modify trusted resources

Both agents and resources have secrecy and integrity labels. Labels are sets of tags (strings).

## Core Rules

### Notation

- `A.secrecy` = Agent's secrecy label (set of tags)
- `A.integrity` = Agent's integrity label (set of tags)
- `R.secrecy` = Resource's secrecy label (set of tags)
- `R.integrity` = Resource's integrity label (set of tags)
- `⊇` means "is a superset of" (contains all elements of)

### Read Access Rules

For an agent to **READ** a resource:

1. **Secrecy Check**: `A.secrecy ⊇ R.secrecy`
   - Agent must have clearance for all secrecy tags on the resource
   - *Example*: To read a `{secret, confidential}` document, agent must have at least `{secret, confidential}` in its secrecy label

2. **Integrity Check**: `R.integrity ⊇ A.integrity`
   - Resource must be at least as trustworthy as the agent requires
   - *Example*: If agent requires `{verified}` integrity, resource must have at least `{verified}`

### Write Access Rules

For an agent to **WRITE** to a resource:

1. **Secrecy Check**: `R.secrecy ⊇ A.secrecy`
   - Resource must accept all agent's secrecy tags (no information leak)
   - *Example*: Agent with `{secret}` cannot write to a `{}` (public) resource

2. **Integrity Check**: `A.integrity ⊇ R.integrity`
   - Agent must be at least as trustworthy as resource requires
   - *Example*: To write to a `{production}` resource, agent must have at least `{production}` integrity

### Read-Write Access

For read-write operations, BOTH read AND write rules must be satisfied.

## Key Examples

### Example 1: Secret Agent Cannot Write to Public Resource

```
Agent:    secrecy={secret}, integrity={}
Resource: secrecy={}, integrity={}

Write Check:
  Secrecy: R.secrecy ⊇ A.secrecy → {} ⊇ {secret} → FALSE
  Result: DENIED (would leak secret information to public)
```

### Example 2: High-Integrity Agent Cannot Read Low-Integrity Resource

```
Agent:    secrecy={}, integrity={trusted, verified}
Resource: secrecy={}, integrity={}

Read Check:
  Integrity: R.integrity ⊇ A.integrity → {} ⊇ {trusted, verified} → FALSE
  Result: DENIED (resource is not trustworthy enough for agent)
```

### Example 3: Successful Read of Secret Document

```
Agent:    secrecy={secret, confidential}, integrity={}
Resource: secrecy={secret}, integrity={}

Read Check:
  Secrecy: A.secrecy ⊇ R.secrecy → {secret, confidential} ⊇ {secret} → TRUE
  Integrity: R.integrity ⊇ A.integrity → {} ⊇ {} → TRUE
  Result: ALLOWED
```

### Example 4: Successful Write to Production Database

```
Agent:    secrecy={}, integrity={production, verified}
Resource: secrecy={}, integrity={production}

Write Check:
  Secrecy: R.secrecy ⊇ A.secrecy → {} ⊇ {} → TRUE
  Integrity: A.integrity ⊇ R.integrity → {production, verified} ⊇ {production} → TRUE
  Result: ALLOWED
```

## Public Internet Analogy

The **public internet** has empty labels: `secrecy={}, integrity={}`.

- An agent with `secrecy={secret}` **CANNOT write** to the public internet
  - Because: `{} ⊇ {secret}` is FALSE (would leak secrets)

- An agent with `integrity={trusted}` **CANNOT read** from the public internet
  - Because: `{} ⊇ {trusted}` is FALSE (source not trusted enough)

## Implementation Notes

### CheckFlow Function

The `CheckFlow(target)` method checks if `source ⊆ target` (source has no tags that target doesn't have):

```go
// SecrecyLabel.CheckFlow(target) returns true if all tags in source are also in target
// i.e., source ⊆ target (source is a subset of target)
func (source *SecrecyLabel) CheckFlow(target *SecrecyLabel) (bool, []Tag)

// IntegrityLabel.CheckFlow(target) returns true if all tags in source are also in target
// i.e., source ⊆ target (source is a subset of target)
func (source *IntegrityLabel) CheckFlow(target *IntegrityLabel) (bool, []Tag)
```

**CRITICAL**: To check `A ⊇ B` (A contains all of B), call `B.CheckFlow(A)`.

### Evaluator Functions

The evaluator uses these `CheckFlow` calls to implement the DIFC rules:

```go
// For READ access:
//   Secrecy:   A.secrecy ⊇ R.secrecy   → resource.Secrecy.CheckFlow(agentSecrecy)
//   Integrity: R.integrity ⊇ A.integrity → resource.Integrity.CheckFlow(agentIntegrity)

// For WRITE access:
//   Secrecy:   R.secrecy ⊇ A.secrecy   → agentSecrecy.CheckFlow(&resource.Secrecy)
//   Integrity: A.integrity ⊇ R.integrity → agentIntegrity.CheckFlow(&resource.Integrity)
```

**Remember**: `X.CheckFlow(Y)` returns true when `X ⊆ Y` (all tags in X are in Y).
So to check `A ⊇ B`, call `B.CheckFlow(A)`.

## Testing Guidelines

When writing tests:

1. Empty labels `{}` represent public/untrusted resources
2. To test secrecy violations, give the agent secrecy tags the resource lacks
3. To test integrity violations, give the agent integrity tags the resource lacks
4. For reads: agent needs clearance (secrecy), resource needs trust (integrity)
5. For writes: resource needs to accept secrets (secrecy), agent needs trust (integrity)
