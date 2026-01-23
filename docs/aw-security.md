## Security Model

Agentic Workflows (AW) adopts a layered approach that combines substrate-enforced isolation, declarative specification, and staged execution. Each layer enforces distinct security properties under different assumptions and constrains the impact of failures above it.

### Threat Model

We consider an adversary that may compromise untrusted user-level components, e.g., containers, and causes them to behave arbitrarily within the privileges granted to them. The adversary may attempt to:

- Access or corrupt the memory or state of other components
- Communicate over unintended channels
- Abuse legitimate channels to perform unintended actions
- Confuse higher-level control logic by deviating from expected workflows

We assume the adversary does not compromise the underlying hardware or cryptographic primitives. Attacks exploiting side channels and covert channels are also out of scope.

---

### Layer 1: Substrate-Level Trust

AWs run on a GitHub Actions runner virtual machine (VM) and trust Actions' hardware and kernel-level enforcement mechanisms, including the CPU, MMU, kernel, and container runtime. AW also relies on two privileged containers: (1) a network firewall that is trusted to configure connectivity for other components via `iptables`, and (2) an MCP Gateway that is trusted to configure and spawn isolated containers, e.g., local MCP servers. Collectively, the substrate level ensures memory isolation between components, CPU and resource isolation, mediation of privileged operations and system calls, and explicit, kernel-enforced communication boundaries. These guarantees hold even if an untrusted user-level component is fully compromised and executes arbitrary code. Trust violations at the substrate level require vulnerabilities in the firewall, MCP Gateway, container runtime, kernel, hypervisor, or hardware. If this layer fails, higher-level security guarantees may not hold.

---

### Layer 2: Configuration-Level Trust

AW trusts declarative configuration artifacts, e.g., Action steps, network-firewall policies, MCP server configurations, and the toolchains that interpret them to correctly instantiate system structure and connectivity. The configuration level constrains which components are loaded, how components are connected, which communication channels are permitted, and what component privileges are assigned. Externally minted authentication tokens, e.g., agent API keys and GitHub access tokens, are a critical configuration input and are treated as imported capabilities that bound components' external effects; declarative configuration controls their distribution, e.g., which tokens are loaded into which containers. Security violations arise due to misconfigurations, overly permissive specifications, and limitations of the declarative model. This layer defines what components exist and how they communicate, but it does not constrain how components use those channels over time.

---

### Layer 3: Plan-Level Trust

AW additionally relies on plan-level trust to constrain component behavior over time. At this layer, the trusted compiler decomposes a workflow into stages. For each stage, the plan specifies (1) which components are active and their permissions, (2) the data produced by the stage, and (3) how that data may be consumed by subsequent stages. In particular, plan-level trust ensures that important external side effects are explicit and undergo thorough vetting.

A primary instantiation of plan-level trust is the **SafeOutputs** subsystem. SafeOutputs is a set of trusted components that operations on external state. An agent can interact with read-only MCP servers, e.g., the GitHub MCP server, but externalized writes, such as creating GitHub pull requests, are buffered as artifacts by SafeOutputs rather than applied immediately. When the agent finishes, SafeOutput's buffered artifacts can be processed by a deterministic sequence of filters and analyses defined by configuration. These checks can include structural constraints (e.g., limiting the number of pull requests), policy enforcement, and automated sanitization to ensure that sensitive information such as authentication tokens are not exported. These filtered and transformed artifacts are passed to a subsequent stage in which they are externalized.

Violations at of the planning layer arise from incorrect plan construction, incomplete or overly permissive stage definitions, or errors in the enforcement of plan transitions. This layer does not protect against failures of substrate-level isolation or mis-allocation of permissions at credential-minting or configuration time. However, it limits the blast radius of a compromised component to the stage in which it is active and its influence the artifacts passed to the next stage.

---

