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
	"net/url"
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

	// RecordURLPrefix is the prefix of the path of requests to III catalogues for the permalink of a record.
	RecordPrefix string = "/record=b"

	// PatronInfoPrefix is the prefix of the path of requests to III catalogues for the patron login form.
	PatronInfoPrefix string = "/patroninfo"

	// SearchAuthorIndexPrefix is the prefix of the path of requests to III catalogues for the author search.
	SearchAuthorIndexPrefix string = "/search/a"

	// SearchPrefix is the prefix of the path of requests to III catalogues for the call number search.
	SearchCallNumberIndexPrefix string = "/search/c"

	// SearchTitleIndexPrefix is the prefix of the path of requests to III catalogues for the title search.
	SearchTitleIndexPrefix string = "/search/c"

	// SearchPrefix is the prefix of the path of requests to III catalogues for search results.
	SearchPrefix string = "/search/"

	// SearchResultRecordPrefix is the prefix used in III for seeing a record when browsing search results.
	SearchResultRecordPrefix string = "/search~S9"
)

// A version flag, which should be overwritten when building using ldflags.
var version = "devel"

// Detourer is a struct which stores the data needed to perform redirects.
type Detourer struct {
	idMap map[uint32]uint64 // The map of III BibIDs to ExL IDs.
	primo string            // The domain name (host) for the target Primo instance.
	vid   string            // The vid parameter to use when building Primo URLs.
}

// The Detourer serves HTTP redirects based on the request.
func (d Detourer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// In the default case, redirect to the Primo search form.
	redirectTo := &url.URL{
		Scheme: "https",
		Host:   d.primo,
		Path:   "/discovery/search",
	}

	// Depending on the prefix...
	switch {
	case strings.HasPrefix(r.URL.Path, RecordPrefix):
		buildRecordRedirect(redirectTo, r, d.idMap)
	case strings.HasPrefix(r.URL.Path, PatronInfoPrefix):
		redirectTo.Path = "/discovery/login"
	case strings.HasPrefix(r.URL.Path, SearchAuthorIndexPrefix):
		buildAuthorSearchRedirect(redirectTo, r)
	case strings.HasPrefix(r.URL.Path, SearchCallNumberIndexPrefix):
		buildCallNumberSearchRedirect(redirectTo, r)
	case strings.HasPrefix(r.URL.Path, SearchTitleIndexPrefix):
		buildTitleSearchRedirect(redirectTo, r)
	case strings.HasPrefix(r.URL.Path, SearchPrefix):
		buildSearchRedirect(redirectTo, r)
	case strings.HasPrefix(r.URL.Path, SearchResultRecordPrefix):
		// TODO
	}

	// Set the vid parameter on all redirects.
	setParamInURL(redirectTo, "vid", d.vid)

	// Send the redirect to the client.
	http.Redirect(w, r, redirectTo.String(), http.StatusMovedPermanently)
}

// buildRecordRedirect updates redirectTo to the correct Primo record URL for the requested bibID.
func buildRecordRedirect(redirectTo *url.URL, r *http.Request, idMap map[uint32]uint64) {
	// Convert everything after the RecordPrefix to a integer.
	bibID64, err := strconv.ParseUint(r.URL.Path[len(RecordPrefix):], 10, 32)
	if err == nil {
		bibID := uint32(bibID64)
		exlID, present := idMap[bibID]
		if present {
			redirectTo.Path = "/discovery/fulldisplay"
			setParamInURL(redirectTo, "docid", fmt.Sprintf("alma%v", exlID))
		}
	}
}

// buildAuthorSearchRedirect updates redirectTo to the correct Primo URL for the requested author search.
func buildAuthorSearchRedirect(redirectTo *url.URL, r *http.Request) {
	redirectTo.Path = "/discovery/browse"
	setParamInURL(redirectTo, "browseScope", "author")
	q := r.URL.Query()
	searchParam := q.Get("SEARCH")
	if searchParam != "" {
		setParamInURL(redirectTo, "browseQuery", searchParam)
	}
}

