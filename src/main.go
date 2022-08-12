package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/LeoMarche/blenderer/src/filexchange"
	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/rendererapi"
	"github.com/LeoMarche/blenderer/src/rendererdb"

	fifo "github.com/foize/go.fifo"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

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
func startSequence(configPath string, nodesT *sync.Map, tasksT *sync.Map) (*sql.DB, rendererapi.Configuration) {
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

	//loading server API Keys
	nodesT.Range(func(k, v interface{}) bool {
		c.UserAPIKeys = append(c.UserAPIKeys, v.(*node.Node).APIKey)
		return true
	})

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
	var nodesT *sync.Map = new(sync.Map)
	var tasksT *sync.Map = new(sync.Map)
	var rendersT *sync.Map = new(sync.Map)

	//Initializing
	dB, cg := startSequence(configPath, nodesT, tasksT)

	fmt.Println("### Launching DB routine")
	transacts := fifo.NewQueue()
	stopDB := new(bool)
	*stopDB = false
	go rendererdb.DBTransactRoutines(dB, transacts, stopDB)

	ws := rendererapi.WorkingSet{
		Db:          dB,
		Config:      cg,
		Tasks:       tasksT,
		Renders:     rendersT,
		RenderNodes: nodesT,
		DBTransacts: transacts,
		StopDB:      stopDB,
	}

	fmt.Println("### Starting file server")
	b := new(bool)
	*b = false
	go filexchange.StartListening(ws.Config.Folder, b)

	fmt.Println("### Starting web server")
	handleRequests(&ws)
}

func main() {
	run("main.json")
}
