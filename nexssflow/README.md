# Nexss Flow adapter

The adapter is the Go package `github.com/nexssp/lang/nexssflow`. Add it through Flow's existing `@require` workflow and pin a released tag (replace `vX.Y.Z` with an existing version):

```nflow
@require github.com/nexssp/lang/nexssflow vX.Y.Z
```

For a local checkout, point to the repository directory relative to the `.nflow` file, for example `@require ../nexssp-lang`. Flow resolves the `nexssflow/` package, whose factory exposes bundle ID `lang`; no change to the existing `@require` mechanism is required.

The adapter owns bundle registration, language and file-keyword syntax, lowering to Flow AST nodes, preflight integration, and mapping Flow values to runner stdin/stdout. The `lang` root package owns runner profiles and interpreter execution.

Runner profiles are exported as source modules. They use Flow's `@description` metadata and `@symbols runner` for a local, checked copy of the canonical runner ID. Starter templates are regular package-embedded source assets: Flow's `@embed` contract exposes typed asset references, not raw source strings.

Existing adapter fixtures are embedded from `nflows/`.
