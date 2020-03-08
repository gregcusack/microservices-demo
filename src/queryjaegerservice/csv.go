package main

import (
	"encoding/csv"
	"io"
	"os"
)

func readCSV(filename string) ([]map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)

	return CSVToMap(r)
}

// CSVToMap takes a reader and returns an array of dictionaries, using the header row as the keys
func CSVToMap(r *csv.Reader) ([]map[string]string, error) {
	var rows []map[string]string
	var header []string

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header == nil {
			header = record
		} else {
			dict := make(map[string]string, len(header))
			for i := range header {
				dict[header[i]] = record[i]
			}
			rows = append(rows, dict)
		}
	}

	return rows, nil
}
