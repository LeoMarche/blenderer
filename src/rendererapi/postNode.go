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

	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}

	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 {
		http.Error(w, "404 not found.", http.StatusNotFound)

		return
	}

	var receivedNode *node.Node
	receivedNode.Name = r.FormValue("name")
	receivedNode.IP = strings.Split(getIP(r), ":")[0]
	receivedNode.APIKey = r.FormValue("api_key")
	receivedNode.SetState("available")

	var rt ReturnValue

	//Create node if not exists
	n, loaded := ws.RenderNodes.LoadOrStore(receivedNode.Name+"//"+receivedNode.IP, receivedNode)
	if loaded {
		rt = ReturnValue{"Exists"}
		n.(*node.Node).SetState("available")
		ws.DBTransacts.Add(rendererdb.DBTransact{
			OP:       rendererdb.UPDATENODE,
			Argument: n.(*node.Node),
		})
	} else {
		rt = ReturnValue{"Added"}
		ws.DBTransacts.Add(rendererdb.DBTransact{
			OP:       rendererdb.INSERTNODE,
			Argument: n.(*node.Node),
		})
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
