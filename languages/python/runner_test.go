package python

import (
	"io/fs"
	"strings"
	"testing"

	core "github.com/nexssp/lang/core"
)

func TestPythonRunnerProfilesAreGroupedAndLoadThroughRunnerFiles(t *testing.T) {
	files := core.RunnerFiles()
	for _, path := range []string{"runners/python.nflow", "runners/python2.nflow", "runners/python3.nflow"} {
		t.Run(path, func(t *testing.T) {
			raw, err := fs.ReadFile(files, path)
			if err != nil {
				t.Fatalf("read virtual runner profile: %v", err)
			}
			spec, err := core.ParseRunner(string(raw))
			if err != nil {
				t.Fatalf("ParseRunner: %v", err)
			}
			if spec.ID != "python" {
				t.Errorf("profile ID = %q; want python", spec.ID)
			}
			if !strings.Contains(spec.FileTemplate, "json.load(sys.stdin)") {
				t.Errorf("runner %q has no embedded JSON starter template", path)
			}
		})
	}
}

func TestPythonRunnerProfileDeclaresExpectedAliasesAndRecipes(t *testing.T) {
	data, err := fs.ReadFile(core.RunnerFiles(), "runners/python.nflow")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := core.ParseRunner(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Aliases) != 2 || spec.Aliases[0] != "py" || spec.Aliases[1] != "python" {
		t.Fatalf("aliases = %v; want [py python]", spec.Aliases)
	}
	if len(spec.Install.Windows) == 0 || len(spec.Install.Darwin) == 0 || len(spec.Install.Linux) == 0 {
		t.Fatalf("Python runner install recipes = %+v; want all supported OSes", spec.Install)
	}
}
