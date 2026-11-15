# Datacircle tools for Genkit Go

Find the best GTM APIs. We benchmark all the GTM APIs and we put the results on our website: https://datacircle.dev

This Go module gives [Genkit](https://github.com/genkit-ai/genkit) for Go four tools on Datacircle's open API, https://api.datacircle.dev/v1. You need no key and no account.

| Tool | What the tool gives | Call |
| --- | --- | --- |
| `datacircle_list_apis` | Every GTM API on Datacircle, with the API's slug, number of plans, number of providers and best plan | `GET /apis` |
| `datacircle_best_plans` | One API's plans, sorted by score out of 100, best first. Each plan comes with a rank and a price per 1,000 calls | `GET /apis/{slug}/best` |
| `datacircle_search_plans` | Plans that match a provider or plan name, on all APIs or on one API. Sort by score, provider or price | `GET /plans` |
| `datacircle_provider` | The APIs a provider sells, and the provider's rank on each API | `GET /providers/{slug}` |

## Install

```bash
go get github.com/waynehamadi/genkit-datacircle
```

It needs Genkit Go v1.0.0 or later.

## Use the tools

```go
import datacircle "github.com/waynehamadi/genkit-datacircle"

g := genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{}))
tools := datacircle.DefineTools(g, nil)

resp, err := genkit.Generate(ctx, g,
	ai.WithModelName("googleai/gemini-2.5-flash"),
	ai.WithPrompt("Who is the best provider for the email-finder API?"),
	ai.WithTools(tools...),
)
fmt.Println(resp.Text())
```

`DefineTools` registers the four tools on your Genkit. The Developer UI (`genkit start -- go run .`) then lists them under Tools, and you can run each one there. A prompt or `ai.ToolName("datacircle_best_plans")` finds a tool by its name. Call `DefineTools` once for each Genkit.

To use the tools without registering them, use `datacircle.NewTools(nil)`. `genkit.Generate` registers them for that one call:

```go
resp, err := genkit.Generate(ctx, g,
	ai.WithPrompt("Which APIs does Bright Data sell?"),
	ai.WithTools(datacircle.NewTools(nil)...),
)
```

Each tool has its own constructor: `NewListAPIsTool`, `NewBestPlansTool`, `NewSearchPlansTool` and `NewProviderTool`. Give a model only the ones it needs.

Each tool gives back a JSON object. Each plan comes with its rank, provider, plan, score, price per 1,000 (`perThousand`, in dollars) and its page on datacircle.dev. That page lists every field. When the API gives no data, like for a slug it doesn't know, the tool's result is `{"error": "..."}` with the API's own words, so the model can call again. A network failure is the tool's error, and it stops `genkit.Generate`.

`datacircle.Config` sets the `HTTPClient` the tools use (a timeout, a proxy). `nil` uses `http.DefaultClient`.

## Network

Each tool call makes one HTTPS GET request to https://api.datacircle.dev/v1 and nothing else. The request holds only the tool's own arguments: no key, no config, nothing from your chats. The API takes up to 60 calls a minute from one IP address. The tool waits once, for a minute at most, when the API asks the tool to wait. The module depends only on Genkit Go and `golang.org/x/text`, and stores nothing.

## Links

- Datacircle: https://datacircle.dev
- The API: https://datacircle.dev/api
- The API's spec: https://api.datacircle.dev/v1/openapi.json
- What each key means: https://datacircle.dev/fields

MIT license.
