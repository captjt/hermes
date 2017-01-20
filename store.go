package hermes

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	elastic "gopkg.in/olivere/elastic.v5"
)

type (
	// Document stuct to model our single "Document" store we will ingestion into the
	// elasticsearch index/type
	Document struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Content     string `json:"content"`
		Link        string `json:"link"`
		Tag         string `json:"tag"`
	}

	// IngestionDocument struct to model our ingestion set for multiple types and Documents
	// for our index
	IngestionDocument struct {
		Documents []Document
	}

	// Index struct to model each index ingestion set for our elasticsearch data
	Index struct {
		Host      string
		Index     string
		Documents []Document
	}
)

// Store function will take in an Index struct and Marshal it to JSON and store it into an elasticsearch
// index and type based on the Index values
func Store(data Index, esType string) (bool, error) {
	// host address defaults to 127.0.0.1:9200
	client, err := elastic.NewClient(elastic.SetURL(data.Host))
	if err != nil {
		fmt.Println("New client err: \n", err)
		return false, err
	}

	exists, err := client.IndexExists(data.Index).Do(context.Background())
	if err != nil {
		fmt.Println("Index exists error")
		return false, err
	}
	if !exists {
		// Index does not exist yet.
		createIndex, err := client.CreateIndex(data.Index).Do(context.Background())
		if err != nil {
			fmt.Println("Create index error")
			return false, err
		}
		if !createIndex.Acknowledged {
			// Not acknowledged
		}
	}

	_, _, indexDay := time.Now().Date()

	bulkSize := len(data.Documents)

	bulk := client.Bulk().Index(data.Index).Type(esType)

	// add the documents to the specified bulk's index and type
	for idx, val := range data.Documents {
		bulk.Add(elastic.NewBulkIndexRequest().Id(strconv.Itoa(idx) + "Day" + strconv.Itoa(indexDay)).Doc(val))

		if bulk.NumberOfActions() >= bulkSize {
			// Commit
			res, err := bulk.Do(context.Background())

			if err != nil {
				return false, err
			}
			if res.Errors {
				// Look up the failed documents with res.Failed(), and e.g. recommit
				return false, errors.New("bulk commit failed")
			}
		}
	}

	return true, nil
}
