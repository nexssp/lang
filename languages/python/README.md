# Python language package

Python-specific implementation lives here: the Flow block scanner, safe AST contract inspector, source parser, Python adapter examples, and Python runner profiles/templates under `assets/`. The language-neutral model lives in `contract/`; this package maps Python annotations into that model. Shared runner discovery, execution, installation, and asset loading live in the public `core/` package. Host code should import `github.com/nexssp/lang/core` for the shared runtime.

The contract inspector recognizes top-level dataclasses and `TypedDict` classes with annotated fields. It uses Python's standard-library `ast` module and does not import or execute the inspected source. Python 3.8 or newer is required. Supported annotations map to neutral string, integer, number, boolean, array, object, or any types. Unknown custom classes are represented as objects; unions with multiple non-null types and heterogeneous fixed tuples become `any` (or arrays of `any`). It does not yet resolve inherited fields or third-party model metadata. This contract-inspection API is distinct from `python.ExtractSignatures` and the language-neutral `github.com/nexssp/lang/extract` parser, which emits source snippets and function counts for srcpack-style packaging.

From the module root:

```sh
go run ./cmd/nexss-lang inspect path/to/models.py
go run ./cmd/nexss-lang inspect --class Request path/to/models.py
go run ./cmd/nexss-lang generate --class Request --output request.nflow path/to/models.py
```

`inspect` prints JSON contracts, including source-level requiredness. `generate` writes `@schema` declarations and a commented pipeline example, so the generated file is a safe starter rather than an executable workflow. Flow's `validate:"required"` rule also rejects valid zero values, so the generator intentionally does not emit that tag; source-requiredness stays in the inspect JSON and is called out in the generated file. Python 3.8+ must be available as `python3`; pass `--python` to select another interpreter executable.
