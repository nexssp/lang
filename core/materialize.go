package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nexssp/kernel/xerr"
)

// materialize writes the source to a per-flow temp dir and returns its path.
//
// Contract:
//   - One temp dir per LangBlock invocation. A fresh dir keeps concurrent
//     and retrying executions from clobbering each other.
//   - The caller invokes cleanupMaterialized on return, including when the
//     interpreter exits with an error or the context is canceled.
//   - Cleanup on SIGKILL is best-effort and not attempted. The temp dir
//     lives under os.TempDir(); the OS rotates it. This is the documented
//     trade-off.
//
// The file is named <id><ext> (e.g. "python.py") so the interpreter sees
// the expected extension. Some interpreters (rustc) refuse to compile
// files without the right extension; others (go run) infer from content.
func materialize(ctx context.Context, lang *Language, source string) (dir, path string, err error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", "", ctxErr
	}
	dir, err = os.MkdirTemp("", "nflow-lang-")
	if err != nil {
		return "", "", xerr.Internal(fmt.Sprintf("lang.%s: create temporary source directory", lang.ID), err)
	}

	name := lang.ID + lang.Ext
	path = filepath.Join(dir, name)

	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", "", xerr.Internal(fmt.Sprintf("lang.%s: write temporary source file", lang.ID), err)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		_ = os.RemoveAll(dir)
		return "", "", ctxErr
	}

	return dir, path, nil
}

// cleanupMaterialized is idempotent. Calling it with an empty string is a
// no-op.
func cleanupMaterialized(dir string) {
	if dir == "" {
		return
	}
	_ = os.RemoveAll(dir)
}
