package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/rendererdb"
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

	st := "Error"
	var n *node.Node

	tmp_n, ok := ws.RenderNodes.Load(r.FormValue("name") + "//" + ip)
	if ok {
		n = tmp_n.(*node.Node)
	} else {
		st = "Error : Missing parameter 'name'"
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

	keys := []string{}

	for k := range r.Form {
		keys = append(keys, k)
	}

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

	st = "Error : No matching Renders"
	tmpMap, ok := ws.Renders.Load(r.FormValue("id"))
	if ok {
		rdr, ok := tmpMap.(*sync.Map).Load(fr)
		if ok {
			switch rst := rdr.(*Render).myTask.State; rst {

			//Normal frame
			case "rendering":

				//Update render stats
				t = rdr.(*Render)
				t.myTask.Lock()
				t.myTask.State = r.FormValue("state")
				t.Percent = r.FormValue("percent")
				t.Mem = r.FormValue("mem")
				t.myTask.Unlock()

				//Handle the case 'frame rendered'
				if t.myTask.State == "rendered" {

					//Updating database and freeing node for further renders
					t.myNode.Free()
					ws.DBTransacts.Add(rendererdb.DBTransact{
						OP:       rendererdb.UPDATENODE,
						Argument: t.myNode,
					})

					//Removing task from Renders and updating database
					tmpMap.(*sync.Map).Delete(fr)
					ws.DBTransacts.Add(rendererdb.DBTransact{
						OP:       rendererdb.UPDATETASK,
						Argument: t.myTask,
					})
				}
				st = "OK"

			//Aborted frame
			case "abort":

				//Notify worker
				st = "ABORT"

				//Delete job from on progress renders
				tmpMap.(*sync.Map).Delete(fr)

			//Default
			default:
				st = "The frame is like " + rst
			}
		}

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
}
