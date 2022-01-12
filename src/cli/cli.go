package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LeoMarche/blenderer/src/rendererapi"
)

var certPath = flag.String("c", "host.cert", "Path to the SSL cert for connecting to the api")
var apiKey = flag.String("k", "unsecure_sample", "API key to use to connect to the API")
var URL = flag.String("u", "localhost:9000", "URL to use to connect to the API")
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

func getInput(reader *bufio.Reader) string {
	text, _ := reader.ReadString('\n')
	// convert CRLF to LF
	text = strings.Replace(text, "\n", "", -1)
	return text
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
func uploadCompleted(APIendpoint, APIkey, project, id, size string, client *http.Client, target interface{}) error {
	finalEndpoint := APIendpoint + "/uploadCompleted"

	// Uses local self-signed cert
	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey},
		"project": {project},
		"id":      {id},
		"size":    {size}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

//id, api_key, size and project
func getAllRenders(APIendpoint, APIkey string, client *http.Client, target interface{}) error {
	finalEndpoint := APIendpoint + "/getAllRenders"

	// Uses local self-signed cert
	resp, err := client.PostForm(finalEndpoint, url.Values{
		"api_key": {APIkey}})

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func main() {
	flag.Parse()
	client := initialize()
	stop := false
	reader := bufio.NewReader(os.Stdin)

	for !stop {
		fmt.Println(
			"Enter 'postJob' to post a new job",
			"Enter 'uploadCompleted' to tell the API an upload is complete",
			"Enter 'getAllRenders' to get all renders",
		)
		fmt.Print("-> ")

		text := getInput(reader)

		switch text {
		case "postJob":
			fmt.Print("project Name : ")
			pName := getInput(reader)
			fmt.Print("Input Name : ")
			iName := getInput(reader)
			fmt.Print("Output Name : ")
			oName := getInput(reader)
			fmt.Print("frameStart : ")
			fStart := getInput(reader)
			fmt.Print("frameStop : ")
			fStop := getInput(reader)
			fmt.Print("renderer Name : ")
			rName := getInput(reader)
			fmt.Print("renderer Version : ")
			rVer := getInput(reader)
			startTime := strconv.FormatInt(time.Now().UnixNano(), 10)
			up := new(rendererapi.Upload)
			err := postJob(*URL, *apiKey, pName, iName, oName, fStart, fStop, rName, rVer, startTime, client, up)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(up.Token, up.Project, up.State)
		case "uploadCompleted":
			fmt.Print("project Name : ")
			pName := getInput(reader)
			fmt.Print("Token : ")
			id := getInput(reader)
			fmt.Print("size : ")
			size := getInput(reader)
			rv := new(rendererapi.ReturnValue)
			err := uploadCompleted(*URL, *apiKey, pName, id, size, client, rv)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(rv.State)
		case "getAllRenders":
			ret := new([]rendererapi.TaskToSend)
			err := getAllRenders(*URL, *apiKey, client, ret)
			if err != nil {
				log.Fatal(err)
			} else {
				fmt.Println("all good")
			}

		}
	}
}
