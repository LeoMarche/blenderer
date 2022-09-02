package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/LeoMarche/blenderer/src/render"
	"github.com/LeoMarche/blenderer/src/rendererapi"
)

var nameFlag = flag.String("n", "no_name", "The name of the worker")
var insecure = flag.Bool("i", false, "set this flag to allow insecure connections to API")
var folderFlag = flag.String("f", "", "by setting this flag, you can optionnally provide a folder not using the config file")

type configuration struct {
	API struct {
		Endpoint string
		Key      string
	}
	Fileserver  string
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
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal, exiting now ...")
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

func updateJob(APIendpoint, APIkey, state, name string, frame int, percent, mem float64, id string, target interface{}, client *http.Client) error {

	finalEndpoint := APIendpoint + "/updateJob"

	// Uses local self-signed cert
	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey},
		"id":      {id},
		"state":   {state},
		"name":    {name},
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

func setAvailable(APIendpoint, APIkey, name string, target interface{}, client *http.Client) error {

	finalEndpoint := APIendpoint + "/setAvailable"

	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey},
		"name":    {name}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func setDown(APIendpoint, APIkey, name string, target interface{}, client *http.Client) error {

	finalEndpoint := APIendpoint + "/setDown"

	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey},
		"name":    {name}})

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

func uploadFile(serverIP, ID, filepath string) error {
	c, err := net.Dial("tcp", serverIP)
	if err != nil {
		return err
	}
	st, err := os.Stat(filepath)
	if err != nil {
		return err
	}
	toSend := []byte("SEND " + strconv.FormatInt(st.Size(), 10) + " " + ID + " " + path.Base(filepath))
	_, err = c.Write(toSend)
	if err != nil {
		return err
	}
	src, err := os.Open(filepath)
	if err != nil {
		return err
	}

	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	if err != nil {
		return err
	}
	status := string(buf[:n])

	if status != "READY" {
		return fmt.Errorf("encountered status code %s instead of READY when trying to upload", status)
	}
	buf = make([]byte, 1024)
	var nRead int64
	nRead = 0

	for nRead != st.Size() {
		n, err := src.Read(buf)
		if err != nil {
			return err
		}
		nRead += int64(n)
		c.Write(buf[:n])
	}

	buf = make([]byte, 1024)
	n, err = c.Read(buf)
	if err != nil {
		return err
	}
	fin := string(buf[:n])

	if fin != "SUCCESS" {
		return fmt.Errorf("encountered bad status after finishing upload : %s instead of SUCCESS", fin)
	}
	return nil
}

func receiveFile(fileServer, id, srcFile, dstFolder string) error {
	c, err := net.Dial("tcp", fileServer)
	if err != nil {
		return err
	}
	defer c.Close()
	toSend := []byte("RECEIVE " + id + " " + srcFile)
	_, err = c.Write(toSend)
	if err != nil {
		return err
	}
	dst := path.Join(dstFolder, srcFile)
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	if err != nil {
		return err
	}
	instr := strings.Split(string(buf[:n]), " ")
	if len(instr) != 2 {
		return fmt.Errorf("expected instructions of length 2, got %d instead", len(instr))
	}
	status := instr[0]
	ln, err := strconv.ParseInt(instr[1], 10, 64)
	if err != nil {
		return err
	}

	if status != "READY" {
		return fmt.Errorf("expected status READY, got %s instead", status)
	}

	_, err = c.Write([]byte("GO"))
	if err != nil {
		return err
	}

	var nRead int64
	nRead = 0

	for nRead != ln {
		n, err := c.Read(buf)
		if err != nil {
			return err
		}
		nRead += int64(n)
		destination.Write(buf[:n])
	}

	_, err = c.Write([]byte("SUCCESS"))

	if err != nil {
		return err
	}

	return nil
}

