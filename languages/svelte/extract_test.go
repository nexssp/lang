package svelte

import (
	"strings"
	"testing"
)

func TestParseSvelteSnippets(t *testing.T) {
	svelteCode := []byte(`<script lang="ts">
export let title: string = "Dashboard";

export function refreshData(): void {
    console.log("fetching...");
}

function internalHelper() {}
</script>

<HeaderNav title={title} />
<Sidebar isOpen={true} />
<div>
    <h1>{title}</h1>
    <MetricCard metric="cpu" />
</div>
`)

	snippets, funcCount := parseSvelte(svelteCode, true)

	joined := strings.Join(snippets, "\n")
	if !strings.Contains(joined, "refreshData") {
		t.Errorf("expected refreshData function signature from <script>")
	}
	if strings.Contains(joined, "internalHelper") {
		t.Errorf("did not expect unexported internalHelper in exported_only")
	}
	if !strings.Contains(joined, "<HeaderNav title />") && !strings.Contains(joined, "<HeaderNav") {
		t.Errorf("expected HeaderNav custom component tag, got:\n%s", joined)
	}
	if funcCount == 0 {
		t.Errorf("expected function count > 0")
	}
}
