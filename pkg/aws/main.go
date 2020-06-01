package main

func main() {
	// TODO(efried): Process CLI like:
	// $0 {-f|--file} path/to/spec.yaml | {-D|--delete-all}

	// deleteEverything()
	desiredState := fileSystems{
		"fs1": fileSystem{
			accessPoints: accessPoints{
				"apX": "",
			},
		},
		"fs2": fileSystem{
			accessPoints: accessPoints{
				"apY": "",
				"apZ": "",
			},
		},
	}
	ensureFileSystemState(desiredState)
}