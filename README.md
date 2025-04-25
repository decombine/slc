# Decombine SLC
[![License Apache 2][License-Image]][License-Url] [![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg)](CODE_OF_CONDUCT.md)

[License-Url]: https://www.apache.org/licenses/LICENSE-2.0
[License-Image]: https://img.shields.io/badge/License-Apache2-blue.svg

## Contracts for cloud-native business

Create Decombine Smart Legal Contracts (SLC) with Go. Decombine SLC orchestrates and automates agreement and contractual 
business logic as cloud-native workflows.

SLC incorporates open source [Cloud Native Computing Foundation](https://www.cncf.io/) (CNCF) tooling such as:

- [Kubernetes](https://kubernetes.io/) for workload execution
- [NATS](https://nats.io/) for distributed event handling
- [CloudEvents](https://cloudevents.io/) for event schema
- [Flux](https://fluxcd.io/) for GitOps lifecycles and resourcing
- [Open Policy Agent](https://www.openpolicyagent.org/) for state and policy enforcement

Using SLC, you can create transparent, repeatable, and predictable business workflows that are ready for the most complex or 
demanding enterprise requirements.

Learn more about Decombine SLC at [decombine.com](https://decombine.com).

### Installation

The `slc` Go library can be installed with:

```bash
go get "github.com/decombine/slc@latest"
```

The `contract` CLI can be installed with:

```bash
go install "github.com/decombine/contract@latest"
```

## Use Cases

### Zero-trust infrastructure and services

SLC can be declaratively defined using templates or directly in Go. The SLC definition is a blueprint of the 
exact workloads that are triggered by the SLC and how. As long as the SLC definition is available, or the source code operating 
the SLC, parties have programmatic proof of the system.

### Complex, agentic multi-party business logic

SLC provide structure and transparency around business logic in a vendor-agnostic way using a combination of industry-leading 
open source tools. Furthermore, they can be networked with other SLC to form complex, zero-trust workflows which treat 
services and agents as first-class consumers.

## Overview

`slc` is an SDK to create Decombine Smart Legal Contracts (SLC). Decombine SLC performs programmatic contractual
execution in a standardized, templated, and declarative way. SLC are defined as a definition file that maps different
states and events with software actions to be executed. SLC are designed to be used in a [GitOps](https://www.gitops.tech/) workflow.

Decombine SLC were invented to offer an accessible, transparent, and cloud-native way for parties to form an agreement.
SLC ensure transparency about *what* will happen and *when*. In contrast to conventional agreements where a digitized
contract may be created and then all other processes are separate, an SLC instead integrates contract-related processes
into an end-to-end workflow.

Decombine SLC are intended to provide:

- Safer, predictable, and lower cost execution of agreements
- Increased transparency and understanding throughout the lifecycle of contract execution
- Greater accessibility to complex solutions by integrating with industry-leading open source

Decombine SLC can be run on conventional infrastructure and are currently under development for Kubernetes-native
workloads. Decombine SLC are not a replacement for natural language contracts, but instead, augment conventional contracts.

Decombine SLC can be self-hosted or run on a managed service such as [decombine.com](https://decombine.com).

Decombine SLC are defined as a JSON, YAML, or TOML definition that includes a State Configuration to create a [Unified Modeling
Language state machine](https://en.wikipedia.org/wiki/UML_state_machine). Each state in the state machine can be associated
with a set of actions to be executed on entry and exit of the state. Actions will eventually be any arbitrary code, but currently
development is focused on Kubernetes-native runtime.

Transitions between states are triggered by events. Events use the [CloudEvents](https://cloudevents.io/) specification.

## How to use

The `slc` package can be used to generate and validate the SLC definition and State Configuration. You can also use the
[contract CLI](https://github.com/decombine/contract) to generate and validate SLC in various formats (JSON, YAML, TOML).

SLC are designed and implemented declaratively to use [GitOps](https://www.gitops.tech/). The SLC definition, contractual text references, associated 
policies, and actions are stored in a Git repository. The Git repository is then synchronized with a runtime environment such as
Kubernetes. A remote service such as [Decombine](https://decombine.com) can be used to provide state synchronization, orchestration,
and event handling or `slc-controller` can be self-hosted.

The [slc-controller](https://github.com/decombine/slc-controller) can be installed on Kubernetes to schedule [SmartLegalContract]()
resources and schedule the workloads defined in the SLC definition.

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests.

## Versioning

Decombine SLC are currently under active development. `slc`, `contract`, and `slc-controller` are all in alpha and adhere to
[Semantic Versioning](https://semver.org/). The Decombine specification itself is versioned and will be updated as needed.

## Resources

- [Decombine.com](https://decombine.com)
- [Decombine SLC Specification]()
- [Decombine SLC Whitepaper]()

## License

This project is licensed under the Apache License, Version 2.0 - see the [LICENSE](LICENSE) file for details.