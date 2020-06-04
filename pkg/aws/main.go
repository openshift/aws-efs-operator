package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"sigs.k8s.io/yaml"
)

type spec map[string][]string

func usage() {
	fmt.Fprintf(
		flag.CommandLine.Output(),
		"Usage: %s {--spec PATH | --discover | --delete-all}\n\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage

	var specfile = flag.String(
		"spec",
		"",
		`Path to a YAML spec file describing the desired file system and access point state.
The file represents a map, keyed by file system "token", of lists of access point "tokens".
(These tokens are arbitrary unique strings used to ensure idempotency.) For example:

    fs1:
        - apX
    fs2:
        - apY
        - apZ
    fs3: []

This will create three file systems. The first will have one access point; the second will
have two access points; the third will have none.`)

	var deleteAll = flag.Bool(
		"delete-all",
		false,
		"Delete all mount targets, file systems, and access points.")

	var discover = flag.Bool(
		"discover",
		false,
		`Discover and print file system and access point pairs, one per line, e.g.
    fs-a99c122a:fsap-099537fb4bb7d50ea
    fs-b89c123b:fsap-04e855ae78fe51eed
    fs-b89c123b:fsap-0b02dc545c4f9b076`)

	flag.Parse()

	numopts := 0
	if *specfile != "" {
		numopts++
	}
	if *deleteAll {
		numopts++
	}
	if *discover {
		numopts++
	}
	if numopts != 1 {
		fmt.Fprintf(
			flag.CommandLine.Output(),
			"Must specify exactly one of --spec, --delete-all, and --discover.\nUse -h for help.\n")
		os.Exit(2)
	}

	if *deleteAll {
		deleteEverything()
		os.Exit(0)
	}

	if *discover {
		discoverPrint()
		os.Exit(0)
	}

	y, err := ioutil.ReadFile(*specfile)
	if err != nil {
		panic(err)
	}
	specmap := make(spec)
	err = yaml.Unmarshal(y, &specmap)
	if err != nil {
		panic(err)
	}

	desiredState := make(fileSystems)
	for fskey, aplist := range specmap {
		desiredState[fskey] = fileSystem{
			accessPoints: make(accessPoints),
		}
		for _, apkey := range aplist {
			desiredState[fskey].accessPoints[apkey] = ""
		}
	}
	ensureFileSystemState(desiredState)
}
