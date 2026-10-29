# Nexss Lang

<small>Nexss Flow 🌀 ready</small>

`lang` is a reusable Go runtime for language runners plus a Nexss Flow adapter. The public [`core/`](core/) package owns runner profiles, aliases, interpreter discovery and installation, source materialization, and raw-byte process execution. [`nexssflow/`](nexssflow/README.md) contains the Flow-specific integration: directives, AST nodes, bundle registration, preflight, subprocess execution, and stream mapping.

Use the public runtime from `github.com/nexssp/lang/core`. Language-specific implementation and assets belong in their own packages: Python in `languages/python/`, Go in `languages/golang/`, and JavaScript, Rust, TypeScript, POSIX shell, Bash, WSL, and PowerShell in their matching directories under `languages/`. Each language owns its runner profile and any source template under `assets/`; `contract/` remains language-neutral, `cmd/nexss-lang/` contains the CLI, and `nexssflow/` contains Flow integration. `core.RunnerFiles()` exposes language assets under virtual `runners/` paths used by Flow's `@require` bundle.

## Using the core

Aliases resolve without checking whether an interpreter is installed. Runner profiles define stdin encoding and stdout decoding; `Execute` returns raw streams so the host retains control of process and serialization policy.

```go
package example

import (
    "context"

    "github.com/nexssp/lang/core"
)

func run(ctx context.Context, source string, input any) (any, error) {
    program, err := core.ProgramForTag("py3", source)
    if err != nil {
        return nil, err
    }
    language, err := core.Lookup(program.ID, program.Version)
    if err != nil {
        return nil, err
    }
    stdin, err := language.Encode(input)
    if err != nil {
        return nil, err
    }
    ctx = core.WithAutoInstallDisabled(ctx)
    stdout, _, err := core.Execute(ctx, program, stdin, nil)
    if err != nil {
        return nil, err
    }
    return language.Decode(stdout)
}
```

`Preflight{}` checks interpreter availability by default. Installing during preflight requires an explicitly supplied `Ensure`. `Execute` and `Resolve` can use the production installer when a profile declares an installation recipe; `WithAutoInstallDisabled` disables that behavior for the request. Run only code and installation recipes you trust.

Hosts can inject process execution through `Executor` / `ExecutorFunc` and register profiles through `RegisterRunnerSpec` or `RegisterDefine`. Runner files remain NFlow declarations, shell profiles remain explicit, and source templates live with their language profiles. The core uses Kernel `xerr` classification for its errors. Python-specific source parsing, contract inspection, runner assets, and examples are grouped in [`languages/python/`](languages/python/README.md); Flow-specific integration remains in [`nexssflow/`](nexssflow/README.md).

## Inspect Python contracts and generate NFlow schemas

The `nexss-lang` command statically inspects annotated Python dataclasses and `TypedDict` declarations, then maps them into the shared language-neutral model in `contract/`. Inspection uses Python's standard-library AST parser; it does not import or execute the inspected module. Python 3.8 or newer is required.

```sh
go run ./cmd/nexss-lang inspect models.py
go run ./cmd/nexss-lang inspect --class Request models.py
go run ./cmd/nexss-lang generate --class Request --output request.nflow models.py
```

The generated NFlow file contains `@schema` declarations and a commented pipeline example. Requiredness remains explicit in the inspection JSON; the generator does not use Flow's `validate:"required"` because that rule also rejects zero values such as `0` and `false`. Python 3.8+ must be installed as `python3`, or selected with `--python`. Current extraction covers top-level dataclasses and `TypedDict`; it does not yet resolve inherited fields or framework-specific model metadata.

## Reusable source-signature extraction

The language-neutral [`extract/`](extract/) package exposes the multi-language source parser used by packaging tools. Its API preserves the `Snippets`, `FuncCount`, and `Content` result shape expected by srcpack; it is separate from Python's dataclass/`TypedDict` contract inspector above.

```go
import "github.com/nexssp/lang/extract"

result, err := extract.Parse("service.py", source, "python", extract.Options{
    ExportedOnly: true,
    StripComments: true,
})
```

The callable signature is `func Parse(path string, src []byte, lang string, opts Options) (Result, error)`. The dispatcher currently supports Go, PHP, Python, Rust, JavaScript, TypeScript, JSX, TSX, Svelte, Vue, and Zig; Flow/NFlow documents pass through unchanged. Each extractor implementation lives in its matching package under `languages/` (`golang`, `php`, `python`, `rust`, `javascript`, `typescript`, `svelte`, `vue`, or `zig`). `Options` also supports indentation/blank-line cleanup, head/tail truncation, and signature-only output; `ProcessContent` and `Truncate` are available for callers that need the same text handling independently.

## Shells on Windows

Shell environments are explicit rather than inferred. `sh` selects POSIX `sh`, `bash` selects Bash, `wsl` selects Bash through WSL, and `ps`, `powershell`, or `pwsh` selects PowerShell (preferring `pwsh`). WSL is opt-in: on Windows, preflight verifies that `wsl.exe` can start a registered Bash distribution with `wslpath`; it does not treat a bare launcher as sufficient. The runner uses `wslpath -u` to convert the script path according to the distro's mount configuration instead of hardcoding `/mnt/c`.

## Requiring the Flow adapter

Use the existing external `@require` workflow to load the adapter. After publishing a version, use the adapter package path and pin a released tag (replace `vX.Y.Z` with an existing version):

```nflow
@require github.com/nexssp/lang/nexssflow vX.Y.Z
```

For local development, point to the repository directory relative to the `.nflow` file:

```nflow
@require ../nexssp-lang
```

Flow resolves the `nexssflow/` package and maps its suffix to bundle ID `lang`. The adapter registers its factory and compiler contributions through the existing Flow bundle contract; no core Flow changes or new `@require` mechanism are needed.

Preprocessing, directives, keywords, AST nodes, preflight, and stream mapping stay in `nexssflow/`. The reusable core does not depend on Flow's parser or compiler.

## Verification

The focused checks for the public runtime and language packages are:

```sh
go test ./core ./languages/python ./nexssflow ./extract ./languages/golang/... ./languages/javascript/... ./languages/rust/... ./languages/typescript/... ./languages/shell/... ./languages/bash/... ./languages/wsl/... ./languages/powershell/... ./languages/php/... ./languages/svelte/... ./languages/vue/... ./languages/zig/...
go vet ./core ./languages/python ./nexssflow ./extract ./languages/golang/... ./languages/javascript/... ./languages/rust/... ./languages/typescript/... ./languages/shell/... ./languages/bash/... ./languages/wsl/... ./languages/powershell/... ./languages/php/... ./languages/svelte/... ./languages/vue/... ./languages/zig/...
```
