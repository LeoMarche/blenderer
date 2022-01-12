package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/LeoMarche/blenderer/src/node"
)

//SetAvailable Handler for /setAvailable
//The request must be a post with api_key
func (ws *WorkingSet) SetAvailable(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/setAvailable" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}

	//Determine which node is asking for a job
	ip := strings.Split(getIP(r), ":")[0]

	var n *node.Node
	conf := false

	for i := 0; i < len(ws.RenderNodes); i++ {
		if ws.RenderNodes[i].IP == ip {
			n = ws.RenderNodes[i]
			conf = true
		}
	}

	if !conf {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	//If the node asking for the job isn't registered
	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 || n.IP != ip {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	n.Free()

	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(ReturnValue{"OK"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}
