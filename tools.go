package datacircle

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// ListAPIsInput is datacircle_list_apis' input.
type ListAPIsInput struct {
	Query string `json:"query,omitempty" jsonschema_description:"Words of the API's name or slug, like email or linkedin profile: the APIs that hold the most of them come first. The sheet's order when left out"`
	Limit int    `json:"limit,omitempty" jsonschema_description:"How many APIs to give, 1 to 200. 20 when left out: the sheet has about 150"`
}

// BestPlansInput is datacircle_best_plans' input.
type BestPlansInput struct {
	Slug  string `json:"slug" jsonschema_description:"The API's slug, like email-finder or linkedin-profile, as datacircle_list_apis gives it"`
	Limit int    `json:"limit,omitempty" jsonschema_description:"How many plans to give, 1 to 50. 10 when left out"`
}

// SearchPlansInput is datacircle_search_plans' input.
type SearchPlansInput struct {
	API    string `json:"api,omitempty" jsonschema_description:"Only this API's plans: its name or slug, like email-finder. A provider's name goes in query, never here. Every API when left out"`
	Query  string `json:"query,omitempty" jsonschema_description:"Words that must all appear in the provider's or plan's name, like bright data"`
	Sort   string `json:"sort,omitempty" jsonschema_description:"score (best first, the default), provider (A to Z) or price ($ per 1,000, lowest first)"`
	Limit  int    `json:"limit,omitempty" jsonschema_description:"How many plans to give, 1 to 50. 10 when left out"`
	Offset int    `json:"offset,omitempty" jsonschema_description:"How many found plans to skip first. 0 when left out"`
}

// ProviderInput is datacircle_provider's input.
type ProviderInput struct {
	Slug string `json:"slug" jsonschema_description:"The provider's slug, like bright-data, or its name, like Bright Data: the name is made a slug"`
}

const (
	listAPIsName    = "datacircle_list_apis"
	bestPlansName   = "datacircle_best_plans"
	searchPlansName = "datacircle_search_plans"
	providerName    = "datacircle_provider"

	listAPIsAbout    = "Lists the GTM APIs on Datacircle's sheet (https://datacircle.dev), with each API's slug, how many plans and providers it has and its best plan. Call it first to find an API's slug."
	bestPlansAbout   = "Gives an API's plans best first by their score out of 100 on Datacircle's benchmark (https://datacircle.dev), each with its rank, provider, plan, $ per 1,000 calls and its page. perThousand is a plan's price of 1,000 items in dollars, pay the plan's whole price in its currency."
	searchPlansAbout = "Searches every plan on Datacircle's sheet (https://datacircle.dev), or one API's, by provider or plan name, sorted by score, provider or price. Gives how many were found and up to limit of them. perThousand is a plan's price of 1,000 items in dollars, pay the plan's whole price in its currency."
	providerAbout    = "Gives each API one provider sells and its rank on each on Datacircle's benchmark (https://datacircle.dev), with its best plan there, its best rank first. total is how many APIs it sells, ranked on how many of them it has a rank."
)

// NewTools gives the four tools, datacircle_list_apis, datacircle_best_plans, datacircle_search_plans and datacircle_provider,
// not registered: pass them to genkit.Generate with ai.WithTools(tools...), which registers them for that call.
func NewTools(cfg *Config) []ai.ToolRef {
	return []ai.ToolRef{NewListAPIsTool(cfg), NewBestPlansTool(cfg), NewSearchPlansTool(cfg), NewProviderTool(cfg)}
}

// DefineTools registers the four tools on g and gives them, so that the Developer UI lists and runs them and a prompt
// or ai.ToolName("datacircle_best_plans") finds them by name. Call it once for a Genkit: a name is registered once.
func DefineTools(g *genkit.Genkit, cfg *Config) []ai.ToolRef {
	c := newClient(cfg)
	return []ai.ToolRef{
		genkit.DefineTool(g, listAPIsName, listAPIsAbout, listAPIsFunc(c)),
		genkit.DefineTool(g, bestPlansName, bestPlansAbout, bestPlansFunc(c)),
		genkit.DefineTool(g, searchPlansName, searchPlansAbout, searchPlansFunc(c)),
		genkit.DefineTool(g, providerName, providerAbout, providerFunc(c)),
	}
}

