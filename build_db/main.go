// Command build_db creates a tweet-author database file.
//
// Inputs to build_db should be CSV files, like the ones
// generated by https://github.com/unixpickle/tweetdump.
package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"sort"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/tweetures"
)

func main() {
	var inputFile, outputFile string
	flag.StringVar(&inputFile, "in", "", "input CSV file")
	flag.StringVar(&outputFile, "out", "", "output DB file")
	flag.Parse()

	if inputFile == "" || outputFile == "" {
		essentials.Die("Required flags: -in and -out. See -help.")
	}

	log.Println("Opening input...")
	f, err := os.Open(inputFile)
	if err != nil {
		essentials.Die(err)
	}
	defer f.Close()

	log.Println("Couting usernames...")
	counts, err := usernameCounts(f)
	if err != nil {
		essentials.Die(err)
	}

	log.Println("Grouping tweets by user...")
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		essentials.Die(err)
	}
	mapping, err := tweetsPerUser(f, counts)
	if err != nil {
		essentials.Die(err)
	}

	log.Println("Sorting usernames...")
	usernames := make([]string, 0, len(mapping))
	for user := range mapping {
		usernames = append(usernames, user)
	}
	sort.Strings(usernames)

	log.Println("Creating output...")
	dbFile, err := os.Create(outputFile)
	if err != nil {
		essentials.Die(err)
	}
	defer dbFile.Close()

	log.Println("Writing output...")
	records := make(chan tweetures.Record, 1)
	go func() {
		defer close(records)
		for _, user := range usernames {
			userBytes := []byte(user)
			for _, tweet := range mapping[user] {
				records <- tweetures.Record{
					User: userBytes,
					Body: tweet,
				}
			}
		}
	}()
	if err := tweetures.WriteDB(dbFile, records); err != nil {
		essentials.Die(err)
	}
}

func usernameCounts(r io.Reader) (map[string]int, error) {
	reader := csv.NewReader(r)
	counts := map[string]int{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if len(row) < 3 {
			return nil, errors.New("expected at least 3 columns")
		}
		counts[row[1]]++
	}
	return counts, nil
}

func tweetsPerUser(r io.Reader, counts map[string]int) (map[string][][]byte, error) {
	reader := csv.NewReader(r)

	// Read every tweet body into a single buffer, then slice
	// it up into individual tweets.
	// This avoids some memory fragmentation, although not a
	// lot as far as I can tell.

	indices := map[string][]int{}
	buffer := bytes.Buffer{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else if counts[row[1]] < 2 {
			continue
		}
		user := row[1]
		msg := []byte(row[len(row)-1])
		indices[user] = append(indices[user], buffer.Len(), buffer.Len()+len(msg))
		buffer.Write(msg)
	}

	fullBytes := buffer.Bytes()
	res := map[string][][]byte{}
	for user, is := range indices {
		var strs [][]byte
		for i := 0; i < len(is); i += 2 {
			strs = append(strs, fullBytes[is[i]:is[i+1]])
		}
		res[user] = strs
	}
	return res, nil
}
