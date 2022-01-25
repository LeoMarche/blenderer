package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/rendererdb"
)

//PostJob Handler for /postNode
//The request must be a post with api_key and name
func (ws *WorkingSet) PostNode(w http.ResponseWriter, r *http.Request) {

	//Verify requests parameters
	if r.Method != "POST" || r.URL.Path != "/postNode" {
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
	for _, n := range ws.RenderNodes {
		var rt ReturnValue
		if n.Name == r.FormValue("name") && n.IP == strings.Split(getIP(r), ":")[0] && n.APIKey == r.FormValue("api_key") {
			rt = ReturnValue{"Exists"}
			n.SetState("available")
			rendererdb.UpdateNodeInDB(ws.Db, n)

		} else {
			var receivedNode node.Node
			receivedNode.Name = r.FormValue("name")
			receivedNode.IP = strings.Split(getIP(r), ":")[0]
			receivedNode.APIKey = r.FormValue("api_key")
			receivedNode.SetState("available")
			rt = ReturnValue{"Added"}

			go func() {
				ws.RenderNodesMutex.Lock()
				ws.RenderNodes = append(ws.RenderNodes, &receivedNode)
				rendererdb.InsertNodeInDB(ws.Db, &receivedNode)
				ws.RenderNodesMutex.Unlock()
			}()

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
}
