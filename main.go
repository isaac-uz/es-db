package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"time"

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

const (
	SaveInterval = 120 * time.Millisecond
)

var (
	chanSave            = make(chan []*SaveData, 1_000)
	ctx                 = context.Background()
	elasticsearchClient *elasticsearch.Client
)

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
	r.GET("/do-conn", func(c *gin.Context) {
		es, err := initES()
		if err != nil {
			fail(c, err)
			return
		}

		ping, err := es.Ping()
		if err != nil {
			fail(c, err)
			return
		}

		finish(c, ping.String())
	})

	log.Fatalln(r.Run(":8080"))

}

func saveService() {

	for data := range chanSave {

		for i := range data {

			err := save(data[i])

			if err != nil {
				fmt.Println("on saveService:")
				fmt.Println(err)
			}

			time.Sleep(SaveInterval)

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
	data := make([]*SaveData, 0, 1)
	err := c.BindJSON(&data)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("json parse err: %v", err))
		return
	}

	chanSave <- data

	finish(c, "OK")

}

func handleSearch(c *gin.Context) {
	data := new(SearchData)
	err := c.BindJSON(&data)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("json parse err: %v", err))
		return
	}

	objs, err := searchFuzzy(data)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("search err: %v", err))
		return
	}

	finish(c, objs)

}

func searchFuzzy(data *SearchData) ([]Obj, error) {
	client := getES()
	searchBody, err := json.Marshal(gin.H{"query": data.Query})
	if err != nil {
		return nil, err
	}

	// Perform the search request
	res, err := client.Search(
		client.Search.WithIndex(data.Index),                 // Index name
		client.Search.WithBody(bytes.NewReader(searchBody)), // Search body
		client.Search.WithTrackTotalHits(true),              // Track total hits
		client.Search.WithPretty(),                          // Pretty print
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

	if elasticsearchClient == nil {
		initES()
	}

	return elasticsearchClient
}

func initES() (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{
			"http://localhost:9200",
		},
	}

	client, err := elasticsearch.NewClient(cfg)
	if err == nil {
		elasticsearchClient = client
	}

	return client, err
}

type SaveData struct {
	Index string `json:"index"`
	Id    string `json:"id"`
	Doc   any    `json:"doc"`
}

type SearchData struct {
	Index string `json:"index"`
	Query any    `json:"query"`
}

type Obj = map[string]any

func finish(c *gin.Context, res any, err ...error) {
	if len(err) > 0 && err[0] != nil {
		fail(c, err[0])
	}

	c.JSON(http.StatusOK, gin.H{
		"res":    res,
		"status": true,
	})

}

func fail(c *gin.Context, err error) {
	c.JSON(400, gin.H{
		"msg":    err.Error(),
		"status": true,
	})
}

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
