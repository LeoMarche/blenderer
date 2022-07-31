package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/rendererdb"
)

//SetAvailable Handler for /setAvailable
//The request must be a post with api_key and name
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

	//If the node asking for the job isn't registered
	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	var n *node.Node

	rt := new(ReturnValue)

	tmpNode, ok := ws.RenderNodes.Load(r.FormValue("name") + "//" + ip)
	if ok {
		n = tmpNode.(*node.Node)
		rt.State = "OK"
		n.SetState("available")
		ws.DBTransacts.Add(rendererdb.DBTransact{
			OP:       rendererdb.UPDATENODE,
			Argument: n,
		})
	} else {
		rt.State = "Can't find node"
	}

	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(rt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}
