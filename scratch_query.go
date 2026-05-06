package main

import (
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/query"
	"fmt"
	"log"
)

func main() {
	cfg := config.LoadConfig()
	state := ingest.NewAppState(cfg)
	state.Load(cfg)

	queries := []string{
		"TABLE status WHERE status == \"fazendo\"",
		"TABLE file.mtime, status SORT file.mtime DESC",
		"TABLE file.name, file.size SORT file.size DESC",
	}

	for _, q := range queries {
		fmt.Printf("\nExecuting query: %s\n", q)
		res, err := query.Execute(q, state, cfg)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("Headers: %v\n", res.Headers)
		fmt.Printf("Rows (%d):\n", len(res.Rows))
		for _, row := range res.Rows {
			fmt.Printf("  %v\n", row)
		}
	}
}
