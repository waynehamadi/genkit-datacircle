package datacircle

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

func TestDefinitions(t *testing.T) {
	want := []string{"datacircle_list_apis", "datacircle_best_plans", "datacircle_search_plans", "datacircle_provider"}
	g := genkit.Init(context.Background())
	for _, set := range [][]ai.ToolRef{NewTools(nil), DefineTools(g, nil)} {
		for i, ref := range set {
			tl, ok := ref.(ai.Tool)
			if !ok || tl.Name() != want[i] {
				t.Fatalf("tool %d is %s, want the tool %s", i, ref.Name(), want[i])
			}
			schema, _ := json.Marshal(tl.Definition().InputSchema)
			for name, has := range map[string]string{
				"datacircle_best_plans":   `"required":["slug"]`,
				"datacircle_search_plans": `"sort":{"description":"score (best first, the default), provider (A to Z) or price ($ per 1,000, lowest first)","type":"string"}`,
				"datacircle_provider":     `"required":["slug"]`,
			} {
				if tl.Name() == name && !strings.Contains(string(schema), has) {
					t.Errorf("%s's input schema %s, want it to hold %s", name, schema, has)
				}
			}
			if strings.Contains(string(schema), `"required":["query"`) || strings.Contains(string(schema), `"limit"]`) {
				t.Errorf("%s requires an optional input: %s", tl.Name(), schema)
			}
		}
	}
	if tl := genkit.LookupTool(g, "datacircle_provider"); tl == nil {
		t.Error("DefineTools left datacircle_provider unregistered")
	}
}

func TestCalls(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		switch {
		case r.URL.Path == "/apis":
			w.Write([]byte(`[{"api":"Employee profile","slug":"linkedin-profile"},{"api":"Employee email","slug":"email-finder","plans":3}]`))
		case r.URL.Path == "/apis/email-finder/best":
			w.Write([]byte(`{"api":"Employee email","plans":[{"provider":"A","rank":1,"terms":"long"}]}`))
		case r.URL.Path == "/plans" && (r.URL.Query().Get("api") == "Bright Data" || r.URL.Query().Get("api") == "bright-data"):
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"api must be one of: Employee email"}`))
		case r.URL.Path == "/plans" && strings.Contains(r.URL.Query().Get("query"), "-"):
			w.Write([]byte(`{"total":0,"plans":[]}`))
		case r.URL.Path == "/plans":
			w.Write([]byte(`{"total":1,"plans":[{"provider":"Bright Data","rank":2}]}`))
		case r.URL.Path == "/providers/bright-data":
			w.Write([]byte(`{"provider":"Bright Data","apis":[{"api":"Employee profile","plans":[{"plan":"C"}]},{"api":"Employee email","rank":4,"plans":[{"plan":"B","rank":9},{"plan":"A","rank":4}]}]}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"no API has the slug nope"}`))
		}
	}))
	defer server.Close()
	cfg := &Config{BaseURL: server.URL}
	ctx := context.Background()
	for _, call := range []struct {
		tool  ai.Tool
		input map[string]any
		has   string
	}{
		{NewListAPIsTool(cfg), map[string]any{"query": "email"}, `"total":1`},
		{NewBestPlansTool(cfg), map[string]any{"slug": "Email Finder", "limit": 2}, `"provider":"A"`},
		{NewSearchPlansTool(cfg), map[string]any{"api": "Bright Data"}, `No API is named`},
		{NewSearchPlansTool(cfg), map[string]any{"api": "bright-data"}, `these are the plans whose name holds \"bright data\".","plans":[{"provider":"Bright Data"`},
		{NewSearchPlansTool(cfg), map[string]any{"query": "bright-data", "sort": "price"}, `"plans":[{"provider":"Bright Data"`},
		{NewProviderTool(cfg), map[string]any{"slug": "Bright Data"}, `"apis":[{"api":"Employee email","best":{"plan":"A"`},
		{NewProviderTool(cfg), map[string]any{"slug": "bright-data"}, `"ranked":1,"total":2`},
		{NewBestPlansTool(cfg), map[string]any{"slug": "nope"}, `{"error":"no API has the slug nope"}`},
	} {
		got, err := call.tool.RunRaw(ctx, call.input)
		out, _ := json.Marshal(got)
		if err != nil || !strings.Contains(string(out), call.has) {
			t.Errorf("%s(%v) = %s, %v; want it to hold %s", call.tool.Name(), call.input, out, err, call.has)
		}
		if strings.Contains(string(out), "terms") {
			t.Errorf("%s gave a plan's every key: %s", call.tool.Name(), out)
		}
	}
	if want := "/apis/email-finder/best?limit=2"; paths[1] != want {
		t.Errorf("called %s, want %s", paths[1], want)
	}
	if want := "/plans?query=bright+data&sort=price"; paths[8] != want {
		t.Errorf("called %s, want %s", paths[8], want)
	}
}
