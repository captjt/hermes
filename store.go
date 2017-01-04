package main

import (
	"fmt"
	"strconv"

	elastic "gopkg.in/olivere/elastic.v3"
)

type (
	// Document stuct to model our single "Document" store we will ingestion into the
	// elasticsearch index/type
	Document struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Link    string `json:"link"`
	}

	// IndexType struct to model our ingestion set for multiple types and Documents
	// for our index
	IndexType struct {
		DocType   string
		Documents []Document
	}

	// Index struct to model each index ingestion set for our elasticsearch data
	Index struct {
		Host  string
		Index string
		Type  IndexType
	}
)

// Store function will take in an Index struct and Marshal it to JSON and store it into an elasticsearch
// index and type based on the Index values
func Store(data Index) (bool, error) {
	// host address defaults to 127.0.0.1:9200
	client, err := elastic.NewClient(elastic.SetURL("http://127.0.0.1:9200/"))
	if err != nil {
		fmt.Println("New client err: \n", err)
		return false, err
	}

	indexExists, err := client.IndexExists(data.Index).Do()
	if err != nil {
		fmt.Println("Index exists error")
		return false, err
	}
	if !indexExists {
		// Index does not exist yet.
		createIndex, err := client.CreateIndex(data.Index).Do()
		if err != nil {
			fmt.Println("Create index error")
			return false, err
		}
		if !createIndex.Acknowledged {
			// Not acknowledged
		}
	}

	// create a new document for every document scraped
	for idx, val := range data.Type.Documents {
		newDoc, err := client.Index().
			Index(data.Index).
			Type(data.Type.DocType).
			Id(strconv.Itoa(idx) + "test").
			BodyJson(val).
			Refresh(true).
			Do()
		if err != nil {
			fmt.Println("Ingestion error @ index ", idx)
			fmt.Println("   Data index: ", data.Index)
			fmt.Println("   Data type: ", data.Type.DocType)
			return false, err
		}

		// just checking for new document
		fmt.Printf("Indexed tweet %s to index %s, type %s\n", newDoc.Id, newDoc.Index, newDoc.Type)
	}

	// flush to make sure the index got written to
	_, err = client.Flush().Index(data.Index).Do()
	if err != nil {
		fmt.Println("Flush error")
		return false, err
	}

	return true, nil
}
