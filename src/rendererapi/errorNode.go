package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/rendererdb"
)

//ErrorNode handler for /errorNode
func (ws *WorkingSet) ErrorNode(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}

	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 {
		http.Error(w, "404 not found.", http.StatusNotFound)

		return
	}

	var rt ReturnValue

	rt.State = "Done"

	tmpNode, ok := ws.RenderNodes.Load(r.FormValue("name") + "//" + strings.Split(getIP(r), ":")[0])

	if !ok {
		rt.State = "Couldn't find matching node"
	} else {
		// Set Node in error and put back the task in waiting state
		tmpNode.(*node.Node).SetState("error")
		ws.DBTransacts.Add(rendererdb.DBTransact{
			OP:       rendererdb.UPDATENODE,
			Argument: tmpNode.(*node.Node),
		})

		rendersToDelete := make(map[interface{}][]interface{})
		ws.Renders.Range(func(key, value interface{}) bool {
			value.(*sync.Map).Range(func(key2, value2 interface{}) bool {
				if value2.(*Render).myNode.IP == strings.Split(getIP(r), ":")[0] && value2.(*Render).myNode.Name == r.FormValue("name") {

					//Add the keys to the rendersToDeletes
					if val, ok := rendersToDelete[key]; ok {
						rendersToDelete[key] = append(val, key2)
					} else {
						rendersToDelete[key] = []interface{}{key2}
					}
				}
				return true
			})
			return true
		})

		// Delete concerned renders from the render list
		for key, value := range rendersToDelete {
			m, ok := ws.Renders.Load(key)
			if ok {
				for _, key2 := range value {
					deletedRT, ok := m.(*sync.Map).Load(key2)
					m.(*sync.Map).Delete(key2)
					if ok {

						//Set the state of the renders the node was doing
						deletedRT.(*Render).myTask.SetState("waiting")

						ws.DBTransacts.Add(&rendererdb.DBTransact{
							OP:       rendererdb.UPDATETASK,
							Argument: deletedRT.(*Render).myTask,
						})
					}
				}
			}
		}
	}

	//Send answer
	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(rt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}
