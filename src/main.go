package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/render"
	"github.com/LeoMarche/blenderer/src/rendererapi"
	"github.com/LeoMarche/blenderer/src/rendererdb"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

//Updates a slice of WorkingSet according of the expected type of the slice
func updateWSList(li *[]*render.Task, expState string) int {
	lr := new([]*render.Task)
	for i := 0; i < len((*li)); i++ {
		if (*li)[i].State == expState {
			*lr = append(*lr, (*li)[i])
		}
	}

	ret := len(*li) - len(*lr)
	*li = *lr
	return ret
}

//Updates a slice of Renders in WorkingSet
func updateWSRendersList(li *[]*rendererapi.Render, expState string) int {
	lr := new([]*rendererapi.Render)
	for i := 0; i < len((*li)); i++ {
		if (*li)[i].GetState() == expState {
			*lr = append(*lr, (*li)[i])
		}
	}

	ret := len(*li) - len(*lr)
	*li = *lr
	return ret
}

//Update all the slices of the WorkingSet
func updateWS(ws *rendererapi.WorkingSet) {
	for {
		time.Sleep(10 * time.Second)

		//Locking Mutexes
		ws.UploadingMutex.Lock()
		ws.WaitingMutex.Lock()
		ws.RendersMutex.Lock()
		ws.CompletedMutex.Lock()

		//Processing cleaning of lists
		uploading := updateWSList(&ws.Uploading, "uploading")
		waiting := updateWSList(&ws.Waiting, "waiting")
		completed := updateWSList(&ws.Completed, "rendered")
		rendering := updateWSRendersList(&ws.Renders, "rendering")

		//Unlocking Mutexes
		ws.CompletedMutex.Unlock()
		ws.RendersMutex.Unlock()
		ws.WaitingMutex.Unlock()
		ws.UploadingMutex.Unlock()
		fmt.Println("***")
		fmt.Printf("Cleaning process completed, removing %d from uploads, %d from waitings, %d from completed and %d from rendering !\n", uploading, waiting, completed, rendering)
		fmt.Printf("There is currently %d elements in uploads, %d in waiting, %d in completed and %d in rendering !\n", len(ws.Uploading), len(ws.Waiting), len(ws.Completed), len(ws.Renders))
	}
}

//This function loads the config from a json file
func loadConfig(configPath string) (rendererapi.Configuration, error) {

	var config rendererapi.Configuration
	configFile, err := os.Open(configPath)
	if err != nil {
		return config, err
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config, err
}

//This describes the start sequence of the program
func startSequence(configPath string, nodesT *[]*node.Node, tasksT *[]*render.Task) (*sql.DB, rendererapi.Configuration) {
	c, err := loadConfig(configPath)
	fmt.Println("	-> Config loaded")

	if err != nil {
		log.Fatal(err.Error())
	}

	dB, err := rendererdb.LoadDatabase(c.DBName)

	if err != nil {
		log.Fatal(err.Error())
	}
	fmt.Println("	-> Database created")

	//loading servers and tasks
	err = rendererdb.LoadNodeFromDB(dB, nodesT)

	if err != nil {
		log.Fatal(err.Error())
	}

	err = rendererdb.LoadTasksFromDB(dB, tasksT)

	if err != nil {
		log.Fatal(err.Error())
	}

	//loading server api_keys
	for i := 0; i < len(*nodesT); i++ {
		c.UserAPIKeys = append(c.UserAPIKeys, (*nodesT)[i].APIKey)
	}

	fmt.Println("	-> Lists loaded")
	return dB, c
}

//Our HTTP router
func handleRequests(ws *rendererapi.WorkingSet) {

	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.HandleFunc("/getAllRenderTasks", ws.GetAllRenderTasks)
	myRouter.HandleFunc("/getJob", ws.GetJob)
	myRouter.HandleFunc("/postJob", ws.PostJob)
	myRouter.HandleFunc("/updateJob", ws.UpdateJob)
	myRouter.HandleFunc("/uploadCompleted", ws.UploadCompleted)
	myRouter.HandleFunc("/abortJob", ws.AbortJob)
	myRouter.HandleFunc("/setAvailable", ws.SetAvailable)
	myRouter.HandleFunc("/postNode", ws.PostNode)
	myRouter.HandleFunc("/errorNode", ws.ErrorNode)

	log.Fatal(http.ListenAndServeTLS(":9000", ws.Config.Certname+".cert", ws.Config.Certname+".key", myRouter))
}

func run(configPath string) {

	fmt.Println("### Starting up !")

	//Initializing working arrays
	var nodesT *[]*node.Node = new([]*node.Node)
	var tasksT *[]*render.Task = new([]*render.Task)

	//Initializing
	dB, cg := startSequence(configPath, nodesT, tasksT)

	ws := rendererapi.WorkingSet{
		Db:          dB,
		Config:      cg,
		RenderNodes: *nodesT,
	}

	fmt.Println("### Populating WorkingSet")

	for i := 0; i < len(*tasksT); i++ {

		switch st := (*tasksT)[i].State; st {
		case "uploading":
			ws.Uploading = append(ws.Uploading, (*tasksT)[i])
		case "waiting":
			ws.Waiting = append(ws.Waiting, (*tasksT)[i])
		case "rendering":
			(*tasksT)[i].State = "waiting"
			ws.Waiting = append(ws.Waiting, (*tasksT)[i])
		case "completed":
			ws.Completed = append(ws.Completed, (*tasksT)[i])
		}
	}

	fmt.Println("### Starting background tasks updates")
	go updateWS(&ws)

	fmt.Println("### Starting web server")
	handleRequests(&ws)
}

func main() {
	run("main.json")
}
