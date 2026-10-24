// Command comicinfo parses a ComicInfo.xml file and prints its metadata as JSON.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/comicinfo"
)

func main() {
	path := "-"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	var data []byte
	var err error
	if path == "-" {
		data, err = readStdin()
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "comicinfo:", err)
		os.Exit(1)
	}

	ci, err := comicinfo.ParseBytes(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "comicinfo:", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ci); err != nil {
		fmt.Fprintln(os.Stderr, "comicinfo:", err)
		os.Exit(1)
	}
}

func readStdin() ([]byte, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, fmt.Errorf("no input file given")
	}
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return buf, err
		}
	}
	return buf, nil
}