func run(configPath string) {

	// Handler when exiting
	SetupCloseHandler()

	// Parsing the flags
	flag.Parse()

	if *nameFlag == "no_name" {
		log.Fatal("You must specify a name using -n")
	}

	//Retrieving configuration
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	if *folderFlag != "" {
		config.Folder = *folderFlag
	}

	if !path.IsAbs(config.Folder) {
		fmt.Printf("the folder path must be absolute (it appears that '%s' is not), replacing it with its absolute version ... \n", config.Folder)
		config.Folder, err = filepath.Abs(config.Folder)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("folder path translated to %s\n", config.Folder)
	}

	// Register the client on the master
	client := getClient()
	job := new(render.Task)
	resp := new(rendererapi.ReturnValue)
	err = postNode(config.API.Endpoint, config.API.Key, *nameFlag, resp, client)
	if err != nil || (resp.State != "Exists" && resp.State != "Added") {
		log.Fatalf("Error during initialization : %e, state : %s", err, resp.State)
	}

	rT := new(render.RendererTask)
	var state string
	var percent, mem float64

	for !mustStop {

		// Retrieve a job from the master
		err = getJob(config.API.Endpoint, config.API.Key, *nameFlag, job, client)
		if err != nil {
			log.Fatal(err)
		}

		if job.ID != "" {

			//Adapt the render.Task to fit reality
			outputFolder := filepath.Join(config.Folder, job.ID)
			os.MkdirAll(outputFolder, os.FileMode(0777))
			job.Input = filepath.Join(outputFolder, job.Input)
			job.Output = filepath.Join(outputFolder, job.Output)

			if _, err := os.Stat(job.Input); errors.Is(err, os.ErrNotExist) {
				err = receiveFile(config.Fileserver, job.ID, path.Base(job.Input), outputFolder)
				if err != nil {
					log.Fatalf("Error during receiving of file : %s", err.Error())
				}
			}

			//Create render task
			rT = job.MatchRenderer(config.Executables)

			if rT != nil {

				// Launch render if the rendering engine is present
				// TODO : verify the available rendering engines on the master before assigning
				pr, err := rT.LaunchRender()

				// If error during launching render, stop the client and put the node in error for the master
				if err != nil {
					rt := new(rendererapi.ReturnValue)
					errorNode(config.API.Endpoint, config.API.Key, *nameFlag, rt, client)
					log.Fatal(err)
				}

				go func() {
					err := pr.Wait()
					var rt rendererapi.ReturnValue
					if err != nil && !mustStop {
						errorNode(config.API.Endpoint, config.API.Key, *nameFlag, rt, client)
						log.Fatalf("Error during rendering : %e", err)
					}
				}()

				// Wait up to 100 seconds for the render engine to start
				for i := 0; i < 1000; i++ {
					state, percent, mem = rT.CheckRender()
					if state == "rendering" || state == "rendered" {
						break
					}
					time.Sleep(100 * time.Millisecond)
				}

				// Wait for the render to end (aborted or rendered)
				for state != "rendered" {
					s := new(returnvalue)
					rv := updateJob(config.API.Endpoint, config.API.Key, state, *nameFlag, rT.Task.Frame, percent, mem, rT.Task.ID, s, client)
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

					if mustStop {
						break
					}
				}

				state, percent, mem = rT.CheckRender()

				// Upload file if rendered
				if state == "rendered" {
					uploadFile(config.Fileserver, rT.Task.ID, rT.Task.Output+fmt.Sprintf("%05d", rT.Task.Frame)+".png")
				}

				// Try to update and abort process if aborted or problem
				s := new(returnvalue)
				rv := updateJob(config.API.Endpoint, config.API.Key, state, *nameFlag, rT.Task.Frame, percent, mem, rT.Task.ID, s, client)
				if rv != nil || s.State != "OK" {
					// Kill can't return useful errors
					pr.Process.Kill()
					setAvailable(config.API.Endpoint, config.API.Key, *nameFlag, s, client)
				}
			}

		} else {
			time.Sleep(1 * time.Second)
		}
	}

	// When asked to stop
	s := new(returnvalue)
	if job.ID != "" && state != "rendered" {

		// If a job was running, requeue it
		err = updateJob(config.API.Endpoint, config.API.Key, "requeue", *nameFlag, rT.Task.Frame, 0.0, 0.0, rT.Task.ID, s, client)
		if err != nil {
			fmt.Println(err)
		}
		if s.State != "REQUEUED" {
			fmt.Println(fmt.Errorf("couldn't requeue the task when quitting, state is %s", s.State))
		}
	}

	// Update the node state in the master
	err = setDown(config.API.Endpoint, config.API.Key, *nameFlag, s, client)
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	run("client.json")
}
