package bench

import (
	"strings"
	"testing"
)

func TestParseCodexEvents(t *testing.T) {
	data := []byte("{\"type\":\"item.completed\",\"item\":{\"type\":\"mcp_tool_call\"}}\n" +
		"{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":120,\"cached_input_tokens\":40,\"output_tokens\":10}}\n")
	usage, calls, completed := parseCodexEvents(data)
	if !completed || calls != 1 || usage.InputTokens != 120 || usage.CachedInputTokens != 40 || usage.OutputTokens != 10 {
		t.Fatalf("wrong event parse: %+v, %d, %t", usage, calls, completed)
	}
}

func TestReportDoesNotClaimGeneralImprovement(t *testing.T) {
	report := RenderReport(Suite{})
	if !strings.Contains(report, "do not generalize") || !strings.Contains(report, "not statistically significant") {
		t.Fatal("report omitted study limitations")
	}
}

func TestCodexEnvironmentExcludesTypeSafeKey(t *testing.T) {
	got := withoutTypeSafeKey([]string{"PATH=/usr/bin", "TYPESAFE_API_KEY=secret", "GOWORK=off"})
	if len(got) != 2 || got[0] != "PATH=/usr/bin" || got[1] != "GOWORK=off" {
		t.Fatalf("unsafe environment: %#v", got)
	}
}
