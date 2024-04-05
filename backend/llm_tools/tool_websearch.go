package llm_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/nilsherzig/localLLMSearch/utils"
	"github.com/sourcegraph/conc"
	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/tools"
)

type WebSearch struct {
	CallbacksHandler callbacks.Handler
	SessionString    string
}

var usedLinks = make(map[string][]string)

var _ tools.Tool = WebSearch{}

func (c WebSearch) Description() string {
	return `Usefull for searching the internet. You have to use this tool if you're not 100% certain. The top 10 results will be added to the vector db. The top 3 results are also getting returned to you directly. For more search queries through the same websites, use the VectorDB tool.`
}

func (c WebSearch) Name() string {
	return "WebSearch"
}

func (ws WebSearch) Call(ctx context.Context, input string) (string, error) {
	if ws.CallbacksHandler != nil {
		ws.CallbacksHandler.HandleToolStart(ctx, input)
	}

	input = strings.TrimPrefix(input, "\"")
	input = strings.TrimSuffix(input, "\"")
	inputQuery := url.QueryEscape(input)
	searXNGDomain := os.Getenv("SEARXNG_DOMAIN")
	url := fmt.Sprintf("%s/?q=%s&format=json", searXNGDomain, inputQuery)
	resp, err := http.Get(url)

	if err != nil {
		log.Println("Error making the request:", err)
		return "", err
	}
	defer resp.Body.Close()

	var apiResponse utils.SeaXngResult
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		fmt.Println("Error parsing the JSON:", err)
		return "", err
	}

	log.Printf("Search found %d Results\n", len(apiResponse.Results))
	wg := conc.WaitGroup{}
	var mu sync.Mutex
	counter := 0
	for _, result := range apiResponse.Results {
		result := result

		for _, usedLink := range usedLinks[ws.SessionString] {
			if usedLink == result.URL {
				continue
			}
		}

		if counter > 10 {
			break
		}

		// if result link ends in .pdf, skip
		if strings.HasSuffix(result.URL, ".pdf") {
			continue
		}

		mu.Lock()
		counter += 1
		mu.Unlock()

		wg.Go(func() {
			defer func() {
				mu.Lock()
				usedLinks[ws.SessionString] = append(usedLinks[ws.SessionString], result.URL)
				mu.Unlock()
			}()

			err := utils.DownloadWebsiteToVectorDB(ctx, result.URL, ws.SessionString)
			if err != nil {
				log.Printf("error from evaluator: %s", err.Error())
				return
			}
			ch, ok := ws.CallbacksHandler.(utils.CustomHandler)
			if ok {
				newSource := utils.Source{
					Name: "WebSearch",
					Link: result.URL,
				}

				ch.HandleSourceAdded(ctx, newSource)
			}
		})
	}
	wg.Wait()
	result, err := SearchVectorDB.Call(
		SearchVectorDB{
			CallbacksHandler: nil,
			SessionString:    ws.SessionString,
		},
		context.Background(), input)
	if err != nil {
		return fmt.Sprintf("error from vector db search: %s", err.Error()), nil //nolint:nilerr
	}

	if ws.CallbacksHandler != nil {
		ws.CallbacksHandler.HandleToolEnd(ctx, result)
	}

	if len(apiResponse.Results) == 0 {
		return "No results found", fmt.Errorf("No results, we might be rate limited")
	}

	return result, nil
}
