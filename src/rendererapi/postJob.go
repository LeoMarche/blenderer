package rendererapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LeoMarche/blenderer/src/render"
	"github.com/LeoMarche/blenderer/src/rendererdb"
)

//PostJob Handler for /postJob
//The request must be a post with api_key, project, input, output, frameStart, frameStop, rendererName, rendererVersion, startTime
func (ws *WorkingSet) PostJob(w http.ResponseWriter, r *http.Request) {

	//Verify requests parameters
	if r.Method != "POST" || r.URL.Path != "/postJob" {
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

	//Create Video Task
	var err error

	var receivedTask render.VideoTask
	receivedTask.Project = r.FormValue("project")
	receivedTask.Input = r.FormValue("input")
	receivedTask.Output = r.FormValue("output")
	receivedTask.FrameStart, err = strconv.Atoi(r.FormValue("frameStart"))
	if err != nil {
		return
	}
	receivedTask.FrameStop, err = strconv.Atoi(r.FormValue("frameStop"))
	if err != nil {
		return
	}
	receivedTask.RendererName = r.FormValue("rendererName")
	receivedTask.RendererVersion = r.FormValue("rendererVersion")

	t := time.Now().String()
	sha256 := sha256.Sum256([]byte(t))
	receivedTask.ID = base64.StdEncoding.EncodeToString(sha256[:])
	receivedTask.ID = strings.Replace(receivedTask.ID, "/", "", -1)

	receivedTask.State = "uploading"
	receivedTask.StartTime = r.FormValue("startTime")

	it := receivedTask.GetIndividualTasks()

	go func() {
		ws.UploadingMutex.Lock()
		ws.Uploading = append(ws.Uploading, it...)
		rendererdb.InsertProjectsInDB(ws.Db, it)
		ws.UploadingMutex.Unlock()
	}()

	up := Upload{Project: receivedTask.Project, Token: receivedTask.ID, State: "ready"}

	//Send answer
	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(up)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}
