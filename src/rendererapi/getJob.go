package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/render"
	"github.com/LeoMarche/blenderer/src/rendererdb"
)

//GetJob Handler for /getJob
//The request must be a post with api_key and name
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

	tmp_n, ok := ws.RenderNodes.Load(r.FormValue("name") + "//" + ip)
	if ok {
		n = tmp_n.(*node.Node)
		authorized = true
	} else {
		return
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

	candidates := make(map[interface{}][]interface{})

	ws.Tasks.Range(func(key, value interface{}) bool {
		value.(*sync.Map).Range(func(key2, value2 interface{}) bool {
			if value2.(*render.Task).State == "waiting" {

				//Add the keys to the candidates
				if val, ok := candidates[key]; ok {
					candidates[key] = append(val, key2)
				} else {
					candidates[key] = []interface{}{key2}
				}
			}
			return true
		})
		return true
	})

	t := new(render.Task)
	found := false
	for taskID, frameList := range candidates {

		//Load the Map containing the different tasks associated with frames
		tmpMap, ok := ws.Tasks.Load(taskID)
		if ok {

			//For each frame in candidates frames
			for _, fr := range frameList {

				//Load the corresponding task
				tsk, ok := tmpMap.(*sync.Map).Load(fr)
				if ok {
					valid := false
					var rd *Render

					tsk.(*render.Task).Lock()

					//Check if task can be comissionned
					if tsk.(*render.Task).State == "waiting" {
						t = tsk.(*render.Task)
						r := n.Commission()
						if r {
							tsk.(*render.Task).State = "rendering"
							rd = &Render{
								myTask: tsk.(*render.Task),
								myNode: n,
							}
							ws.DBTransacts.Add(&rendererdb.DBTransact{
								OP:       rendererdb.UPDATETASK,
								Argument: rd.myTask,
							})
							ws.DBTransacts.Add(&rendererdb.DBTransact{
								OP:       rendererdb.UPDATENODE,
								Argument: rd.myNode,
							})
							valid = true
						}
					}
					tsk.(*render.Task).Unlock()

					//If task commissionable, comission it and stop searching
					if valid {
						newMap := new(sync.Map)
						tmpMap, _ := ws.Renders.LoadOrStore(rd.myTask.ID, newMap)
						tmpMap.(*sync.Map).Store(rd.myTask.Frame, rd)
						found = true
						break
					}
				}
			}
		}
		if found {
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(*t)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}
