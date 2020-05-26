package main

func main() {
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