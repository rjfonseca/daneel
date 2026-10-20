//nolint:revive // package comment not required
package tui

import (
	"encoding/json"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
)

type TUICommandMsg struct { //nolint:revive
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args"`
}

type TUIStateQueryMsg struct { //nolint:revive
	ReplyChan chan map[string]interface{}
}

func StartMCPServer(p *tea.Program) { //nolint:revive
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			reply := make(chan map[string]interface{})
			p.Send(TUIStateQueryMsg{ReplyChan: reply})
			state := <-reply
			_ = json.NewEncoder(w).Encode(state) //nolint:errcheck,gosec // safe to ignore
		})
		mux.HandleFunc("/command", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var msg TUICommandMsg
			if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			p.Send(msg)
			w.WriteHeader(http.StatusOK)
		})
		// Listen on a local port dedicated to Daneel TUI MCP
		_ = http.ListenAndServe("127.0.0.1:43210", mux) //nolint:gosec // intentional
	}()
}
