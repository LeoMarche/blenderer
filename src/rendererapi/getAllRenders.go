package rendererapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/LeoMarche/blenderer/src/render"
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

	//Count all waiting, uploading and completed frames
	ws.Tasks.Range(func(k, v interface{}) bool {
		v.(*sync.Map).Range(func(k2, v2 interface{}) bool {
			id = -1
			id = isIn(v2.(*render.Task).ID, *lid)
			if id == -1 {
				newTTS := TaskToSend{v2.(*render.Task).Project, v2.(*render.Task).ID, 0.0, 0, v2.(*render.Task).StartTime}
				*ret = append(*ret, newTTS)
				*lid = append(*lid, v2.(*render.Task).ID)
				id = len(*lid) - 1
			}
			if v2.(*render.Task).State == "uploading" || v2.(*render.Task).State == "waiting" {
				(*ret)[id].Nb++
			} else if v2.(*render.Task).State == "rendered" {
				(*ret)[id].Nb++
				(*ret)[id].Percent++
			}
			return true
		})
		return true
	})

	//Count all rendering frames
	ws.Renders.Range(func(k, v interface{}) bool {
		v.(*sync.Map).Range(func(k2, v2 interface{}) bool {
			rdr := v2.(*Render)
			id = -1
			id = isIn(rdr.myTask.ID, *lid)
			if id == -1 {
				newTTS := TaskToSend{rdr.myTask.Project, rdr.myTask.ID, 0.0, 0, rdr.myTask.StartTime}
				*ret = append(*ret, newTTS)
				*lid = append(*lid, rdr.myTask.ID)
				id = len(*lid) - 1
			}
			(*ret)[id].Nb++
			var s float64
			s, _ = strconv.ParseFloat(rdr.Percent, 64)
			(*ret)[id].Percent += s
			return true
		})
		return true
	})

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
