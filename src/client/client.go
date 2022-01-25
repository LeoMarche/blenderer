package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/LeoMarche/blenderer/src/render"
	"github.com/LeoMarche/blenderer/src/rendererapi"
)

var nameFlag = flag.String("n", "no_name", "The name of the worker")
var insecure = flag.Bool("i", false, "set this flag to allow insecure connections to API")

type configuration struct {
	API struct {
		Endpoint string
		Key      string
	}
	Folder      string
	Certfile    string
	Executables []render.Renderer
}

type returnvalue struct {
	State string
}

const (
	localCertFile = "../host.cert"
)

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	fmt.Println(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	buf := make([]byte, 1024)
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return 0, err
		}
		if n == 0 {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
			return 0, err
		}
	}
	return 0, err
}

var mustStop = false

func loadConfig(configPath string) (configuration, error) {

	var config configuration
	configFile, err := os.Open(configPath)
	if err != nil {
		return config, err
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config, err
}

//SetupCloseHandler setups a handler for os interrupt
func SetupCloseHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		mustStop = true
	}()
}

func getClient() *http.Client {

	flag.Parse()

	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Read in the cert file
	certs, err := ioutil.ReadFile(localCertFile)
	if err != nil {
		log.Fatalf("Failed to append %q to RootCAs: %v", localCertFile, err)
	}

	// Append our cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		log.Println("No certs appended, using system certs only")
	}

	// Trust the augmented cert pool in our client
	config := &tls.Config{
		InsecureSkipVerify: *insecure,
		RootCAs:            rootCAs,
	}
	tr := &http.Transport{TLSClientConfig: config}
	return &http.Client{Transport: tr}
}

func getJob(APIendpoint string, APIkey string, name string, target interface{}, client *http.Client) error {

	finalEndpoint := APIendpoint + "/getJob"

	// Uses local self-signed cert
	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey},
		"name":    {name},
	})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func updateJob(APIendpoint, APIkey, state string, frame int, percent, mem float64, id string, target interface{}, client *http.Client) error {

	finalEndpoint := APIendpoint + "/updateJob"

	// Uses local self-signed cert
	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey},
		"id":      {id},
		"state":   {state},
		"frame":   {strconv.Itoa(frame)},
		"percent": {strconv.FormatFloat(percent, 'f', -1, 64)},
		"mem":     {strconv.FormatFloat(mem, 'f', -1, 64)}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func postNode(APIendpoint, APIkey, name string, target interface{}, client *http.Client) error {
	finalEndpoint := APIendpoint + "/postNode"

	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey},
		"name":    {name}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func setAvailable(APIendpoint, APIkey string, target interface{}, client *http.Client) error {

	finalEndpoint := APIendpoint + "/setAvailable"

	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func errorNode(APIendpoint, APIkey, name string, target interface{}, client *http.Client) error {
	finalEndpoint := APIendpoint + "/errorNode"

	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey},
		"name":    {name}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func run(configPath string) {

	flag.Parse()

	if *nameFlag == "no_name" {
		log.Fatal("You must specify a name using -n")
	}

	//Retrieving configuration
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
	client := getClient()
	job := new(render.Task)

	resp := new(rendererapi.ReturnValue)

	err = postNode(config.API.Endpoint, config.API.Key, *nameFlag, resp, client)

	if err != nil || (resp.State != "Exists" && resp.State != "Added") {
		log.Fatalf("Error during initialization : %e, state : %s", err, resp.State)
	}

	for !mustStop {
		err = getJob(config.API.Endpoint, config.API.Key, *nameFlag, job, client)

		if err != nil {
			log.Fatal(err)
		}

		if job.ID != "" {

			//Adapt the render.Task to fit reality
			outputFolder := filepath.Join(config.Folder, job.ID)
			os.MkdirAll(outputFolder, os.ModePerm)
			job.Input = filepath.Join(outputFolder, job.Input)
			job.Output = filepath.Join(outputFolder, job.Output)

			//Create render task
			rT := job.MatchRenderer(config.Executables)

			if rT != nil {
				pr, err := rT.LaunchRender()

				go func() {
					err := pr.Wait()
					var rt rendererapi.ReturnValue
					if err != nil {
						errorNode(config.API.Endpoint, config.API.Key, *nameFlag, rt, client)
						log.Fatalf("Error during rendering : %e", err)
					}
				}()

				if err != nil {
					log.Fatal(err)
				}

				var state string
				var percent, mem float64

				for i := 0; i < 1000; i++ {
					state, percent, mem = rT.CheckRender()
					if state == "rendering" || state == "rendered" {
						break
					}
					time.Sleep(100 * time.Millisecond)
				}

				for state != "rendered" {
					s := new(returnvalue)
					rv := updateJob(config.API.Endpoint, config.API.Key, state, rT.Task.Frame, percent, mem, rT.Task.ID, s, client)
					if rv != nil || s.State != "OK" {

						//If aborting render
						switch st := s.State; st {
						case "ABORT":
							fmt.Println("Order from master to abort render")
						default:
							fmt.Println(st)
						}

						fmt.Println("Task is going to be cancelled")
						break
					}
					time.Sleep(1 * time.Second)
					state, percent, mem = rT.CheckRender()
				}

				state, percent, mem = rT.CheckRender()
				s := new(returnvalue)
				rv := updateJob(config.API.Endpoint, config.API.Key, state, rT.Task.Frame, percent, mem, rT.Task.ID, s, client)
				if rv != nil || s.State != "OK" {
					if err := pr.Process.Kill(); err != nil {
						log.Fatal("failed to kill process: ", err)
					}
					setAvailable(config.API.Endpoint, config.API.Key, s, client)
				}
			}

		} else {
			time.Sleep(5 * time.Second)
		}
	}

}

func main() {

	run("client.json")
}