// NewListAPIsTool gives datacircle_list_apis, not registered: GET /apis.
func NewListAPIsTool(cfg *Config) ai.Tool {
	return ai.NewTool(listAPIsName, listAPIsAbout, listAPIsFunc(newClient(cfg)))
}

// NewBestPlansTool gives datacircle_best_plans, not registered: GET /apis/{slug}/best.
func NewBestPlansTool(cfg *Config) ai.Tool {
	return ai.NewTool(bestPlansName, bestPlansAbout, bestPlansFunc(newClient(cfg)))
}

// NewSearchPlansTool gives datacircle_search_plans, not registered: GET /plans.
func NewSearchPlansTool(cfg *Config) ai.Tool {
	return ai.NewTool(searchPlansName, searchPlansAbout, searchPlansFunc(newClient(cfg)))
}

// NewProviderTool gives datacircle_provider, not registered: GET /providers/{slug}.
func NewProviderTool(cfg *Config) ai.Tool {
	return ai.NewTool(providerName, providerAbout, providerFunc(newClient(cfg)))
}

func listAPIsFunc(c *client) ai.ToolFunc[ListAPIsInput, map[string]any] {
	return func(ctx *ai.ToolContext, in ListAPIsInput) (map[string]any, error) { return listAPIs(ctx, c, in) }
}

func bestPlansFunc(c *client) ai.ToolFunc[BestPlansInput, map[string]any] {
	return func(ctx *ai.ToolContext, in BestPlansInput) (map[string]any, error) { return bestPlans(ctx, c, in) }
}

func searchPlansFunc(c *client) ai.ToolFunc[SearchPlansInput, map[string]any] {
	return func(ctx *ai.ToolContext, in SearchPlansInput) (map[string]any, error) { return searchPlans(ctx, c, in) }
}

func providerFunc(c *client) ai.ToolFunc[ProviderInput, map[string]any] {
	return func(ctx *ai.ToolContext, in ProviderInput) (map[string]any, error) { return provider(ctx, c, in) }
}

func listAPIs(ctx context.Context, c *client, in ListAPIsInput) (map[string]any, error) {
	var apis []map[string]any
	if err := c.get(ctx, "/apis", nil, &apis); err != nil {
		return result(nil, err)
	}
	words := strings.Fields(strings.ReplaceAll(strings.ToLower(in.Query), "-", " "))
	type held struct {
		matched int
		api     map[string]any
	}
	var found []held
	for _, api := range apis {
		name := strings.ToLower(str(api["api"]) + " " + strings.ReplaceAll(str(api["slug"]), "-", " "))
		matched := 0
		for _, w := range words {
			if strings.Contains(name, w) {
				matched++
			}
		}
		if matched > 0 || len(words) == 0 {
			found = append(found, held{matched, brief(api, []string{"api", "slug", "plans", "providers", "best", "bestPage", "sheetPage"})})
		}
	}
	// The most words matched first, the sheet's order within each.
	for i := 1; i < len(found); i++ {
		for j := i; j > 0 && found[j].matched > found[j-1].matched; j-- {
			found[j], found[j-1] = found[j-1], found[j]
		}
	}
	limit := 20
	if in.Limit > 0 {
		limit = clamp(in.Limit, 1, 200)
	}
	out := []map[string]any{}
	for i := 0; i < len(found) && i < limit; i++ {
		out = append(out, found[i].api)
	}
	return result(map[string]any{"total": len(found), "apis": out}, nil)
}

func bestPlans(ctx context.Context, c *client, in BestPlansInput) (map[string]any, error) {
	slug := slugify(in.Slug)
	if slug == "" {
		return map[string]any{"error": "Give an API's slug, like email-finder: datacircle_list_apis gives every slug"}, nil
	}
	q := url.Values{}
	if in.Limit > 0 {
		q.Set("limit", strconv.Itoa(clamp(in.Limit, 1, 50)))
	}
	var answer map[string]any
	if err := c.get(ctx, "/apis/"+slug+"/best", q, &answer); err != nil {
		return result(nil, err)
	}
	answer["plans"] = briefPlans(plans(answer["plans"]))
	return result(answer, nil)
}

