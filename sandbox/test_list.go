//go:build ignore

package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
)

func main() {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 10, 10)
	fmt.Printf("Page: %d, PerPage: %d\n", l.Paginator.Page, l.Paginator.PerPage)
}
