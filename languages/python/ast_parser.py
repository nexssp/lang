"""Read Python contracts with ast.parse only; never import or execute the input."""
import ast
import json
import sys


def short_name(node):
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Attribute):
        return node.attr
    return ""


def decorator_name(node):
    if isinstance(node, ast.Call):
        node = node.func
    return short_name(node)


def is_wrapper(node, names):
    return isinstance(node, ast.Subscript) and short_name(node.value) in names


def required_override(node):
    if is_wrapper(node, {"Required"}):
        return True
    if is_wrapper(node, {"NotRequired"}):
        return False
    return None


def is_dataclass(node):
    return any(decorator_name(item) == "dataclass" for item in node.decorator_list)


def is_typed_dict(node):
    return any(short_name(base) == "TypedDict" for base in node.bases)


def is_classvar(node):
    wrapped = node.value if isinstance(node, ast.Subscript) else node
    return short_name(wrapped) in {"ClassVar", "InitVar"}


def is_non_init_field(value):
    if not isinstance(value, ast.Call) or short_name(value.func) != "field":
        return False
    return any(
        keyword.arg == "init"
        and isinstance(keyword.value, ast.Constant)
        and keyword.value.value is False
        for keyword in value.keywords
    )


def dataclass_default_is_required(value):
    if value is None:
        return True
    if not isinstance(value, ast.Call) or short_name(value.func) != "field":
        return False
    return not any(keyword.arg in {"default", "default_factory"} for keyword in value.keywords)


def inspect_class(node, source):
    dataclass = is_dataclass(node)
    typed_dict = is_typed_dict(node)
    if not dataclass and not typed_dict:
        return None

    total = True
    for keyword in node.keywords:
        if keyword.arg == "total" and isinstance(keyword.value, ast.Constant):
            total = bool(keyword.value.value)

    fields = []
    for statement in node.body:
        if not isinstance(statement, ast.AnnAssign) or not isinstance(statement.target, ast.Name):
            continue
        if is_classvar(statement.annotation):
            continue
        if dataclass and is_non_init_field(statement.value):
            continue

        annotation = ast.get_source_segment(source, statement.annotation)
        if annotation is None:
            continue
        override = required_override(statement.annotation)
        if typed_dict:
            required = total if override is None else override
        else:
            required = dataclass_default_is_required(statement.value)
            if override is not None:
                required = override
        fields.append({
            "name": statement.target.id,
            "annotation": annotation,
            "required": required,
        })

    if not fields:
        return None
    return {
        "name": node.name,
        "description": ast.get_docstring(node) or "",
        "fields": fields,
    }


source = sys.stdin.read()
try:
    tree = ast.parse(source, filename="<input>")
except SyntaxError as error:
    print(f"Python syntax error at {error.lineno}:{error.offset}: {error.msg}", file=sys.stderr)
    sys.exit(2)

contracts = []
for statement in tree.body:
    if isinstance(statement, ast.ClassDef):
        result = inspect_class(statement, source)
        if result is not None:
            contracts.append(result)

json.dump(contracts, sys.stdout, separators=(",", ":"))
