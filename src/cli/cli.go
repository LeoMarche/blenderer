package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/LeoMarche/blenderer/src/rendererapi"
)

var certPath = flag.String("c", "../host.cert", "Path to the SSL cert for connecting to the api")
var apiKey = flag.String("k", "sample_unsecure", "API key to use to connect to the API")
var URL = flag.String("u", "https://localhost:9000", "URL to use to connect to the API")
var fileServer = flag.String("fs", "localhost:9005", "IP and port of the fileserver distributing the render files")
var insecure = flag.Bool("i", false, "set this flag to allow insecure connections to API")

func initialize() *http.Client {
	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Read in the cert file
	certs, err := ioutil.ReadFile(*certPath)
	if err != nil {
		log.Fatalf("Failed to append %q to RootCAs: %v", *certPath, err)
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

func postJob(APIendpoint, APIkey, project, input, output, frameStart, frameStop, rendererName, rendererVersion, startTime string, client *http.Client, target interface{}) error {
	finalEndpoint := APIendpoint + "/postJob"

	// Uses local self-signed cert
	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key":         {APIkey},
		"project":         {project},
		"input":           {input},
		"output":          {output},
		"frameStart":      {frameStart},
		"frameStop":       {frameStop},
		"rendererName":    {rendererName},
		"rendererVersion": {rendererVersion},
		"startTime":       {startTime}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

//id, api_key, size and project
func uploadCompleted(APIendpoint, APIkey, project, id, size, input string, client *http.Client, target interface{}) error {
	finalEndpoint := APIendpoint + "/uploadCompleted"

	// Uses local self-signed cert
	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey},
		"project": {project},
		"id":      {id},
		"size":    {size},
		"input":   {input}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

//id, api_key, size and project
func getAllRenders(APIendpoint, APIkey string, client *http.Client, target interface{}) error {
	finalEndpoint := APIendpoint + "/getAllRenderTasks"

	// Uses local self-signed cert
	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func customUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage : \n")

	fmt.Fprintf(flag.CommandLine.Output(), "    ./cli <flags> <operation>\n\n")

	fmt.Fprintf(flag.CommandLine.Output(), "Flags : \n")

	flag.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(flag.CommandLine.Output(), "    - %s <%s>, description : %s\n", f.Name, f.Value, f.Usage)
	})

	fmt.Fprint(flag.CommandLine.Output(), "\n")

	operationsHelp := `Operations :
    post-job <file> <frameStart> <frameStop> <rendererName> <rendererVersion>
        Description:
            Posts a new render to the rendering system
        Arguments:
            <file> : path to the file that needs to be posted
            <frameStart> : number of the initial frame to render
            <frameStop> : number of the last frame to render
            <rendererName> : name of the renderer to use
            <rendererVersion> : version of the renderer to use
    get-all
        Description:
            Get all tasks handled by the rendering system and returns stats

`
	fmt.Fprint(flag.CommandLine.Output(), operationsHelp)

	examplesHelp := `Examples :
    Post a new render:
        ./cli -c dummy_cert.cert -fs fil.server:9005 -i -k secured_key -u api.server:9000 post-job dummy.blend 1 5 blender 2.91.0
    Get stats on renders:
        ./cli -c dummy_cert.cert -fs fil.server:9005 -i -k secured_key -u api.server:9000 get-all

`

	fmt.Fprint(flag.CommandLine.Output(), examplesHelp)

}

func main() {

	flag.Usage = customUsage

	flag.Parse()
	client := initialize()

	argTab := flag.Args()

	command := argTab[0]

	switch command {
	case "post-job":
		// check if the correct number of arguments was given
		if len(argTab) != 6 {
			log.Fatal(fmt.Errorf("post-job called with %d arguments instead of 6", len(argTab)))
		}

		fPath := argTab[1]
		frameStart := argTab[2]
		frameStop := argTab[3]
		rName := argTab[4]
		rVer := argTab[5]

		// Verify that frames can be casted to ints
		if _, err := strconv.Atoi(frameStart); err != nil {
			log.Fatal(fmt.Errorf("post-job called with frameStart=%s which doesn't looks like an int", frameStart))
		}
		if _, err := strconv.Atoi(frameStop); err != nil {
			log.Fatal(fmt.Errorf("post-job called with frameStop=%s which doesn't looks like an int", frameStop))
		}

		startTime := strconv.FormatInt(time.Now().UnixNano(), 10)
		up := new(rendererapi.Upload)
		err := postJob(*URL, *apiKey, path.Base(fPath), path.Base(fPath), path.Base(fPath), frameStart, frameStop, rName, rVer, startTime, client, up)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Task created, token/ID : %s, project : %s, current state : %s\n", up.Token, up.Project, up.State)
		err = uploadFile(*fileServer, up.Token, fPath)
		if err != nil {
			log.Fatal(err)
		}
		st, err := os.Stat(fPath)
		if err != nil {
			log.Fatal(err)
		}
		size := strconv.FormatInt(st.Size(), 10)
		ID := up.Token
		rv := new(rendererapi.ReturnValue)
		err = uploadCompleted(*URL, *apiKey, path.Base(fPath), ID, size, path.Base(fPath), client, rv)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Upload done, current state : %s\n", rv.State)

	case "get-all":
		//Check the number of arguments to call get-all
		if len(argTab) != 1 {
			log.Fatal(fmt.Errorf("get-all called with %d arguments instead of 1", len(argTab)))
		}

		ret := new([]rendererapi.TaskToSend)
		err := getAllRenders(*URL, *apiKey, client, ret)
		if err != nil {
			log.Fatal(err)
		} else {
			fmt.Println(*ret)
		}
	}
}
