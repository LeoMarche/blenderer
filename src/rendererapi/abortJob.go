package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
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
		js, err := json.Marshal(returnvalue{
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

	go func() {
		//Locking Mutexes
		ws.UploadingMutex.Lock()
		ws.WaitingMutex.Lock()
		ws.RendersMutex.Lock()

		//Aborting all the frames
		for i := 0; i < len(ws.Uploading); i++ {
			if ws.Uploading[i].ID == r.FormValue("id") {
				ws.Uploading[i].State = "abort"
				updateProjectinDB(ws.Db, ws.Uploading[i])
			}
		}

		for i := 0; i < len(ws.Waiting); i++ {
			if ws.Waiting[i].ID == r.FormValue("id") {
				ws.Waiting[i].State = "abort"
				updateProjectinDB(ws.Db, ws.Waiting[i])
			}
		}

		for i := 0; i < len(ws.Renders); i++ {
			if ws.Renders[i].myTask.ID == r.FormValue("id") {
				ws.Renders[i].myTask.State = "abort"
				updateProjectinDB(ws.Db, ws.Renders[i].myTask)
			}
		}
		//Unlockign Mutexes
		ws.RendersMutex.Unlock()
		ws.WaitingMutex.Unlock()
		ws.UploadingMutex.Unlock()
	}()

	st = "OK"

	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(returnvalue{
		State: st,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(js)
}
