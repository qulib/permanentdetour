// Copyright 2019 Carleton University Library All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	// EnvPrefix is the prefix for the environment variables.
	EnvPrefix string = "PERMANENTDETOUR_"

	// DefaultAddress is the default address to serve from.
	DefaultAddress string = ":8877"

	// PrimoDomain is the domain at which Primo instances are hosted.
	PrimoDomain string = "primo.exlibrisgroup.com"

	// MaxMappingFileLength is the maximum number of lines in a mapping file.
	MaxMappingFileLength uint64 = 1000000

	// RecordURLPrefix is the prefix used in III for simple permalinks to records
	RecordURLPrefix string = "/record=b"

	// PatronInfoPrefix is the prefix used in III for the login form.
	PatronInfoPrefix string = "/patroninfo"

	// SearchPrefix is the prefix used in III for search requests.
	SearchPrefix string = "/search"
)

// A version flag, which should be overwritten when building using ldflags.
var version = "devel"

// Detourer is a struct which stores the data needed to perform redirects.
type Detourer struct {
	idMap map[uint32]uint64 // The map of III BibIDs to ExL IDs.
	primo string            // The scheme (https) and domain name for the target Primo instance.
	vid   string            // The vid parameter to use when building Primo URLs.
}

// The Detourer serves HTTP redirects based on the request.
func (d Detourer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// In the default case, redirect to the Primo search form.
	redirectTo := fmt.Sprintf("%v/discovery/search?vid=%v", d.primo, d.vid)

	// Depending on the prefix...
	switch {
	case strings.HasPrefix(r.URL.Path, RecordURLPrefix):
		bibID64, err := strconv.ParseUint(r.URL.Path[len(RecordURLPrefix):], 10, 32)
		if err == nil {
			bibID := uint32(bibID64)
			exlID, present := d.idMap[bibID]
			if present {
				redirectTo = fmt.Sprintf("%v/discovery/fulldisplay?vid=%v&docid=alma%v", d.primo, d.vid, exlID)
			}
		}
	case strings.HasPrefix(r.URL.Path, PatronInfoPrefix):
		redirectTo = fmt.Sprintf("%v/discovery/login?vid=%v", d.primo, d.vid)
	case strings.HasPrefix(r.URL.Path, SearchPrefix):
		// TODO
	}

	// Send the redirect to the client.
	http.Redirect(w, r, redirectTo, http.StatusMovedPermanently)
}

func main() {

	// Define the command line flags.
	addr := flag.String("address", DefaultAddress, "Address to bind on.")
	subdomain := flag.String("primo", "", "The subdomain of the target Primo instance, ?????.primo.exlibrisgroup.com. Required.")
	vid := flag.String("vid", "", "VID parameter for Primo. Required.")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Permanent Detour: A tiny web service which redirects Sierra Web OPAC requests to Primo URLs.\n")
		fmt.Fprintf(os.Stderr, "Version %v\n", version)
		fmt.Fprintf(os.Stderr, "Usage: permanentdetour [flag...] [file...]\n")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "  Environment variables read when flag is unset:")

		flag.VisitAll(func(f *flag.Flag) {
			uppercaseName := strings.ToUpper(f.Name)
			fmt.Fprintf(os.Stderr, "  %v%v\n", EnvPrefix, uppercaseName)
		})
	}

	// Process the flags.
	flag.Parse()

	// If any flags have not been set, see if there are
	// environment variables that set them.
	err := overrideUnsetFlagsFromEnvironmentVariables()
	if err != nil {
		log.Fatalln(err)
	}

	if *subdomain == "" {
		log.Fatalln("A primo subdomain is required.")
	}
	if *vid == "" {
		log.Fatalln("A vid is required.")
	}

	// The Detourer has all the data needed to build redirects.
	d := Detourer{
		primo: fmt.Sprintf("https://%v.%v", *subdomain, PrimoDomain),
		vid:   *vid,
	}

	// Map of III BibIDs to ExL IDs
	// The initial size is an estimate based on the number of arguments.
	size := uint64(len(flag.Args())) * MaxMappingFileLength
	d.idMap = make(map[uint32]uint64, size)

	// Process each file in the arguments list.
	for _, mappingFilePath := range flag.Args() {
		// Add the mappings from this file to the idMap.
		err := processFile(d.idMap, mappingFilePath)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("%v III BibID to Ex Libris ID mappings processed.\n", len(d.idMap))

	// Use an explicit request multiplexer.
	mux := http.NewServeMux()
	mux.Handle("/", d)

	server := http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	shutdown := make(chan struct{})
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		// Wait to receive a message on the channel.
		<-sigs
		err := server.Shutdown(context.Background())
		if err != nil {
			log.Printf("Error shutting down server, %v.\n", err)
		}
		close(shutdown)
	}()

	log.Println("Starting server.")
	err = server.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatalf("Fatal server error, %v.\n", err)
	}

	<-shutdown
	log.Println("Server stopped.")
}

