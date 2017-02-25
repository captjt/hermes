package hermes

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"strconv"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"gopkg.in/olivere/elastic.v5"
)

type (
	// Document stuct to model our single "Document" store we will ingestion into the
	// elasticsearch index/type
	Document struct {
		ID          string    `json:"id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		Content     string    `json:"content"`
		Link        string    `json:"link"`
		Tag         string    `json:"tag"`
		Time        time.Time `json:"time"`
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

	// The Elasticsearch struct type is to model the storage into a single ELasticsearch node.
	// It must have a host, index and type to ingest data to.
	Elasticsearch struct {
		Host, Index, Type string
	}
)

// Store function will take total documents, es host, es index, es type and the Documents to be ingested.
// It will return with an error if faulted or will print stats on ingestion process (Total, Requests/sec, Time to ingest)
func (e *Elasticsearch) Store(n int, docs []Document) error {
	rand.Seed(time.Now().UnixNano())

	if e.Host == "" {
		log.WithFields(log.Fields{
			"total documents": n,
			"host":            e.Host,
			"index":           e.Index,
			"type":            e.Type,
		}).Fatal("missing host parameter")
		return errors.New("missing host parameter")
	}
	if e.Index == "" {
		log.WithFields(log.Fields{
			"total documents": n,
			"host":            e.Host,
			"index":           e.Index,
			"type":            e.Type,
		}).Fatal("missing index parameter")
		return errors.New("missing index parameter")
	}
	if e.Type == "" {
		log.WithFields(log.Fields{
			"total documents": n,
			"host":            e.Host,
			"index":           e.Index,
			"type":            e.Type,
		}).Fatal("missing type parameter")
		return errors.New("missing type parameter")
	}
	if n <= 0 {
		log.WithFields(log.Fields{
			"total documents": n,
			"host":            e.Host,
			"index":           e.Index,
			"type":            e.Type,
		}).Fatal("total documents must be a positive number")
		return errors.New("n must be a positive number")
	}

	// Create an Elasticsearch client
	client, err := elastic.NewClient(elastic.SetURL(e.Host), elastic.SetSniff(true))
	if err != nil {
		log.WithFields(log.Fields{
			"total documents": n,
			"host":            e.Host,
			"index":           e.Index,
			"type":            e.Type,
			"error":           err,
		}).Fatal("an elasticsearch client connection error")
		return err
	}

	// Setup a group of goroutines from the errgroup package
	g, ctx := errgroup.WithContext(context.TODO())

	// The first goroutine will emit documents and send it to the second goroutine
	// via the docsc channel.
	// The second Goroutine will simply bulk insert the documents.
	docsc := make(chan Document)

	begin := time.Now()

	// Goroutine to traverse documents
	g.Go(func() error {
		defer close(docsc)

		buf := make([]byte, 32)
		for _, v := range docs {

			_, err := rand.Read(buf)
			if err != nil {
				return err
			}
			v.ID = base64.URLEncoding.EncodeToString(buf)

			fmt.Printf("new ID: %s\n", v.ID)

			// Send over to 2nd goroutine, or cancel
			select {
			case docsc <- v:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	// Second goroutine will consume the documents sent from the first and bulk insert into ES
	var total uint64
	g.Go(func() error {
		bulk := client.Bulk().Index(e.Index).Type(e.Type)
		for d := range docsc {
			// Simple progress
			current := atomic.AddUint64(&total, 1)
			dur := time.Since(begin).Seconds()
			sec := int(dur)
			pps := int64(float64(current) / dur)
			fmt.Printf("%10d | %6d req/s | %02d:%02d\r", current, pps, sec/60, sec%60)

			// Enqueue the document
			bulk.Add(elastic.NewBulkIndexRequest().Id(d.ID).Doc(d))
			if bulk.NumberOfActions() >= 1000 {
				// Commit
				res, err := bulk.Do(ctx)
				if err != nil {
					log.WithFields(log.Fields{
						"error": err,
					}).Error("an elasticsearch bulk insert error")
					return err
				}
				if res.Errors {
					// Look up the failed documents with res.Failed(), and e.g. recommit
					return errors.New("bulk commit failed")
				}

				// elasticsearch bulk insert function is enabled again after .Do ("commit")
				// "bulk" is reset after Do, so you can reuse it
			}

			select {
			default:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Commit the final batch before exiting
		if bulk.NumberOfActions() > 0 {
			_, err = bulk.Do(ctx)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("an elasticsearch bulk insert error")
				return err
			}
		}
		return nil
	})

	// Wait until all goroutines are finished
	if err := g.Wait(); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("a global goroutine wait error")
		return err
	}

	// Final results
	dur := time.Since(begin).Seconds()
	sec := int(dur)
	pps := int64(float64(total) / dur)
	fmt.Printf("\n\n|- %10d -|- %6d req/s -|- %02d:%02d -|\n", total, pps, sec/60, sec%60)
	log.WithFields(log.Fields{
		"total":    total,
		"req/s":    pps,
		"duration": strconv.Itoa(sec/60) + ":" + strconv.Itoa(sec%60),
		"error":    err,
	}).Info("total storage statistics")

	return nil
}
