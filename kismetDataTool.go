package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"kismetDataTool/kismetClient"
	"log"
	"net/url"
	"os"
	"strings"
)

var (
	// Flags
	kismetUrl string
	kismetDB string
	filterSpec string

	help      bool
	debug     bool

	// Program vars
	kismetUsername string
	kismetPassword string

	dbMode bool
	restMode bool

	dlog      *log.Logger
	ilog      *log.Logger
)

func init() {
	const (
		urlUsage   = "Used to identify the URL to access the Kismet REST API"
		dbUsage = "Used to identify a local Kismet sqlite3 database file"
		filterUsage = "This flag is used to set a filter for the Kismet REST API if the -restURL " +
			"flag is used, **or** this flag is used to set a filter for a kismet sqlite3 database. " +
			"When using the -restURL flag, filters must be specified space delineated in a single string. " +
			"See /system/tracked_fields.html for a list of fields that this " +
			"program will use to filter requests and results. " +
			"When using the -dbFile flag, filters must be specified by their column names " +
			"in their respective tables. A valid dbFile filter might look like the following: " +
			"`devices/devmac devices/avg_lat` etc. All dbFile filters must specify the same table."

		helpUsage  = "Display this help info and exit"
		debugUsage = "Enable debug output"

		debugDefault = true
	)

	flag.StringVar(&kismetDB, "dbFile", "", dbUsage)
	flag.StringVar(&kismetUrl, "restUrl", "", urlUsage)
	flag.StringVar(&filterSpec, "filter", "", filterUsage)

	flag.BoolVar(&help, "help", false, helpUsage)
	flag.BoolVar(&debug, "verbose", debugDefault, debugUsage)

}

func main() {
	ilog = log.New(os.Stdout, "", 0)
	defer ilog.Println("Exiting. Have a good day! (っ◕‿◕)っ")

	flag.Parse()
	if help {
		flag.PrintDefaults()
		return
	}

	if debug {
		dlog = log.New(os.Stderr, "DEBUG: ", log.Ltime)
	} else {
		dlog = log.New(ioutil.Discard, "", 0)
	}

	defer dlog.Println("FINISH")

	dlog.Println("Parsing command line options")
	if kismetUrl == kismetDB {
		flag.PrintDefaults()
		ilog.Println("Please choose either database or rest mode.")
		return
	} else if kismetUrl != "" {
		restMode = true
	} else {
		dbMode = true
	}

	if dbMode {
		doDB()
	} else { // REST mode

		// Test the url and filter flags before prompting for username and password
		ilog.Println("Kismet URL:", kismetUrl)
		if testUrl, err := url.Parse(kismetUrl) ; err == nil {
			if !(testUrl.Scheme == "http") && !(testUrl.Scheme == "https") {
				flag.PrintDefaults()
				dlog.Println("URL does not appear to have http or https protocol:", testUrl.Scheme)
				ilog.Println("Please enter a valid `http` or `https` url")
				return
			}
		} else {
			flag.PrintDefaults()
			dlog.Println("Failed to create url:", err)
			ilog.Println("Please enter a valid `http` or `https` url")
			return
		}

		// Basic check. If they are bad filters, let kismet error out instead of us :D
		if filterSpec == "" {
			flag.PrintDefaults()
			ilog.Println("Please specify filters for rest calls")
			return
		}

		// Get kismet username and password
		fmt.Print("Kismet username: ")
		if _, err := fmt.Scanf("%s", &kismetUsername) ; err != nil {
			ilog.Println("Failed to read username")
			return
		}

		fmt.Print("Kismet password: ")
		if _, err := fmt.Scanf("%s", &kismetPassword) ; err != nil {
			ilog.Println("Failed to read password")
			return
		}

		// Test the username and password parameters
		if kismetUsername == "" || kismetPassword == "" {
			flag.PrintDefaults()
			ilog.Println("You must specify a username and password!")
			return
		}

		doRest()
	}
}

func doRest() {
	var (
		filters = strings.Split(filterSpec, " ")
		kClient kismetClient.KismetRestClient
	)

	dlog.Println("Creating Kismet client")

	if newKClient, err := kismetClient.NewRestClient(kismetUrl, kismetUsername, kismetPassword, filters) ; err == nil {
		dlog.Println("Successfully created kismet client")
		kClient = newKClient
		defer kClient.Finish()
	} else {
		ilog.Printf("Failed to create kismet client: %v\n", err)
		return
	}

	printElems(&kClient)
}

func doDB() {
	var (
		dbClient kismetClient.KismetDBClient
		filters []string
		table string
		columns []string
	)

	// get teh filters
	filters = strings.Split(filterSpec, " ")

	columns = make([]string, 0)
	for _, v := range filters {
		subFilter := strings.Split(v, "/")
		if len(subFilter) != 2 {
			ilog.Println("Bad DB Filter:", v)
			return
		}

		newTable := subFilter[0]
		columns = append(columns, subFilter[1])
		if table == "" {
			table = newTable
		} else if table != newTable {
			ilog.Println("Bad DB Filter:", v)
			return
		}
	}

	// get teh client
	if newClient, err := kismetClient.NewDBClient(kismetDB, table, columns) ; err == nil {
		dbClient = newClient
		defer dbClient.Finish() // Cleanup
	} else {
		ilog.Println("Failed to create a database client: ", err)
		return
	}

	printElems(&dbClient) // So apparently referencing a type that implements a supertype makes it compatible with that supertype
}

func printElems(client kismetClient.DataLineReader) {
	var (
		clientGenerator func() (kismetClient.DataElement, error)
	)

	if newGenerator, err := client.Elements() ; err == nil {
		clientGenerator = newGenerator
	} else {
		ilog.Println("Failed to read data from database: ", err)
		return
	}

	count := 0
	for elem, err := clientGenerator() ; err == nil && elem.HasData; elem, err = clientGenerator() {
		count++
		ilog.Printf("Got Elem %d ID: %v with coords: %v %v", count, elem.ID, elem.Lat, elem.Lon)
	}
}
