package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"strings"

	"log"
	"math/rand/v2"
	"net/http"
	"os/exec"
)

//func handler(w http.ResponseWriter, r *http.Request) {
//	// Make a request to localhost:80
//	resp, err := http.Get("http://localhost:9200")
//	if err != nil {
//		http.Error(w, fmt.Sprintf("Failed to fetch from localhost:80: %v", err), http.StatusInternalServerError)
//		return
//	}
//	defer resp.Body.Close()
//
//	// Copy the response from localhost:80 to the response writer
//	w.WriteHeader(resp.StatusCode)
//	_, err = io.Copy(w, resp.Body)
//	if err != nil {
//		http.Error(w, "Failed to copy response from localhost:80", http.StatusInternalServerError)
//	}
//}

var chanSave = make(chan *SaveData, 1_000)
var ctx = context.Background()

func main() {

	go saveService()

	r := gin.Default()

	r.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message":    "Server is running!!!",
			"random_num": rand.Int() % 1_000,
		})
	})

	r.POST("/save", handleSave)
	r.POST("/search", handleSearch)

	log.Fatalln(r.Run(":8080"))

}

func saveService() {

	for data := range chanSave {

		err := save(data)

		if err != nil {
			fmt.Println("on saveService:")
			fmt.Println(err)
		}

	}

}

func save(data *SaveData) error {

	b, err := json.Marshal(data.Doc)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %v", err)
	}

	// Perform the index request (upsert)
	res, err := getES().Index(
		data.Index,                            // Index name
		bytes.NewReader(b),                    // Document data
		getES().Index.WithDocumentID(data.Id), // Document ID
		getES().Index.WithRefresh("true"),     // Refresh immediately
	)
	if err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to save document: %s", res.String())
	}

	return nil

}

func handleSave(c *gin.Context) {
	data := new(SaveData)
	err := c.BindJSON(&data)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("json parse err: %v", err))
		return
	}

	chanSave <- data

	c.String(http.StatusOK, "OK")

}

func handleSearch(c *gin.Context) {
	data := new(SearchData)
	err := c.BindJSON(&data)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("json parse err: %v", err))
		return
	}

	if data.Fuzziness == "" {
		data.Fuzziness = "AUTO"
	}

	objs, err := searchFuzzy(getES(), data)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("search err: %v", err))
		return
	}

	c.JSON(http.StatusOK, objs)

}

func searchFuzzy(client *elasticsearch.Client, data *SearchData) ([]Obj, error) {
	searchBody := fmt.Sprintf(`
	{
		"query": {
			"match": {
				"%s": {
					"query": "%s",
					"fuzziness": "%s"
				}
			}
		}
	}`, data.Field, data.Value, data.Fuzziness)

	// Perform the search request
	res, err := client.Search(
		client.Search.WithIndex(data.Index),                   // Index name
		client.Search.WithBody(strings.NewReader(searchBody)), // Search body
		client.Search.WithTrackTotalHits(true),                // Track total hits
		client.Search.WithPretty(),                            // Pretty print
	)
	if err != nil {
		return nil, fmt.Errorf("failed to perform fuzzy search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	// Parse the response
	var r struct {
		Hits struct {
			Hits []struct {
				Source Obj `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err = json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	// Extract the results
	var objs []Obj
	for _, hit := range r.Hits.Hits {
		objs = append(objs, hit.Source)
	}
	return objs, nil
}

func getES() *elasticsearch.Client {
	cfg := elasticsearch.Config{
		Addresses: []string{
			"http://localhost:9200",
		},
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil
	}

	return client
}

type SaveData struct {
	Index string `json:"index"`
	Id    string `json:"id"`
	Doc   any    `json:"doc"`
}

type SearchData struct {
	Index     string `json:"index"`
	Field     string `json:"field"`
	Value     any    `json:"value"`
	Fuzziness string `json:"fuzziness"`
}

type Obj = map[string]any

func init() {

	go func() {

		// Command to run (e.g., `ls` on Linux, or `dir` on Windows)
		cmd := exec.Command("bin/elasticsearch")

		// Run the command and capture the output
		err := cmd.Run()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

	}()

}
