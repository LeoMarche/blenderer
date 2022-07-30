package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/LeoMarche/blenderer/src/render"
)

//AbortJob is handler for aborting jobs
//The request must be a post with api_key, id
func (ws *WorkingSet) AbortJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/abortJob" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	keys := []string{}

	for k := range r.Form {
		keys = append(keys, k)
	}

	st := "Error"

	//When parameters are missing
	if isIn("api_key", keys) == -1 || isIn("id", keys) == -1 {
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
	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	//Retrieving whole jobs with given id
	tmpMap, ok := ws.Tasks.Load(r.FormValue("id"))
	if ok {

		// Putting all job in state abort
		tmpMap.(*sync.Map).Range(func(key, value interface{}) bool {
			value.(*render.Task).SetState("abort")
			return true
		})
		st = "OK"
	} else {
		st = "error: can't find job"
	}

	//Optionnaly remove the jobs that are being rendered
	ws.Renders.Delete(r.FormValue(("id")))

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
