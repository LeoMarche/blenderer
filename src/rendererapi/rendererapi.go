package rendererapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/render"
)

//WorkingSet contains variables for main to work
type WorkingSet struct {
	Db             *sql.DB
	Uploading      []*render.Task
	Waiting        []*render.Task
	Completed      []*render.Task
	RenderNodes    []*node.Node
	Renders        []*Render
	Config         Configuration
	UploadingMutex sync.Mutex
	WaitingMutex   sync.Mutex
	CompletedMutex sync.Mutex
	RendersMutex   sync.Mutex
}

type returnvalue struct {
	State string
}

func getIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

func isIn(s string, t []string) int {

	for i := 0; i < len(t); i++ {
		if t[i] == s {
			return i
		}
	}

	return -1
}

//UpdateJob is handler for updating jobs
//The request must be a post with api_key, id, frame, state, percent, mem
func (ws *WorkingSet) UpdateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/updateJob" {
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

	for i := 0; i < len(ws.RenderNodes); i++ {
		if (ws.RenderNodes)[i].IP == ip {
			n = ws.RenderNodes[i]
		}
	}

	keys := []string{}

	for k := range r.Form {
		keys = append(keys, k)
	}

	st := "Error"

	//When parameters are missing
	if isIn("api_key", keys) == -1 || isIn("id", keys) == -1 || isIn("frame", keys) == -1 || isIn("state", keys) == -1 || isIn("percent", keys) == -1 || isIn("mem", keys) == -1 {
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
	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 || n.IP != ip {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	fr, _ := strconv.Atoi(r.FormValue("frame"))

	var t *Render

	//Locking Mutexs to update renders
	ws.RendersMutex.Lock()

	st = "Error : No matching Renders"

	for i := 0; i < len(ws.Renders); i++ {
		if ws.Renders[i].myTask.ID == r.FormValue("id") && ws.Renders[i].myTask.Frame == fr {
			switch rst := ws.Renders[i].myTask.State; rst {
			//Normal frame
			case "rendering":
				t = ws.Renders[i]
				t.myTask.State = r.FormValue("state")
				t.Percent = r.FormValue("percent")
				t.Mem = r.FormValue("mem")
				if t.myTask.State == "rendered" {

					//Locking mutex to ass task to completed tasks
					ws.CompletedMutex.Lock()
					ws.Completed = append(ws.Completed, t.myTask)
					ws.CompletedMutex.Unlock()

					//Updating database and freeing node for further renders
					t.myNode.Free()
					t.updateDatabase(ws.Db)
				}
				st = "OK"
				break

			//Aborted frame
			case "abort":
				st = "ABORT"
				break

			//Default
			default:
				st = "The frame is like " + rst
				break
			}
		}
	}

	//Unlocking mutex
	ws.RendersMutex.Unlock()

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

//GetJob Handler for /getJob
//The request must be a post with api_key
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
	conf := false

	for i := 0; i < len(ws.RenderNodes); i++ {
		if ws.RenderNodes[i].IP == ip {
			n = ws.RenderNodes[i]
			conf = true
		}
	}

	if !conf {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	//If the node asking for the job isn't registered
	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 || n.IP != ip {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	t := new(render.Task)
	ws.WaitingMutex.Lock()
	for i := 0; i < len(ws.Waiting); i++ {
		if ws.Waiting[i].State == "waiting" {
			t = ws.Waiting[i]
			t.State = "rendering"
			r := n.Commission()
			if r {

				//Creating new render
				rd := Render{
					myTask: t,
					myNode: n,
				}

				//Pushing it in Renders
				ws.RendersMutex.Lock()
				ws.Renders = append(ws.Renders, &rd)
				ws.RendersMutex.Unlock()

				rd.updateDatabase(ws.Db)
			}
			break
		}
	}
	ws.WaitingMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(*t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

//SetAvailable Handler for /setAvailable
//The request must be a post with api_key
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

	var n *node.Node
	conf := false

	for i := 0; i < len(ws.RenderNodes); i++ {
		if ws.RenderNodes[i].IP == ip {
			n = ws.RenderNodes[i]
			conf = true
		}
	}

	if !conf {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	//If the node asking for the job isn't registered
	if r.FormValue("api_key") == "" || isIn(r.FormValue("api_key"), ws.Config.UserAPIKeys) == -1 || n.IP != ip {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	n.Free()

	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(returnvalue{"OK"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

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
		insertProjectsInDB(ws.Db, it)
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

//UploadCompleted must be triggered by client when the upload is completed in the good folder
//The request should be a post with values id, api_key, size and project
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

	resp := new(returnvalue)

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
						updateProjectinDB(ws.Db, ws.Uploading[i])
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

func isInInterface(t *[]interface{}, te interface{}) bool {
	for i := 0; i < len(*t); i++ {
		if (*t)[i] == te {
			return true
		}
	}

	return false
}

//GetAllRenderTasks handle the api to get all renders tasks
//The request must be a post with api_key
func (ws *WorkingSet) GetAllRenderTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/getAllRenderTasks" {
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

	type taskToSend struct {
		Project   string
		ID        string
		Percent   float64
		Nb        int
		StartTime string
	}

	ret := new([]taskToSend)
	lid := new([]string)

	w.Header().Set("Content-Type", "application/json")

	id := -1

	for i := 0; i < len(ws.Waiting); i++ {
		id = -1
		id = isIn(ws.Waiting[i].ID, *lid)
		if id == -1 {
			newTTS := taskToSend{ws.Waiting[i].Project, ws.Waiting[i].ID, 0.0, 0, ws.Waiting[i].StartTime}
			*ret = append(*ret, newTTS)
			*lid = append(*lid, ws.Waiting[i].ID)
			id = len(*lid) - 1
		}
		(*ret)[id].Nb++
	}

	for i := 0; i < len(ws.Renders); i++ {
		id = -1
		id = isIn(ws.Renders[i].myTask.ID, *lid)
		if id == -1 {
			newTTS := taskToSend{ws.Renders[i].myTask.Project, ws.Renders[i].myTask.ID, 0.0, 0, ws.Renders[i].myTask.StartTime}
			*ret = append(*ret, newTTS)
			*lid = append(*lid, ws.Renders[i].myTask.ID)
			id = len(*lid) - 1
		}
		(*ret)[id].Nb++
		var s float64
		s, _ = strconv.ParseFloat(ws.Renders[i].Percent, 64)
		(*ret)[id].Percent += s
	}

	for i := 0; i < len(ws.Completed); i++ {
		id = -1
		id = isIn(ws.Completed[i].ID, *lid)
		if id == -1 {
			newTTS := taskToSend{ws.Completed[i].Project, ws.Completed[i].ID, 0.0, 0, ws.Completed[i].StartTime}
			*ret = append(*ret, newTTS)
			*lid = append(*lid, ws.Completed[i].ID)
			id = len(*lid) - 1
		}
		(*ret)[id].Nb++
		(*ret)[id].Percent++
	}
	for i := 0; i < len(*ret); i++ {
		(*ret)[i].Percent = (*ret)[i].Percent / float64((*ret)[i].Nb)
	}

	js, err := json.Marshal(*ret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

//Configuration is the main configuration
type Configuration struct {
	Folder      string
	DBName      string
	Certname    string
	UserAPIKeys []string
}

//Upload allows client to upload file
type Upload struct {
	Token   string
	Project string
	State   string
}

//Render is the base descriptor of a render
type Render struct {
	myTask  *render.Task
	myNode  *node.Node
	Percent string
	Mem     string
}

//GetState returns the state of the myTask of the Render Object
func (r *Render) GetState() string {
	return r.myTask.State
}

//This function updates states in database using state of objects
func (r *Render) updateDatabase(db *sql.DB) {

	reqTask := "UPDATE projects SET state = ? WHERE project = ? AND id = ? AND frame = ?"
	reqNode := "UPDATE compute_nodes SET state = ? WHERE name = ? AND ip = ?"

	tx, e := db.Begin()
	if e != nil {
		log.Fatal(e.Error())
	}
	statement, err := db.Prepare(reqTask)
	if err != nil {
		tx.Rollback()
		log.Fatal(err.Error())
	}
	statement.Exec(r.myTask.State, r.myTask.Project, r.myTask.ID, r.myTask.Frame)

	statement, err = db.Prepare(reqNode)
	if err != nil {
		tx.Rollback()
		log.Fatal(err.Error())
	}
	statement.Exec(r.myNode.State(), r.myNode.Name, r.myNode.IP)
	tx.Commit()
}

//Updates a project in database
func updateProjectinDB(db *sql.DB, rt *render.Task) {
	reqTask := "UPDATE projects SET state = ? WHERE project = ? AND id = ? AND frame = ?"
	tx, e := db.Begin()
	if e != nil {
		log.Fatal(e.Error())
	}
	statement, err := db.Prepare(reqTask)
	if err != nil {
		tx.Rollback()
		log.Fatal(err.Error())
	}
	statement.Exec(rt.State, rt.Project, rt.ID, rt.Frame)
	tx.Commit()
}

//Insert a list of projects in database
func insertProjectsInDB(db *sql.DB, it []*render.Task) {
	rq := "INSERT INTO projects VALUES(?,?,?,?,?,?,?,?,?)"
	tx, e := db.Begin()
	if e != nil {
		log.Fatal(e.Error())
	}
	statement, err := db.Prepare(rq)
	for i := 0; i < len(it); i++ {
		statement.Exec(
			it[i].Project,
			it[i].ID,
			it[i].Input,
			it[i].Output,
			it[i].Frame,
			it[i].State,
			it[i].RendererName,
			it[i].RendererVersion,
			it[i].StartTime)
		if err != nil {
			tx.Rollback()
			log.Fatal(err.Error())
		}
	}
	err = tx.Commit()
}
