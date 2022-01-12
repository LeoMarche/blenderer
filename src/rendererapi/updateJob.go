package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/LeoMarche/blenderer/src/node"
)

//UpdateJob is handler for updating jobs
//The request must be a post with api_key, id, frame, state, percent, mem
func (ws *WorkingSet) UpdateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/updateJob" {
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

	for i := 0; i < len(ws.RenderNodes); i++ {
		if (ws.RenderNodes)[i].IP == ip {
			n = ws.RenderNodes[i]
		}
	}

	keys := []string{}

	for k := range r.Form {
		keys = append(keys, k)
	}

	st := "Error"

	//When parameters are missing
	if isIn("api_key", keys) == -1 || isIn("id", keys) == -1 || isIn("frame", keys) == -1 || isIn("state", keys) == -1 || isIn("percent", keys) == -1 || isIn("mem", keys) == -1 {
		st = "Error : Missing Parameter"
		w.Header().Set("Content-Type", "application/json")
		js, err := json.Marshal(ReturnValue{
			State: st,
		})

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write(js)
		return
	}

	//If the node asking for the job isn't registered
	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 || n.IP != ip {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	fr, _ := strconv.Atoi(r.FormValue("frame"))

	var t *Render

	//Locking Mutexs to update renders
	ws.RendersMutex.Lock()

	st = "Error : No matching Renders"

	for i := 0; i < len(ws.Renders); i++ {
		if ws.Renders[i].myTask.ID == r.FormValue("id") && ws.Renders[i].myTask.Frame == fr {
			switch rst := ws.Renders[i].myTask.State; rst {
			//Normal frame
			case "rendering":
				t = ws.Renders[i]
				t.myTask.State = r.FormValue("state")
				t.Percent = r.FormValue("percent")
				t.Mem = r.FormValue("mem")
				if t.myTask.State == "rendered" {

					//Locking mutex to ass task to completed tasks
					ws.CompletedMutex.Lock()
					ws.Completed = append(ws.Completed, t.myTask)
					ws.CompletedMutex.Unlock()

					//Updating database and freeing node for further renders
					t.myNode.Free()
					t.UpdateDatabase(ws.Db)
				}
				st = "OK"

			//Aborted frame
			case "abort":
				st = "ABORT"

			//Default
			default:
				st = "The frame is like " + rst
			}
		}
	}

	//Unlocking mutex
	ws.RendersMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(ReturnValue{
		State: st,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}
