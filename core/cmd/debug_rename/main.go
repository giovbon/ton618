package main

import (
	"fmt"
	"ton618/core/internal/core/db"
)

func main() {
	store, err := db.NewStore("data/ton618.db")
	if err != nil {
		fmt.Println("Erro db:", err)
		return
	}
	defer store.Close()

	has23 := store.NoteExists("notes/simples-atalho-23.md")
	has2344 := store.NoteExists("notes/simples-atalho-2344.md")

	fmt.Printf("notes/simples-atalho-23.md exists in DB? %v\n", has23)
	fmt.Printf("notes/simples-atalho-2344.md exists in DB? %v\n", has2344)

	if has23 {
		c, _ := store.GetNote("notes/simples-atalho-23.md")
		fmt.Printf("Content 23:\n%s\n", c)
	}
	if has2344 {
		c, _ := store.GetNote("notes/simples-atalho-2344.md")
		fmt.Printf("Content 2344:\n%s\n", c)
	}

	bl, _ := store.GetBacklinks("notes/simples-atalho-23.md")
	fmt.Printf("Backlinks for 23: %v\n", bl)

	bl2, _ := store.GetBacklinks("notes/simples-atalho-2344.md")
	fmt.Printf("Backlinks for 2344: %v\n", bl2)
}
