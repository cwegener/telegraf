// +build windows
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/winsvc"

	"github.com/influxdata/telegraf/agent"
	"github.com/influxdata/telegraf/internal/config"
	_ "github.com/influxdata/telegraf/plugins/inputs/all"
	_ "github.com/influxdata/telegraf/plugins/outputs/all"
)

var fDebug = flag.Bool("debug", false,
	"show metrics as they're generated to stdout")
var fQuiet = flag.Bool("quiet", false,
	"run in quiet mode")
var fTest = flag.Bool("test", false, "gather metrics, print them out, and exit")
var fConfig = flag.String("config", "", "configuration file to load")
var fConfigDirectory = flag.String("config-directory", "",
	"directory containing additional *.conf files")
var fVersion = flag.Bool("version", false, "display the version")
var fSampleConfig = flag.Bool("sample-config", false,
	"print out full sample configuration")
var fPidfile = flag.String("pidfile", "", "file to write our pid to")
var fInputFilters = flag.String("input-filter", "",
	"filter the inputs to enable, separator is :")
var fOutputFilters = flag.String("output-filter", "",
	"filter the outputs to enable, separator is :")
var fUsage = flag.String("usage", "",
	"print usage for a plugin, ie, 'telegraf -usage mysql'")
var fServiceName = flag.String("service-name", "telegraf-winsvc", "Set service name")
var fServiceDesc = flag.String("service-desc", "Telegraf windows service", "Set service description")

var fServiceInstall = flag.Bool("service-install", false, "Install service")
var fServiceUninstall = flag.Bool("service-remove", false, "Remove service")
var fServiceStart = flag.Bool("service-start", false, "Start service")
var fServiceStop = flag.Bool("service-stop", false, "Stop service")

var fInputFiltersLegacy = flag.String("filter", "",
	"filter the inputs to enable, separator is :")
var fOutputFiltersLegacy = flag.String("outputfilter", "",
	"filter the outputs to enable, separator is :")
var fConfigDirectoryLegacy = flag.String("configdirectory", "",
	"directory containing additional *.conf files")

// Version ...
var Version string

var appPath string

const usage = `Telegraf, The plugin-driven server agent for collecting and reporting metrics.

Usage:

  telegraf <flags>

The flags are:

  -config <file>     configuration file to load
  -test              gather metrics once, print them to stdout, and exit
  -sample-config     print out full sample configuration to stdout
  -config-directory  directory containing additional *.conf files
  -input-filter      filter the input plugins to enable, separator is :
  -output-filter     filter the output plugins to enable, separator is :
  -usage             print usage for a plugin, ie, 'telegraf -usage mysql'
  -debug             print metrics as they're generated to stdout
  -quiet             run in quiet mode
  -version           print the version to stdout

Examples:

  # generate a telegraf config file:
  telegraf -sample-config > telegraf.conf

  # generate config with only cpu input & influxdb output plugins defined
  telegraf -sample-config -input-filter cpu -output-filter influxdb

  # run a single telegraf collection, outputing metrics to stdout
  telegraf -config telegraf.conf -test

  # run telegraf with all plugins defined in config file
  telegraf -config telegraf.conf

  # run telegraf, enabling the cpu & memory input, and influxdb output plugins
  telegraf -config telegraf.conf -input-filter cpu:mem -output-filter influxdb
`

func init() {
	// change to current dir
	var err error
	if appPath, err = winsvc.GetAppPath(); err != nil {
		log.Fatal(err)
	}
	if err := os.Chdir(filepath.Dir(appPath)); err != nil {
		log.Fatal(err)
	}
}

func main() {
	flag.Usage = usageExit
	flag.Parse()

	if flag.NFlag() == 0 {
		usageExit()
	}

	// install service
	if *fServiceInstall {
		if err := winsvc.InstallService(appPath, *fServiceName, *fServiceDesc); err != nil {
			log.Fatalf("installService(%s, %s): %v\n", *fServiceName, *fServiceDesc, err)
		}
		fmt.Printf("Done\n")
		return
	}

	// remove service
	if *fServiceUninstall {
		if err := winsvc.RemoveService(*fServiceName); err != nil {
			log.Fatalln("removeService:", err)
		}
		fmt.Printf("Done\n")
		return
	}

	// start service
	if *fServiceStart {
		if err := winsvc.StartService(*fServiceName); err != nil {
			log.Fatalln("startService:", err)
		}
		fmt.Printf("Done\n")
		return
	}

	// stop service
	if *fServiceStop {
		if err := winsvc.StopService(*fServiceName); err != nil {
			log.Fatalln("stopService:", err)
		}
		fmt.Printf("Done\n")
		return
	}

	// run as service
	if !winsvc.InServiceMode() {
		log.Println("main:", "runService")
		if err := winsvc.RunAsService(*fServiceName, start, stop, false); err != nil {
			log.Fatalf("svc.Run: %v\n", err)
		}
		return
	}

	start()
}

