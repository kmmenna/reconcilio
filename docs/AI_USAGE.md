# AI in development

Codex was the only AI tool I used to develop Reconcilio.

## Planning, implementation, and review

1. I used Matt Pocock’s `grill-with-docs` to discuss the product idea, features, business rules, architecture, and project structure with Codex. We captured the domain vocabulary in the glossary and recorded architectural decisions in ADRs. [Skill](https://github.com/mattpocock/skills/blob/main/skills/engineering/grill-with-docs/SKILL.md) · [Glossary](../GLOSSARY.md) · [ADRs](adr/)
2. I used `to-spec` to turn those discussions into specifications, then `to-tickets` to break each specification into implementation tickets and record their dependencies. Both specifications and tickets were tracked in the project’s GitHub Issues. [to-spec](https://github.com/mattpocock/skills/blob/main/skills/engineering/to-spec/SKILL.md) · [to-tickets](https://github.com/mattpocock/skills/blob/main/skills/engineering/to-tickets/SKILL.md) · [Reconcilio issues](https://github.com/kmmenna/reconcilio/issues) · [MVP specification](https://github.com/kmmenna/reconcilio/issues/1)
3. I assigned each ticket to a Codex agent for implementation and used agents for a first review pass against the specification and repository conventions. I performed the final review of the resulting changes.

## Backend and frontend guidance

- **Modern Go Guidelines** guided backend implementation. I used it to help Codex avoid falling back to older patterns when the project’s Go version supports a simpler idiom. Its CLI selects guidance for the version declared in `go.mod`. [JetBrains documentation](https://www.jetbrains.com/help/go/modern-go-guidelines.html)
  - `sync.WaitGroup.Go` (Go 1.25) starts a tracked goroutine and handles the `Add` and `Done` calls that older code wires up manually. [WaitGroup guideline](https://github.com/JetBrains/go-modern-guidelines/blob/main/FEATURES.md#sync_waitgroup_go)
  - `errors.AsType[T]` (Go 1.26) returns the typed error and a match flag directly. The older `errors.As` pattern requires declaring the target variable and passing its address. [Error matching guideline](https://github.com/JetBrains/go-modern-guidelines/blob/main/FEATURES.md#errors_as_type)
- I used **Impeccable** to review and polish the analyst workspace, keep layouts consistent across screens, and avoid the generic look Codex can produce by default. [Impeccable design guidance](https://github.com/pbakaus/impeccable#why)

  - Simplify nested cards so related information is clearly grouped.
  - Use consistent spacing so related controls are easy to scan.
  - Keep the color palette consistent across screens, avoid decorative gradients, and check that text and controls have adequate contrast against their backgrounds. [Color guidance](https://impeccable.style/docs/colorize/)