func searchPlans(ctx context.Context, c *client, in SearchPlansInput) (map[string]any, error) {
	api, query, note := in.API, in.Query, ""
	answer, err := search(ctx, c, api, query, in.Sort, in.Limit, in.Offset)
	if e, ok := err.(*apiError); ok && e.status == 400 && strings.Contains(e.message, "api must be") && strings.TrimSpace(query) == "" {
		// A provider's name sent as the API: the plans whose provider or plan name holds it.
		api, query = "", in.API
		answer, err = search(ctx, c, api, query, in.Sort, in.Limit, in.Offset)
		note = "No API is named " + strconv.Quote(in.API) + ": these are the plans whose provider or plan name holds it."
	}
	if total, _ := answer["total"].(float64); err == nil && total == 0 && strings.Contains(query, "-") {
		// A slug sent as a name, like bright-data, which no name holds: its words, like bright data. A name with a hyphen,
		// like A-Leads, is found as it is first.
		words := strings.Join(strings.Fields(strings.ReplaceAll(query, "-", " ")), " ")
		if again, err := search(ctx, c, api, words, in.Sort, in.Limit, in.Offset); err == nil {
			answer = again
			note = strings.TrimSpace(note + " No provider's or plan's name holds " + strconv.Quote(query) + ": these are the plans whose name holds " + strconv.Quote(words) + ".")
		}
	}
	if err != nil {
		return result(nil, err)
	}
	if note != "" {
		answer["note"] = note
	}
	answer["plans"] = briefPlans(plans(answer["plans"]))
	return result(answer, nil)
}

func search(ctx context.Context, c *client, api, query, sort string, limit, offset int) (map[string]any, error) {
	q := url.Values{}
	if strings.TrimSpace(api) != "" {
		q.Set("api", strings.TrimSpace(api))
	}
	if strings.TrimSpace(query) != "" {
		q.Set("query", strings.TrimSpace(query))
	}
	if sort == "score" || sort == "provider" || sort == "price" {
		q.Set("sort", sort)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(clamp(limit, 1, 50)))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	var answer map[string]any
	err := c.get(ctx, "/plans", q, &answer)
	return answer, err
}

func provider(ctx context.Context, c *client, in ProviderInput) (map[string]any, error) {
	slug := slugify(in.Slug)
	if slug == "" {
		return map[string]any{"error": "Give a provider's slug or name, like bright-data or Bright Data"}, nil
	}
	var answer map[string]any
	if err := c.get(ctx, "/providers/"+slug, nil, &answer); err != nil {
		return result(nil, err)
	}
	apis := []map[string]any{}
	for _, entry := range plans(answer["apis"]) {
		ps := plans(entry["plans"])
		kept := brief(entry, []string{"api", "slug", "rank", "bestPage"})
		kept["plans"] = len(ps)
		if best := bestPlan(ps); best != nil {
			kept["best"] = brief(best, []string{"plan", "score", "perThousand", "page"})
		}
		apis = append(apis, kept)
	}
	// Its best rank first, the APIs where it has none last, in the API's order.
	sort.SliceStable(apis, func(i, j int) bool {
		ri, iok := apis[i]["rank"].(float64)
		rj, jok := apis[j]["rank"].(float64)
		return iok && (!jok || ri < rj)
	})
	ranked := 0
	for _, api := range apis {
		if _, ok := api["rank"].(float64); ok {
			ranked++
		}
	}
	answer["apis"] = apis
	answer["total"] = len(apis)
	answer["ranked"] = ranked
	return result(answer, nil)
}

// bestPlan is a provider's best plan on an API: its lowest rank, a plan with no rank having none.
func bestPlan(ps []map[string]any) map[string]any {
	var best map[string]any
	low := 0.0
	for _, p := range ps {
		if r, ok := p["rank"].(float64); ok && (best == nil || r < low) {
			best, low = p, r
		}
	}
	if best == nil && len(ps) > 0 {
		best = ps[0]
	}
	return best
}

func plans(v any) []map[string]any {
	list, _ := v.([]any)
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