// processFile takes a file path, opens the file, and reads it line by line to extract id mappings.
func processFile(m map[uint32]uint64, mappingFilePath string) error {
	// Get the absolute path of the file. Not strictly necessary, but creates clearer error messages.
	absFilePath, err := filepath.Abs(mappingFilePath)
	if err != nil {
		return fmt.Errorf("Could not get absolute path of %v, %v.\n", mappingFilePath, err)
	}

	// Open the file for reading. Close the file automatically when done.
	file, err := os.Open(absFilePath)
	if err != nil {
		return fmt.Errorf("Could not open %v for reading, %v.\n", absFilePath, err)
	}
	defer file.Close()

	// Read the file line by line.
	scanner := bufio.NewScanner(file)
	lnum := 0
	for scanner.Scan() {
		lnum += 1
		bibID, exlID, err := processLine(scanner.Text())
		if err != nil {
			return fmt.Errorf("Unable to process line %v '%v', %v.\n", lnum, scanner.Text(), err)
		}
		_, present := m[bibID]
		if present {
			return fmt.Errorf("Previously seen Bib ID %v was encountered.\n", bibID)
		}
		m[bibID] = exlID
	}
	err = scanner.Err()
	if err != nil {
		return fmt.Errorf("Scanner error when processing %v, %v.\n", absFilePath, err)
	}
	return nil
}

// processLine takes a line of input, and finds the III bib ID and the exL ID.
func processLine(line string) (bibID uint32, exlID uint64, _ error) {
	splitLine := strings.Split(line, ",")
	dashIndex := strings.Index(splitLine[1], "-")
	bibIDString := splitLine[1][1:dashIndex]
	bibID64, err := strconv.ParseUint(bibIDString, 10, 32)
	if err != nil {
		return bibID, exlID, err
	}
	bibID = uint32(bibID64)
	exlID, err = strconv.ParseUint(splitLine[0], 10, 64)
	if err != nil {
		return bibID, exlID, err
	}
	return bibID, exlID, nil
}

// If any flags are not set, use environment variables to set them.
func overrideUnsetFlagsFromEnvironmentVariables() error {

	// A map of pointers to unset flags.
	listOfUnsetFlags := make(map[*flag.Flag]bool)

	// flag.Visit calls a function on "only those flags that have been set."
	// flag.VisitAll calls a function on "all flags, even those not set."
	// No way to ask for "only unset flags". So, we add all, then
	// delete the set flags.

	// First, visit all the flags, and add them to our map.
	flag.VisitAll(func(f *flag.Flag) { listOfUnsetFlags[f] = true })

	// Then delete the set flags.
	flag.Visit(func(f *flag.Flag) { delete(listOfUnsetFlags, f) })

	// Loop through our list of unset flags.
	// We don't care about the values in our map, only the keys.
	for k := range listOfUnsetFlags {

		// Build the corresponding environment variable name for each flag.
		uppercaseName := strings.ToUpper(k.Name)
		environmentVariableName := fmt.Sprintf("%v%v", EnvPrefix, uppercaseName)

		// Look for the environment variable name.
		// If found, set the flag to that value.
		// If there's a problem setting the flag value,
		// there's a serious problem we can't recover from.
		environmentVariableValue := os.Getenv(environmentVariableName)
		if environmentVariableValue != "" {
			err := k.Value.Set(environmentVariableValue)
			if err != nil {
				fmt.Errorf("Unable to set configuration option %v from environment variable %v, "+
					"which has a value of \"%v\"",
					k.Name, environmentVariableName, environmentVariableValue)
			}
		}
	}
	return nil
}
