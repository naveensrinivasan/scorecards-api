package main

import (
	"bufio"
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
)

type Scorecard struct {
	Name string `json:"name"`
	json.RawMessage
}

type GitHubRepo struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

func main() {
	folder := os.Args[1]
	bucketName := "ossfscorecard.dev"
	ctx := context.Background()
	//
	// get all files from a directory
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal(err)
	}

	i := 0
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		file := f.Name()
		fmt.Println("Processing", file)
		openFile, err := os.OpenFile(path.Join(folder, file), os.O_RDONLY, os.ModePerm)
		if err != nil {
			log.Fatalf("open file error: %v", err)
			return
		}
		defer openFile.Close()
		client, err := storage.NewClient(ctx)
		if err != nil {
			log.Panicf("Failed to create client: %v", err)
		}
		// Sets the name for the new bucket.
		// Creates a Bucket instance.
		bucket := client.Bucket(bucketName)
		rd := bufio.NewReader(openFile)
		for {
			line, err := rd.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				fmt.Printf("read file line error: %v \n", err)
				continue
			}
			m := make(map[string]json.RawMessage)
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				panic(err)
			}
			// deserialize json string to struct
			var scorecard Scorecard
			if err := json.Unmarshal([]byte(line), &scorecard); err != nil {
				fmt.Printf("unmarshal json error: %v \n", err)
				continue
			}
			get(&scorecard.Name, m, "name")
			repo := parseGitHubPath(scorecard.Name)
			scorecard.RawMessage, err = json.Marshal(m)
			if err != nil {
				fmt.Printf("marshal error: %v\n", err)
				continue
			}
			obj := bucket.Object(fmt.Sprintf("v2/%s/%s/%s/result.json", repo.Host, repo.Owner, repo.Name))
			writer := obj.NewWriter(ctx)

			_, err = writer.Write(scorecard.RawMessage)
			if err != nil {
				fmt.Printf("write file error: %v\n", err)
				continue
			}
			if err := writer.Close(); err != nil {
				fmt.Printf("close file error: %v \n", err)
				continue
			}
			i++
			if i%100 == 0 {
				fmt.Printf("%v, %d\n", file, i)
			}
		}
	}
}
func get(to interface{}, m map[string]json.RawMessage, s string) {
	if err := json.Unmarshal(m[s], &to); err != nil {
		panic(err)
	}
	delete(m, s)
}

func parseGitHubPath(path string) GitHubRepo {
	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		return GitHubRepo{}
	}
	return GitHubRepo{Host: parts[0], Owner: parts[1], Name: parts[2]}
}
