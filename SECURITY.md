# Security policy

## Supported versions

cerne is young and ships often. Only the [latest release](https://github.com/henriquesakata/cerne/releases/latest) receives security fixes; upgrade before reporting.

## Reporting a vulnerability

Report it privately through a [GitHub private vulnerability report](https://github.com/henriquesakata/cerne/security/advisories/new). Do not open a public issue, discussion or pull request for it.

Include what you can:

- `cerne --version` and your operating system;
- what an attacker can do, and what they need to do it;
- the steps or a proof of concept that reproduces it.

You will get an acknowledgement within 7 days. Once a fix is ready, it ships in a release and the advisory is published, crediting you unless you ask otherwise.

## Scope

cerne runs on your machine and talks only to the places you configure (see [Where your data goes](README.md#where-your-data-goes)). The most relevant reports are:

- the local web UI (`cerne ui`) being reachable or abusable from outside the machine, or from another site in your browser;
- credentials for a backend, a forge or an LLM provider leaking into logs, documents, the event log or the index;
- a crafted backend repository, document or import tree making cerne write outside the project or run code;
- the release binaries or install scripts being tampered with. Every release binary carries a provenance attestation: `gh attestation verify <file> --repo henriquesakata/cerne`.

Prompt injection through documents an AI agent reads is a property of the agent, not of cerne, unless cerne itself acts on the injected text.
