package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/LeoMarche/blenderer/src/rendererdb"
)

//UploadCompleted must be triggered by client when the upload is completed in the good folder
//The request should be a post with values id, api_key, size and project and input
func (ws *WorkingSet) UploadCompleted(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/uploadCompleted" {
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

	resp := new(ReturnValue)

	pat := filepath.Join(ws.Config.Folder, r.FormValue("id"), r.FormValue("input"))

	expSize, err := strconv.Atoi(r.FormValue("size"))

	if st, err := os.Stat(pat); err == nil {
		if st.Size() == int64(expSize) {

			resp.State = "Completed"

			go func() {
				//We lock mutexes and displace from Uploading to Waiting Queue
				ws.UploadingMutex.Lock()
				ws.WaitingMutex.Lock()

				for i := 0; i < len(ws.Uploading); i++ {
					if ws.Uploading[i].Project == r.FormValue("project") && ws.Uploading[i].ID == r.FormValue("id") && ws.Uploading[i].State == "uploading" {
						ws.Uploading[i].State = "waiting"
						rendererdb.UpdateTaskInDB(ws.Db, ws.Uploading[i])
						ws.Waiting = append(ws.Waiting, ws.Uploading[i])
					}
				}
				ws.WaitingMutex.Unlock()
				ws.UploadingMutex.Unlock()
			}()
		} else {
			resp.State = "Uploading"
		}

	} else if os.IsNotExist(err) {
		resp.State = "Not uploaded"

	} else {
		resp.State = "General error"
	}

	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)

}