// buildCallNumberSearchRedirect updates redirectTo to the correct Primo URL for the requested call number search.
func buildCallNumberSearchRedirect(redirectTo *url.URL, r *http.Request) {
	redirectTo.Path = "/discovery/browse"
	setParamInURL(redirectTo, "browseScope", "callnumber.0")
	q := r.URL.Query()
	searchParam := q.Get("SEARCH")
	if searchParam != "" {
		setParamInURL(redirectTo, "browseQuery", searchParam)
	}
}

// buildTitleSearchRedirect updates redirectTo to the correct Primo URL for the requested title search.
func buildTitleSearchRedirect(redirectTo *url.URL, r *http.Request) {
	redirectTo.Path = "/discovery/browse"
	setParamInURL(redirectTo, "browseScope", "title")
	q := r.URL.Query()
	searchParam := q.Get("SEARCH")
	if searchParam != "" {
		setParamInURL(redirectTo, "browseQuery", searchParam)
	}
}

// buildSearchRedirect updates redirectTo to an approximate Primo URL for the requested search.
func buildSearchRedirect(redirectTo *url.URL, r *http.Request) {

	q := r.URL.Query()
	if q.Get("searcharg") == "" {
		return
	}
	redirectTo.Path = "/discovery/search"

	switch q.Get("searchtype") {
	case "t":
		setParamInURL(redirectTo, "query", fmt.Sprintf("title,contains,%v", q.Get("searcharg")))
	case "a":
		setParamInURL(redirectTo, "query", fmt.Sprintf("creator,contains,%v", q.Get("searcharg")))
	case "d":
		setParamInURL(redirectTo, "query", fmt.Sprintf("sub,contains,%v", q.Get("searcharg")))
	case "c":
		redirectTo.Path = "/discovery/browse"
		setParamInURL(redirectTo, "browseScope", "callnumber.0")
		setParamInURL(redirectTo, "browseQuery", q.Get("searcharg"))
	default:
		setParamInURL(redirectTo, "query", fmt.Sprintf("any,contains,%v", q.Get("searcharg")))
	}

	switch q.Get("searchdropdown") {
	case "t":
		setParamInURL(redirectTo, "sortby", "title")
	case "a":
		setParamInURL(redirectTo, "sortby", "author")
	case "c":
		setParamInURL(redirectTo, "sortby", "date_a")
	case "r":
		setParamInURL(redirectTo, "sortby", "date_d")
	}

	setParamInURL(redirectTo, "tab", "Everything")
	setParamInURL(redirectTo, "search_scope", "MyInst_and_CI")
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
		primo: fmt.Sprintf("%v.%v", *subdomain, PrimoDomain),
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
	// Split the input line into fields on commas.
	splitLine := strings.Split(line, ",")
	if len(splitLine) < 2 {
		return bibID, exlID, fmt.Errorf("Line has incorrect number of fields, 2 expected, %v found.\n", len(splitLine))
	}
	// The bibIDs look like this: a1234-instid
	// We need to strip off the first character and anything after the dash.
	dashIndex := strings.Index(splitLine[1], "-")
	if (dashIndex == 0) || (dashIndex == 1) {
		return bibID, exlID, fmt.Errorf("No bibID number was found before dash between bibID and institution id.\n")
	}
	bibIDString := "invalid"
	// If the dash isn't found, use the whole bibID field except the first character.
	if dashIndex == -1 {
		bibIDString = splitLine[1][1:]
	} else {
		bibIDString = splitLine[1][1:dashIndex]
	}
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

// setParamInURL is a helper function which sets a parameter in the query of a url.
func setParamInURL(redirectTo *url.URL, param, value string) {
	q := redirectTo.Query()
	q.Set(param, value)
	redirectTo.RawQuery = q.Encode()
}
