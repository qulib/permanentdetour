// Copyright 2019 Carleton University Library All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// EnvPrefix is the prefix for the environment variables.
	EnvPrefix string = "PERMANENTDETOUR_"

	// DefaultAddress is the default address to serve from.
	DefaultAddress string = ":8877"

	// MaxMappingFileLength is the maximum number of lines in a mapping file.
	MaxMappingFileLength uint64 = 1000000

	// RecordURLPrefix is the prefix used in III for simple permalinks to records
	RecordURLPrefix string = "/record=b"

	// PatronInfoPrefix is the prefix used in III for the login form.
	PatronInfoPrefix string = "/patroninfo"

	// SearchPrefix is the prefix used in III for search requests.
	// SearchPrefix string = "/search"
)

// A version flag, which should be overwritten when building using ldflags.
var version = "devel"

type Detourer struct {
	m              map[uint32]uint64
	maxBibIDLength int
	primoaddress   string
	vid            string
}

// The Detourer serves HTTP redirects based on the request.
func (d Detourer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	redirectTo := fmt.Sprintf("%vdiscovery/search?vid=%v", d.primoaddress, d.vid)
	if strings.HasPrefix(r.URL.Path, RecordURLPrefix) {
		pathBibID := r.URL.Path[len(RecordURLPrefix):]
		if len(pathBibID) <= d.maxBibIDLength {
			bibID64, err := strconv.ParseUint(pathBibID, 10, 32)
			if err == nil {
				bibID := uint32(bibID64)
				exlID, present := d.m[bibID]
				if present {
					redirectTo = fmt.Sprintf("%vfulldisplay?vid=%v&docid=alma%v", d.primoaddress, d.vid, exlID)
				}
			}
		}
	} else if strings.HasPrefix(r.URL.Path, PatronInfoPrefix) {
		redirectTo = fmt.Sprintf("%vlogin?vid=%v", d.primoaddress, d.vid)
	}
	http.Redirect(w, r, redirectTo, http.StatusMovedPermanently)
}

func main() {

	addr := flag.String("address", DefaultAddress, "Address to bind on.")
	primoaddress := flag.String("primoaddress", "", "The URL of the target Primo instance. Ex: https://???.primo.exlibrisgroup.com/discovery/ Required.")
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
	overrideUnsetFlagsFromEnvironmentVariables()

	if *primoaddress == "" {
		log.Fatalln("A primoaddress is required.")
	}
	if *vid == "" {
		log.Fatalln("A vid is required.")
	}

	// The Detourer has all the data needed to build redirects.
	var d Detourer
	d.primoaddress = *primoaddress
	d.vid = *vid

	// Map of III BibIDs to ExL IDs
	// The initial size is an estimate based on the number of arguments.
	size := uint64(len(flag.Args())) * MaxMappingFileLength
	d.m = make(map[uint32]uint64, size)

	// Process each file in the arguments list.
	for _, mappingFile := range flag.Args() {

		// Get the absolute path of the file. Not strictly necessary.
		filePath, err := filepath.Abs(mappingFile)
		if err != nil {
			log.Fatalf("Could not get absolute path of %v, %v.\n", mappingFile, err)
		}

		// Open the file for reading. Close the file automatically when done.
		file, err := os.Open(filePath)
		if err != nil {
			log.Fatal("Could not open %v for reading, %v.\n", filePath, err)
		}
		defer file.Close()

		// Read the file line by line.
		scanner := bufio.NewScanner(file)
		lnum := 0
		for scanner.Scan() {
			lnum += 1
			bibID, bibIDLength, exlID, err := processLine(scanner.Text())
			if err != nil {
				log.Fatalln("Unable to process line %v '%v', '%v'.\n", lnum, scanner.Text(), err)
			}
			_, present := d.m[bibID]
			if present {
				log.Fatalln("Previously seen Bib ID %v was encountered.\n", bibID)
			}
			d.m[bibID] = exlID
			if bibIDLength > d.maxBibIDLength {
				d.maxBibIDLength = bibIDLength
			}
		}
		err = scanner.Err()
		if err != nil {
			log.Fatal("Could not open %v for reading, %v.\n", filePath, err)
		}
	}

	http.Handle("/", d)

	log.Printf("%v III BibID to Ex Libris ID mappings processed.\n", len(d.m))
	log.Println("Starting server...")
	log.Fatalf("FATAL: %v", http.ListenAndServe(*addr, nil))
}

func processLine(line string) (bibID uint32, bibIDLength int, exlID uint64, _ error) {
	splitLine := strings.Split(line, ",")
	dashIndex := strings.Index(splitLine[1], "-")
	bibIDString := splitLine[1][1:dashIndex]
	bibIDLength = len(bibIDString)
	bibID64, err := strconv.ParseUint(bibIDString, 10, 32)
	if err != nil {
		return bibID, bibIDLength, exlID, err
	}
	bibID = uint32(bibID64)
	exlID, err = strconv.ParseUint(splitLine[0], 10, 64)
	if err != nil {
		return bibID, bibIDLength, exlID, err
	}
	return bibID, bibIDLength, exlID, nil
}

// If any flags are not set, use environment variables to set them.
func overrideUnsetFlagsFromEnvironmentVariables() {

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
				log.Fatalf("FATAL: Unable to set configuration option %v from environment variable %v, "+
					"which has a value of \"%v\"",
					k.Name, environmentVariableName, environmentVariableValue)
			}
		}
	}
}
