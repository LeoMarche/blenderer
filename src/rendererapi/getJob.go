package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/render"
)

//GetJob Handler for /getJob
//The request must be a post with api_key
func (ws *WorkingSet) GetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/getJob" {
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

	authorized := false

	for i := 0; i < len(ws.RenderNodes); i++ {
		if ws.RenderNodes[i].IP == ip {
			n = ws.RenderNodes[i]
			authorized = true
		}
	}

	if !authorized {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	//If the node asking for the job isn't registered
	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 || n.IP != ip {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	t := new(render.Task)
	ws.WaitingMutex.Lock()
	for i := 0; i < len(ws.Waiting); i++ {
		if ws.Waiting[i].State == "waiting" {
			t = ws.Waiting[i]
			t.State = "rendering"
			r := n.Commission()
			if r {

				//Creating new render
				rd := Render{
					myTask: t,
					myNode: n,
				}

				//Pushing it in Renders
				ws.RendersMutex.Lock()
				ws.Renders = append(ws.Renders, &rd)
				ws.RendersMutex.Unlock()

				rd.updateDatabase(ws.Db)
			}
			break
		}
	}
	ws.WaitingMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(*t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}
