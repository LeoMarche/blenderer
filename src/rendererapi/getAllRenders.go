package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

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

	ret := new([]TaskToSend)
	lid := new([]string)

	w.Header().Set("Content-Type", "application/json")

	id := -1

	for i := 0; i < len(ws.Waiting); i++ {
		id = -1
		id = isIn(ws.Waiting[i].ID, *lid)
		if id == -1 {
			newTTS := TaskToSend{ws.Waiting[i].Project, ws.Waiting[i].ID, 0.0, 0, ws.Waiting[i].StartTime}
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
			newTTS := TaskToSend{ws.Renders[i].myTask.Project, ws.Renders[i].myTask.ID, 0.0, 0, ws.Renders[i].myTask.StartTime}
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
			newTTS := TaskToSend{ws.Completed[i].Project, ws.Completed[i].ID, 0.0, 0, ws.Completed[i].StartTime}
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
