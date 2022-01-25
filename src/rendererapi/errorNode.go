package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/LeoMarche/blenderer/src/rendererdb"
)

//PostJob Handler for /postNode
//The request must be a post with api_key and name
func (ws *WorkingSet) ErrorNode(w http.ResponseWriter, r *http.Request) {

	//Verify requests parameters
	if r.Method != "POST" || r.URL.Path != "/errorNode" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}

	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 {
		http.Error(w, "404 not found.", http.StatusNotFound)

		return
	}

	// Create node if not exists
	for _, re := range ws.Renders {
		if re.myNode.IP == strings.Split(getIP(r), ":")[0] && re.myNode.Name == r.FormValue("name") {
			re.myNode.Lock()
			re.myNode.SetState("error")
			re.myNode.Unlock()
			go rendererdb.UpdateNodeInDB(ws.Db, re.myNode)

			ws.WaitingMutex.Lock()
			re.myTask.State = "waiting"
			if !isInTasksList(re.myTask, &ws.Waiting) {
				ws.Waiting = append(ws.Waiting, re.myTask)
			}
			ws.WaitingMutex.Unlock()
			go rendererdb.UpdateTaskInDB(ws.Db, re.myTask)

			break
		}
	}

	var rt ReturnValue

	rt.State = "Done"

	//Send answer
	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(rt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}