func start() {
	reload := make(chan bool, 1)
	reload <- true
	for <-reload {
		reload <- false
		flag.Usage = usageExit
		flag.Parse()

		if flag.NFlag() == 0 {
			usageExit()
		}

		var inputFilters []string
		if *fInputFiltersLegacy != "" {
			inputFilter := strings.TrimSpace(*fInputFiltersLegacy)
			inputFilters = strings.Split(":"+inputFilter+":", ":")
		}
		if *fInputFilters != "" {
			inputFilter := strings.TrimSpace(*fInputFilters)
			inputFilters = strings.Split(":"+inputFilter+":", ":")
		}

		var outputFilters []string
		if *fOutputFiltersLegacy != "" {
			outputFilter := strings.TrimSpace(*fOutputFiltersLegacy)
			outputFilters = strings.Split(":"+outputFilter+":", ":")
		}
		if *fOutputFilters != "" {
			outputFilter := strings.TrimSpace(*fOutputFilters)
			outputFilters = strings.Split(":"+outputFilter+":", ":")
		}

		if *fVersion {
			v := fmt.Sprintf("Telegraf - Version %s", Version)
			fmt.Println(v)
			return
		}

		if *fSampleConfig {
			config.PrintSampleConfig(inputFilters, outputFilters)
			return
		}

		if *fUsage != "" {
			if err := config.PrintInputConfig(*fUsage); err != nil {
				if err2 := config.PrintOutputConfig(*fUsage); err2 != nil {
					log.Fatalf("%s and %s", err, err2)
				}
			}
			return
		}

		var (
			c   *config.Config
			err error
		)

		if *fConfig != "" {
			c = config.NewConfig()
			c.OutputFilters = outputFilters
			c.InputFilters = inputFilters
			err = c.LoadConfig(*fConfig)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			fmt.Println("Usage: Telegraf")
			flag.PrintDefaults()
			return
		}

		if *fConfigDirectoryLegacy != "" {
			err = c.LoadDirectory(*fConfigDirectoryLegacy)
			if err != nil {
				log.Fatal(err)
			}
		}

		if *fConfigDirectory != "" {
			err = c.LoadDirectory(*fConfigDirectory)
			if err != nil {
				log.Fatal(err)
			}
		}
		if len(c.Outputs) == 0 {
			log.Fatalf("Error: no outputs found, did you provide a valid config file?")
		}
		if len(c.Inputs) == 0 {
			log.Fatalf("Error: no inputs found, did you provide a valid config file?")
		}

		ag, err := agent.NewAgent(c)
		if err != nil {
			log.Fatal(err)
		}

		if *fDebug {
			ag.Config.Agent.Debug = true
		}

		if *fQuiet {
			ag.Config.Agent.Quiet = true
		}

		if *fTest {
			err = ag.Test()
			if err != nil {
				log.Fatal(err)
			}
			return
		}

		err = ag.Connect()
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Starting Telegraf (version %s)\n", Version)
		log.Printf("Loaded outputs: %s", strings.Join(c.OutputNames(), " "))
		log.Printf("Loaded inputs: %s", strings.Join(c.InputNames(), " "))
		log.Printf("Tags enabled: %s", c.ListTags())

		if *fPidfile != "" {
			f, err := os.Create(*fPidfile)
			if err != nil {
				log.Fatalf("Unable to create pidfile: %s", err)
			}

			fmt.Fprintf(f, "%d\n", os.Getpid())

			f.Close()
		}

	}
}

func stop() {

	log.Printf("Hard Shutdown")
}

func usageExit() {
	fmt.Println(usage)
	os.Exit(0)
}
